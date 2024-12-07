package service

import (
	"context"
	"sync"
	"time"

	"github.com/catalystgo/logger/logger"
	"github.com/escalopa/raft-kv/internal/core"
	"github.com/pkg/errors"
)

type nodeState string

func (s nodeState) String() string {
	return string(s)
}

const (
	FollowerState  nodeState = "follower"
	CandidateState nodeState = "candidate"
	LeaderState    nodeState = "leader"
)

type uint64Obj struct {
	value uint64
	sync.RWMutex
}

type raftState struct {
	ctx context.Context

	raftID uint64

	// persistent state on all servers (attributes mirror the persistent state in StateStore
	// but allow for faster access on reads since they are stored in memory)
	term        uint64Obj
	commitIndex uint64Obj
	lastApplied uint64Obj
	votedFor    uint64Obj

	lastLogIndex uint64
	lastLogTerm  uint64
	lastLogLock  sync.RWMutex // lock for lastLogIndex and lastLogTerm

	// volatile state on all servers
	leaderID uint64Obj

	state     nodeState
	stateLock sync.RWMutex

	lastContact     time.Time // last time we got a heartbeat from the leader or voted for a leader
	lastContactLock sync.RWMutex

	stateState StateStore
	entryStore EntryStore
}

func newRaftState(ctx context.Context, stateState StateStore, entryStore EntryStore) (*raftState, error) {
	sf := &raftState{
		ctx:        ctx,
		state:      FollowerState,
		stateState: stateState,
		entryStore: entryStore,
	}

	err := sf.loadState(ctx)
	if err != nil {
		return nil, err
	}

	return sf, nil
}

func (sf *raftState) GetTerm() uint64 {
	sf.term.RLock()
	defer sf.term.RUnlock()
	return sf.term.value
}

func (sf *raftState) IncrementTerm() uint64 {
	sf.term.Lock()
	defer sf.term.Unlock()

	sf.term.value++
	err := sf.stateState.SetTerm(sf.ctx, sf.term.value)
	if err != nil {
		logger.ErrorKV(sf.ctx, "increment term", "error", err)
		return 0
	}

	return sf.term.value
}

func (sf *raftState) SetTerm(term uint64) {
	sf.term.Lock()
	defer sf.term.Unlock()

	if term < sf.term.value {
		logger.WarnKV(sf.ctx, "set term is less than current term")
		return
	}

	err := sf.stateState.SetTerm(sf.ctx, term)
	if err != nil {
		logger.ErrorKV(sf.ctx, "set term", "error", err)
		return // do not update cache
	}

	sf.term.value = term
}

func (sf *raftState) GetCommitIndex() uint64 {
	sf.commitIndex.RLock()
	defer sf.commitIndex.RUnlock()
	return sf.commitIndex.value
}

func (sf *raftState) SetCommitIndex(index uint64) {
	sf.commitIndex.Lock()
	defer sf.commitIndex.Unlock()

	err := sf.stateState.SetCommit(sf.ctx, index)
	if err != nil {
		logger.ErrorKV(sf.ctx, "set commit index", "error", err)
		return // do not update cache
	}

	sf.commitIndex.value = index
}

func (sf *raftState) GetLastApplied() uint64 {
	sf.lastApplied.RLock()
	defer sf.lastApplied.RUnlock()
	return sf.lastApplied.value
}

func (sf *raftState) SetLastApplied(index uint64) {
	sf.lastApplied.Lock()
	defer sf.lastApplied.Unlock()

	err := sf.stateState.SetLastApplied(sf.ctx, index)
	if err != nil {
		logger.ErrorKV(sf.ctx, "set last applied", "error", err)
		return // do not update cache
	}

	sf.lastApplied.value = index
}

func (sf *raftState) GetVotedFor() uint64 {
	sf.votedFor.RLock()
	defer sf.votedFor.RUnlock()
	return sf.votedFor.value
}

func (sf *raftState) SetVotedFor(votedFor uint64) {
	sf.votedFor.Lock()
	defer sf.votedFor.Unlock()

	err := sf.stateState.SetVotedFor(sf.ctx, votedFor)
	if err != nil {
		logger.ErrorKV(sf.ctx, "set voted for", "error", err)
		return // do not update cache
	}

	sf.votedFor.value = votedFor
}

func (sf *raftState) GetLeaderID() uint64 {
	sf.leaderID.RLock()
	defer sf.leaderID.RUnlock()
	return sf.leaderID.value
}

func (sf *raftState) SetLeaderID(leaderID uint64) {
	sf.leaderID.RLock()
	defer sf.leaderID.RUnlock()
	sf.leaderID.value = leaderID
}

func (sf *raftState) GetState() nodeState {
	sf.stateLock.RLock()
	defer sf.stateLock.RUnlock()
	return sf.state
}

func (sf *raftState) SetState(state nodeState) {
	sf.stateLock.Lock()
	defer sf.stateLock.Unlock()
	sf.state = state
}

func (sf *raftState) GetLastContact() time.Time {
	sf.lastContactLock.RLock()
	defer sf.lastContactLock.RUnlock()
	return sf.lastContact
}

func (sf *raftState) UpdateLastContact() {
	sf.lastContactLock.RLock()
	defer sf.lastContactLock.RUnlock()
	sf.lastContact = time.Now()
}

func (sf *raftState) GetLastLog() (index, term uint64) {
	sf.lastLogLock.RLock()
	defer sf.lastLogLock.RUnlock()
	return sf.lastLogIndex, sf.lastLogTerm
}

func (sf *raftState) SetLastLog(index, term uint64) {
	sf.lastLogLock.Lock()
	defer sf.lastLogLock.Unlock()
	sf.lastLogIndex = index
	sf.lastLogTerm = term
}

func (sf *raftState) IsLeader() bool {
	return sf.state == LeaderState
}

func (sf *raftState) loadState(ctx context.Context) error {
	term, err := sf.stateState.GetTerm(ctx)
	if err != nil {
		if !errors.Is(err, core.ErrNotFound) {
			return err
		}
	}

	votedFor, err := sf.stateState.GetVotedFor(ctx)
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

	lastEntry, err := sf.entryStore.Last(ctx)
	if err != nil {
		if !errors.Is(err, core.ErrNotFound) {
			return err
		}
		lastEntry = &core.Entry{} // no entries
	}

	sf.term.value = term
	sf.votedFor.value = votedFor
	sf.commitIndex.value = commitIndex
	sf.lastApplied.value = lastApplied

	sf.lastLogIndex = lastEntry.Index
	sf.lastLogTerm = lastEntry.Term

	return nil
}
