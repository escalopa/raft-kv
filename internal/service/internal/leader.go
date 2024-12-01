package internal

import (
	"context"
	"sync"
	"time"

	"github.com/catalystgo/logger/logger"
	"github.com/escalopa/raft-kv/internal/core"
	desc "github.com/escalopa/raft-kv/pkg/raft"
	"github.com/pkg/errors"
	"github.com/puzpuzpuz/xsync/v3"
)

const (
	maxEntriesPerRequest = 1000
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

	Config interface {
		GetHeartbeatPeriod() time.Duration
	}
)

type LeaderFacade struct {
	config Config

	raftID core.ServerID

	servers map[core.ServerID]desc.RaftServiceClient

	// nextIndex index for each server of the next log entry to send to that server
	// initialized to leader last log index + 1
	nextIndex *xsync.MapOf[core.ServerID, uint64]

	// matchIndex index for each server of highest log entry known to be replicated on server
	// initialized to 0, increases monotonically
	matchIndex *xsync.MapOf[core.ServerID, uint64]

	// errorScheduler map of serverID and error message timestamp
	errorScheduler *xsync.MapOf[core.ServerID, map[string]time.Time]

	entryStore EntryStore
	stateStore SimpleStateStore

	stateUpdateChan chan<- *core.StateUpdate

	done chan struct{}
	wg   sync.WaitGroup
}

func NewLeaderFacade(
	config Config,
	raftID core.ServerID,
	servers map[core.ServerID]desc.RaftServiceClient,
	entryStore EntryStore,
	stateStore SimpleStateStore,
	stateUpdateChan chan<- *core.StateUpdate,
) *LeaderFacade {
	lf := &LeaderFacade{
		config:          config,
		raftID:          raftID,
		servers:         servers,
		nextIndex:       xsync.NewMapOf[core.ServerID, uint64](),
		matchIndex:      xsync.NewMapOf[core.ServerID, uint64](),
		errorScheduler:  xsync.NewMapOf[core.ServerID, map[string]time.Time](),
		entryStore:      entryStore,
		stateStore:      stateStore,
		stateUpdateChan: stateUpdateChan,
		done:            make(chan struct{}),
	}

	for internalRaftID := range servers {
		lf.errorScheduler.Store(internalRaftID, make(map[string]time.Time))
	}

	close(lf.done) // initially closed
	return lf
}

func (l *LeaderFacade) Start(ctx context.Context) error {
	l.Stop(ctx) // stop any previous running goroutines
	l.done = make(chan struct{})

	entryLast, err := l.entryStore.Last(ctx)
	if err != nil {
		if !errors.Is(err, core.ErrNotFound) {
			return err
		}
		entryLast = &core.Entry{}
	}

	for raftID := range l.servers {
		l.nextIndex.Store(raftID, entryLast.Index+1)
		l.matchIndex.Store(raftID, 0)
	}

	l.wg.Add(len(l.servers))
	for id, server := range l.servers {
		go l.heartbeatLoop(ctx, id, server)
	}

	logger.WarnKV(ctx, "leader started")

	return nil
}

func (l *LeaderFacade) heartbeatLoop(ctx context.Context, raftID core.ServerID, server desc.RaftServiceClient) {
	defer l.wg.Done()

	timer := time.NewTimer(0)
	defer timer.Stop()

	// capture the done channel (protects from running 2 sendHeartbeat
	// when the leader is stopped and started quickly)
	done := l.done

	logger.WarnKV(ctx, "start heartbeat loop", "raft_id", raftID)

	for {
		timer.Reset(l.config.GetHeartbeatPeriod())

		select {
		case <-timer.C:
			l.sendHeartbeat(ctx, raftID, server)
		case <-done:
			logger.WarnKV(ctx, "stop heartbeat loop", "raft_id", raftID)
			return
		}
	}
}

