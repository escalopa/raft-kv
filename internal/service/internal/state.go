package internal

import (
	"context"
	"sync"

	"github.com/catalystgo/logger/logger"
	"github.com/escalopa/raft-kv/internal/core"
	"github.com/pkg/errors"
)

var (
	setLeaderOnce sync.Once
)

type (
	StateStore interface {
		GetTerm(ctx context.Context) (term uint64, err error)
		SetTerm(ctx context.Context, term uint64) (err error)

		GetCommit(ctx context.Context) (commitIndex uint64, err error)
		SetCommit(ctx context.Context, commitIndex uint64) (err error)

		GetLastApplied(ctx context.Context) (lastApplied uint64, err error)
		SetLastApplied(ctx context.Context, lastApplied uint64) (err error)
	}

	Leader interface {
		Start(ctx context.Context) error
		Stop(ctx context.Context)
	}
)

type uint64Obj struct {
	value uint64
	sync.RWMutex
}

type StateFacade struct {
	ctx context.Context

	raftID uint64

	// persistent state on all servers (attributes mirror the persistent state in StateStore
	// but allow for faster access on reads since they are stored in memory)
	term        uint64Obj
	commitIndex uint64Obj
	lastApplied uint64Obj

	// volatile state on all servers
	votedFor uint64Obj
	leaderID uint64Obj

	state     core.State
	stateLock sync.RWMutex

	leader     Leader
	stateState StateStore

	stateUpdateChan <-chan *core.StateUpdate
}

func NewStateFacade(
	ctx context.Context,
	stateState StateStore,
	stateUpdateChan <-chan *core.StateUpdate,
) (*StateFacade, error) {
	sf := &StateFacade{
		ctx:             ctx,
		state:           core.Follower,
		stateState:      stateState,
		stateUpdateChan: stateUpdateChan,
	}

	err := sf.loadState(ctx)
	if err != nil {
		return nil, err
	}

	go sf.processStateUpdates()

	return sf, nil
}

func (sf *StateFacade) SetLeader(leader Leader) {
	setLeaderOnce.Do(func() {
		sf.leader = leader
	})
}

func (sf *StateFacade) GetTerm() uint64 {
	sf.stateLock.RLock()
	defer sf.stateLock.RUnlock()

	sf.term.RLock()
	defer sf.term.RUnlock()
	return sf.term.value
}

func (sf *StateFacade) GetCommitIndex() uint64 {
	sf.commitIndex.RLock()
	defer sf.commitIndex.RUnlock()
	return sf.commitIndex.value
}

func (sf *StateFacade) GetLastApplied() uint64 {
	sf.lastApplied.RLock()
	defer sf.lastApplied.RUnlock()
	return sf.lastApplied.value
}

func (sf *StateFacade) GetVotedFor() uint64 {
	sf.stateLock.RLock()
	defer sf.stateLock.RUnlock()

	sf.votedFor.RLock()
	defer sf.votedFor.RUnlock()
	return sf.votedFor.value
}

func (sf *StateFacade) GetLeaderID() uint64 {
	sf.stateLock.RLock()
	defer sf.stateLock.RUnlock()

	sf.leaderID.RLock()
	defer sf.leaderID.RUnlock()
	return sf.leaderID.value
}

func (sf *StateFacade) GetState() core.State {
	sf.stateLock.RLock()
	defer sf.stateLock.RUnlock()
	return sf.state
}

func (sf *StateFacade) IsLeader() bool {
	return sf.state == core.Leader
}

func (sf *StateFacade) loadState(ctx context.Context) error {
	term, err := sf.stateState.GetTerm(ctx)
	if err != nil {
		if !errors.Is(err, core.ErrNotFound) {
			return err
		}
	}

	commitIndex, err := sf.stateState.GetCommit(ctx)
	if err != nil {
		if !errors.Is(err, core.ErrNotFound) {
			return err
		}
	}

	lastApplied, err := sf.stateState.GetLastApplied(ctx)
	if err != nil {
		if !errors.Is(err, core.ErrNotFound) {
			return err
		}
	}

	sf.term.value = term
	sf.commitIndex.value = commitIndex
	sf.lastApplied.value = lastApplied

	return nil
}

func (sf *StateFacade) processStateUpdates() {
	for {
		select {
		case update := <-sf.stateUpdateChan:
			sf.processStateUpdate(update)
		case <-sf.ctx.Done():
			logger.WarnKV(sf.ctx, "state update processing stopped")
			sf.leader.Stop(sf.ctx) // On context canceled stop the leader too
			return
		}
	}
}

func (sf *StateFacade) processStateUpdate(update *core.StateUpdate) {
	var err error

	if update == nil {
		return
	}

	defer close(update.Done)

	logger.WarnKV(sf.ctx, "process state update",
		"type", update.Type,
		"term", update.Term,
		"state", update.State,
		"votedFor", update.VotedFor,
		"commitIndex", update.CommitIndex,
		"lastApplied", update.LastApplied,
		"leaderID", update.LeaderID,
	)

	switch update.Type {

	// persistent attributes

	// Notice: for some attributes we need to prevent going backwards, this can happen due to multiple reads and data
	// changes in the same time i.e. term can be changed in appendEntries, requestVote and state update at the same time

	case core.StateUpdateTypeTerm:
		sf.term.Lock()
		sf.term.value = max(sf.term.value, update.Term) // prevent going backwards
		err = sf.stateState.SetTerm(sf.ctx, update.Term)
		sf.term.Unlock()
	case core.StateUpdateTypeCommitIndex:
		sf.commitIndex.Lock()
		sf.commitIndex.value = max(sf.commitIndex.value, update.CommitIndex) // prevent going backwards
		err = sf.stateState.SetCommit(sf.ctx, update.CommitIndex)
		sf.commitIndex.Unlock()
	case core.StateUpdateTypeLastApplied:
		sf.lastApplied.Lock()
		sf.lastApplied.value = update.LastApplied
		err = sf.stateState.SetLastApplied(sf.ctx, update.LastApplied)
		sf.lastApplied.Unlock()
	case core.StateUpdateTypeState:
		sf.stateLock.Lock()
		defer sf.stateLock.Unlock()

		oldState := sf.state
		sf.state = update.State

		switch sf.state {
		case core.Leader:
			err = sf.leader.Start(sf.ctx)
		case core.Candidate:
			sf.term.value++
			sf.votedFor.value = sf.raftID
			sf.leaderID.value = sf.raftID
			err = sf.stateState.SetTerm(sf.ctx, sf.term.value)
		case core.Follower:
			sf.votedFor.value = update.VotedFor
			sf.leaderID.value = update.LeaderID
			sf.leader.Stop(sf.ctx)
			if oldState == core.Leader {
				logger.WarnKV(sf.ctx, "leader stopped")
			}
		}

		logger.WarnKV(sf.ctx, "node state update", "old_state", oldState, "new_state", sf.state)

	// volatile attributes

	case core.StateUpdateTypeVotedFor:
		sf.votedFor.Lock()
		sf.votedFor.value = update.VotedFor
		sf.votedFor.Unlock()
	case core.StateUpdateTypeLeaderID:
		sf.leaderID.Lock()
		sf.leaderID.value = update.LeaderID
		sf.leaderID.Unlock()
	}

	if err != nil {
		logger.ErrorKV(sf.ctx, "state update", "error", err, "type", update.Type)
	}
}
