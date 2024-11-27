package internal

import (
	"context"
	"sync"

	"github.com/escalopa/raft-kv/internal/core"
	"github.com/pkg/errors"
)

var (
	loadOnce sync.Once
)

type (
	StateStore interface {
		GetTerm(ctx context.Context) (term uint64, err error)
		SetTerm(ctx context.Context, term uint64) (err error)

		GetVoted(ctx context.Context) (votedFor uint64, err error)
		SetVoted(ctx context.Context, votedFor uint64) (err error)

		GetCommit(ctx context.Context) (commitIndex uint64, err error)
		SetCommit(ctx context.Context, commitIndex uint64) (err error)

		GetLastApplied(ctx context.Context) (lastApplied uint64, err error)
		SetLastApplied(ctx context.Context, lastApplied uint64) (err error)
	}
)

type uint64Obj struct {
	value uint64
	sync.RWMutex
}

type StateFacade struct {
	ctx context.Context

	term        uint64Obj
	commitIndex uint64Obj
	lastApplied uint64Obj

	votedFor uint64Obj
	leaderID uint64Obj

	state core.State

	stateState StateStore

	stateUpdateChan <-chan core.StateUpdate
}

func NewStateFacade(
	ctx context.Context,
	stateState StateStore,
	stateUpdateChan <-chan core.StateUpdate,
) *StateFacade {
	sf := &StateFacade{
		ctx:             ctx,
		state:           core.Follower,
		stateState:      stateState,
		stateUpdateChan: stateUpdateChan,
	}
	return sf
}

func (sf *StateFacade) GetTerm() uint64 {
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
	sf.votedFor.RLock()
	defer sf.votedFor.RUnlock()
	return sf.votedFor.value
}

func (sf *StateFacade) GetLeaderID() uint64 {
	sf.leaderID.RLock()
	defer sf.leaderID.RUnlock()
	return sf.leaderID.value
}

func (sf *StateFacade) IsLeader() bool {
	return sf.state == core.Leader
}

func (sf *StateFacade) LoadState(ctx context.Context) error {
	var err error
	loadOnce.Do(func() {
		err = sf.loadState(ctx)
	})
	return err
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
			return
		}
	}
}

func (sf *StateFacade) processStateUpdate(update core.StateUpdate) {
	// TODO: implement
}