func (l *LeaderFacade) sendHeartbeat(ctx context.Context, raftID core.ServerID, server desc.RaftServiceClient) {
	term := l.stateStore.GetTerm()

	entryLast, err := l.entryStore.Last(ctx)
	if err != nil {
		if !errors.Is(err, core.ErrNotFound) {
			logger.ErrorKV(ctx, "heartbeat last entry", "error", err)
		}

		// no entries to send (first run) but
		// we still need to send the heartbeat
		entryLast = &core.Entry{}
	}

	var (
		lowerBound, _ = l.nextIndex.Load(raftID)
		upperBound    = min(entryLast.Index, lowerBound+maxEntriesPerRequest-1) // inclusive
	)

	entries, err := l.entryStore.Range(ctx, lowerBound, upperBound)
	if err != nil {
		logger.ErrorKV(ctx, "heartbeat range", "error", err, "raft_id", raftID)
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
	prevEntry, err := l.entryStore.At(ctx, lowerBound-1)
	if err != nil {
		if !errors.Is(err, core.ErrNotFound) {
			logger.ErrorKV(ctx, "heartbeat at", "error", err, "raft_id", raftID)
			return
		}

		// no previous entry (first entry)
		if lowerBound == 1 {
			prevEntry = &core.Entry{}
		}
	}

	if prevEntry == nil {
		logger.ErrorKV(ctx, "load prev log entry on heartbeat send",
			"raft_id", raftID,
			"lower_bound", lowerBound,
		)
		return
	}

	res, err := server.AppendEntries(ctx, &desc.AppendEntriesRequest{
		Term:         term,
		LeaderId:     uint64(l.raftID),
		PrevLogIndex: prevEntry.Index,
		PrevLogTerm:  prevEntry.Term,
		LeaderCommit: l.stateStore.GetCommitIndex(),
		Entries:      entriesDesc,
	})
	if err != nil {
		if l.logEnabled(raftID, err) {
			logger.ErrorKV(ctx, "heartbeat append entries", "error", err, "raft_id", raftID)
		}
		return
	}

	if term < res.Term {
		l.sendStateUpdate(core.StateUpdate{
			Type: core.StateUpdateTypeTerm,
			Term: res.Term,
		})
		return
	}

	// If the follower is up to date, then we can update the nextIndex and matchIndex
	if res.Success {
		l.nextIndex.Store(raftID, upperBound+1)
		l.matchIndex.Store(raftID, upperBound)
		return
	}

	// On the next heartbeat send from the latest log + 1
	nextIndex := res.LastLogIndex + 1

	// If prevLogIndex is less than the latest log on the follower, that means the follower has some
	// inconsistent data that need to be cleaned therefore we decrement the log by -1
	if res.LastLogIndex >= prevEntry.Index {
		nextIndex = prevEntry.Index - 1
	}

	l.nextIndex.Store(raftID, nextIndex)
}

func (l *LeaderFacade) sendStateUpdate(update core.StateUpdate) {
	update.Done = make(chan struct{})
	l.stateUpdateChan <- &update
	<-update.Done // wait for the update to be processed
}

// Stop stops the leader state machine and all its goroutines
// It is safe to call this method multiple times
func (l *LeaderFacade) Stop(ctx context.Context) {
	select {
	case <-l.done: // already closed
	default:
		close(l.done)
		logger.WarnKV(ctx, "leader stopped")
	}
	l.wg.Wait()
}

// logEnabled returns true if the server is allowed to log the error
// only one error type per second is allowed to be logged for each server
func (l *LeaderFacade) logEnabled(serverID core.ServerID, err error) bool {
	if err == nil {
		return false
	}

	if _, ok := l.servers[serverID]; !ok {
		return false
	}

	lastError, ok := l.errorScheduler.Load(serverID)
	if !ok {
		return true
	}

	if time.Since(lastError[err.Error()]) > time.Second {
		lastError[err.Error()] = time.Now()
		return true
	}

	return false
}
