package internal

import (
	"context"
	"time"

	"github.com/catalystgo/logger/logger"
	"github.com/escalopa/raft-kv/internal/core"
	desc "github.com/escalopa/raft-kv/pkg/raft"
	"github.com/pkg/errors"
)

const (
	maxEntriesPerRequest = 1000
	heartbeatFrequency   = 100 * time.Millisecond
)

type (
	EntryStore interface {
		AppendEntries(ctx context.Context, entries ...*core.Entry) (err error)
		Last(ctx context.Context) (entry *core.Entry, err error)
		At(ctx context.Context, index uint64) (entry *core.Entry, err error)
		Range(ctx context.Context, from uint64, to uint64) (entries []*core.Entry, err error)
	}

	SimpleStateStore interface {
		GetTerm() uint64
		GetCommitIndex() uint64
	}
)

type LeaderFacade struct {
	raftID core.ServerID

	servers map[core.ServerID]desc.RaftServiceClient

	// nextIndex index for each server of the next log entry to send to that server
	// initialized to leader last log index + 1
	nextIndex map[core.ServerID]uint64

	// matchIndex index for each server of highest log entry known to be replicated on server
	// initialized to 0, increases monotonically
	matchIndex map[core.ServerID]uint64

	entryStore EntryStore
	stateStore SimpleStateStore

	stateUpdateChan chan<- core.StateUpdate

	done chan struct{}
}

func NewLeaderFacade(
	raftID core.ServerID,
	servers map[core.ServerID]desc.RaftServiceClient,
	entryStore EntryStore,
	stateStore SimpleStateStore,
	stateUpdateChan chan<- core.StateUpdate,
) *LeaderFacade {
	return &LeaderFacade{
		raftID:          raftID,
		servers:         servers,
		nextIndex:       make(map[core.ServerID]uint64),
		matchIndex:      make(map[core.ServerID]uint64),
		entryStore:      entryStore,
		stateStore:      stateStore,
		stateUpdateChan: stateUpdateChan,
	}
}

func (l *LeaderFacade) Start(ctx context.Context) error {
	l.Stop() // stop any previous running goroutines
	l.done = make(chan struct{})

	entryLast, err := l.entryStore.Last(ctx)
	if err != nil {
		if !errors.Is(err, core.ErrNotFound) {
			return err
		}
		entryLast = &core.Entry{}
	}

	for id := range l.servers {
		l.nextIndex[id] = entryLast.Index + 1
		l.matchIndex[id] = 0
	}

	for id, server := range l.servers {
		go l.heartbeatLoop(ctx, id, server)
	}

	return nil
}

func (l *LeaderFacade) heartbeatLoop(ctx context.Context, serverID core.ServerID, server desc.RaftServiceClient) {
	ticker := time.NewTicker(heartbeatFrequency)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			l.sendHeartbeat(ctx, serverID, server)
		case <-l.done:
			return
		}
	}
}

func (l *LeaderFacade) sendHeartbeat(ctx context.Context, serverID core.ServerID, server desc.RaftServiceClient) {
	entryLast, err := l.entryStore.Last(ctx)
	if err != nil {
		if !errors.Is(err, core.ErrNotFound) {
			logger.ErrorKV(ctx, "heartbeat last entry", "error", err)
		}
		return // no entries to send
	}

	var (
		lowerBound = l.nextIndex[serverID]
		upperBound = min(entryLast.Index, lowerBound+maxEntriesPerRequest-1) // inclusive
	)

	entries, err := l.entryStore.Range(ctx, lowerBound, upperBound)
	if err != nil {
		logger.ErrorKV(ctx, "heartbeat range", "error", err, "server_id", serverID)
		return
	}

	entriesDesc := make([]*desc.Entry, len(entries))
	for i, e := range entries {
		entriesDesc[i] = &desc.Entry{
			Term:  e.Term,
			Index: e.Index,
			Data:  e.Data,
		}
	}

	// Get the previous entry to send the correct prevLogIndex and prevLogTerm
	entry, err := l.entryStore.At(ctx, lowerBound-1)
	if err != nil {
		if !errors.Is(err, core.ErrNotFound) {
			logger.ErrorKV(ctx, "heartbeat at", "error", err, "server_id", serverID)
			return
		}
		// no previous entry (first entry)
	}

	var (
		prevLogIndex = entryLast.Index
		prevLogTerm  = entryLast.Term
	)

	if entry != nil {
		prevLogIndex = entry.Index
		prevLogTerm = entry.Term
	}

	res, err := server.AppendEntries(ctx, &desc.AppendEntriesRequest{
		Term:         l.stateStore.GetTerm(),
		LeaderId:     uint64(l.raftID),
		PrevLogIndex: prevLogIndex,
		PrevLogTerm:  prevLogTerm,
		LeaderCommit: l.stateStore.GetCommitIndex(),
		Entries:      entriesDesc,
	})
	if err != nil {
		logger.ErrorKV(ctx, "heartbeat append entries", "error", err, "server_id", serverID)
		return
	}

	if l.stateStore.GetTerm() < res.Term {
		l.updateState(core.StateUpdate{
			Type: core.StateUpdateTypeTerm,
			Term: res.Term,
		})
		return
	}

	// If the follower is up to date, then we can update the nextIndex and matchIndex
	if res.Success {
		l.nextIndex[serverID] = upperBound + 1
		l.matchIndex[serverID] = upperBound
		return
	}

	// If the follower is behind, then we need to send the missing entries
	l.nextIndex[serverID] = res.LastLogIndex + 1
	l.matchIndex[serverID] = res.LastLogIndex
}

// Stop stops the leader state machine and all its goroutines
// It is safe to call this method multiple times
func (l *LeaderFacade) Stop() {
	select {
	case <-l.done: // already closed
	default:
		close(l.done)
	}
}

func (l *LeaderFacade) updateState(update core.StateUpdate) {
	update.Done = make(chan struct{})
	l.stateUpdateChan <- update
	<-update.Done // wait for the update to be processed
}