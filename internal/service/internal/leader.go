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
		GetLeaderStalePeriod() time.Duration
		GetLeaderCheckStepDownPeriod() time.Duration
	}
)

type LeaderFacade struct {
	cancel context.CancelFunc

	config Config

	raftID core.ServerID

	servers map[core.ServerID]desc.RaftServiceClient

	// nextIndex index for each server of the next log entry to send to that server
	// initialized to leader last log index + 1
	nextIndex *xsync.MapOf[core.ServerID, uint64]

	// matchIndex index for each server of highest log entry known to be replicated on server
	// initialized to 0, increases monotonically
	matchIndex *xsync.MapOf[core.ServerID, uint64]

	// lastHeartbeat map of serverID and last heartbeat timestamp sent
	// used to detect if we are able to contact the majority of the servers in the cluster
	// if we can't contact the majority of the servers, we should step down
	lastHeartbeat *xsync.MapOf[core.ServerID, time.Time]

	entryStore EntryStore
	stateStore SimpleStateStore

	stateUpdateChan chan<- *core.StateUpdate

	wg sync.WaitGroup
}

func NewLeaderFacade(
	config Config,
	raftID core.ServerID,
	servers map[core.ServerID]desc.RaftServiceClient,
	entryStore EntryStore,
	stateStore SimpleStateStore,
	stateUpdateChan chan<- *core.StateUpdate,
) *LeaderFacade {
	return &LeaderFacade{
		cancel:          func() { /* noop */ }, // avoid nil panic
		config:          config,
		raftID:          raftID,
		servers:         servers,
		nextIndex:       xsync.NewMapOf[core.ServerID, uint64](),
		matchIndex:      xsync.NewMapOf[core.ServerID, uint64](),
		lastHeartbeat:   xsync.NewMapOf[core.ServerID, time.Time](),
		entryStore:      entryStore,
		stateStore:      stateStore,
		stateUpdateChan: stateUpdateChan,
	}
}

func (l *LeaderFacade) Start(ctx context.Context) error {
	ctx, l.cancel = context.WithCancel(ctx)

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
		l.lastHeartbeat.Store(raftID, time.Now())
	}

	l.run(ctx)

	logger.WarnKV(ctx, "leader started")

	return nil
}

func (l *LeaderFacade) run(ctx context.Context) {
	for raftID := range l.servers {
		l.goRepeat(ctx, func() { l.sendHeartbeat(ctx, raftID) }, l.config.GetHeartbeatPeriod)
	}
	l.goRepeat(ctx, func() { l.checkStepDown(ctx) }, l.config.GetLeaderCheckStepDownPeriod)
}

func (l *LeaderFacade) sendHeartbeat(ctx context.Context, raftID core.ServerID) {
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

	server := l.servers[raftID]

	sendCtx, cancel := context.WithTimeout(ctx, 200*time.Second) // TODO: make this configurable
	defer cancel()

	res, err := server.AppendEntries(sendCtx, &desc.AppendEntriesRequest{
		Term:         term,
		LeaderId:     uint64(l.raftID),
		PrevLogIndex: prevEntry.Index,
		PrevLogTerm:  prevEntry.Term,
		LeaderCommit: l.stateStore.GetCommitIndex(),
		Entries:      entriesDesc,
	})
	if err != nil {
		// TODO: add backoff and retry
		logger.ErrorKV(ctx, "heartbeat append entries", "error", err, "raft_id", raftID)
		return
	}

	// If we got a response from the follower, we can update the last heartbeat
	// even if the request was not successful (i.e. the follower is not up to date)
	defer l.lastHeartbeat.Store(raftID, time.Now())

	if term < res.Term {
		logger.InfoKV(ctx, "leader update term", "old_term", term, "res_term", res.Term, "raft_id", raftID)
		l.sendStateUpdate(ctx, core.StateUpdate{
			Type: core.StateUpdateTypeTerm,
			Term: res.Term,
		})
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

// checkStepDown checks if the leader can contact the majority of the servers in the cluster
// if it can't contact the majority of the servers, it should step down
func (l *LeaderFacade) checkStepDown(ctx context.Context) {
	contacted := 1 // leader is counted as contacted
	for raftID := range l.servers {
		lastHeartbeat, ok := l.lastHeartbeat.Load(raftID)
		if !ok {
			continue
		}

		if time.Since(lastHeartbeat) < l.config.GetLeaderStalePeriod() {
			contacted++
		}
	}

	canContactMajority := contacted > (len(l.servers)+1)/2
	if !canContactMajority {
		logger.WarnKV(ctx, "leader stepping down because cannot contact majority of the nodes in cluster", "contacted", contacted)
		l.sendStateUpdate(ctx, core.StateUpdate{
			Type:  core.StateUpdateTypeState,
			State: core.Follower,
		})
		logger.WarnKV(ctx, "leader stepped down")
	}
}

func (l *LeaderFacade) sendStateUpdate(ctx context.Context, update core.StateUpdate) {
	update.Done = make(chan struct{})

	select {
	case l.stateUpdateChan <- &update:
	case <-ctx.Done(): // leader stopped
		return
	}

	select {
	case <-update.Done: // wait for the update to be processed
	case <-ctx.Done(): // leader stopped
		return
	}
}

// repeat runs the given function f every freq until done is closed
func (l *LeaderFacade) goRepeat(ctx context.Context, f func(), freq func() time.Duration) {
	l.wg.Add(1)
	go func() {
		defer l.wg.Done()

		timer := time.NewTimer(0)
		defer timer.Stop()

		for {
			timer.Reset(freq())

			select {
			case <-ctx.Done():
				return
			case <-timer.C:
				f()
			}
		}
	}()
}

// Stop stops the leader state machine and all its goroutines
// It is safe to call this method multiple times
func (l *LeaderFacade) Stop(_ context.Context) {
	l.cancel()
	l.wg.Wait()
}
