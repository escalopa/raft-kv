package service

import (
	"context"
	"sync"
	"time"

	"github.com/escalopa/raft-kv/internal/core"
	desc "github.com/escalopa/raft-kv/pkg/raft"
	"github.com/puzpuzpuz/xsync/v3"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var (
	runOnce = sync.Once{}
)

type (
	EntryStore interface {
		AppendEntries(ctx context.Context, entries ...*core.Entry) (err error)
		Last(ctx context.Context) (entry *core.Entry, err error)
		At(ctx context.Context, index uint64) (entry *core.Entry, err error)
		Range(ctx context.Context, from uint64, to uint64) (entries []*core.Entry, err error)
	}

	StateStore interface {
		GetTerm(ctx context.Context) (term uint64, err error)
		SetTerm(ctx context.Context, term uint64) (err error)

		GetCommit(ctx context.Context) (commitIndex uint64, err error)
		SetCommit(ctx context.Context, commitIndex uint64) (err error)

		GetLastApplied(ctx context.Context) (lastApplied uint64, err error)
		SetLastApplied(ctx context.Context, lastApplied uint64) (err error)

		GetVotedFor(ctx context.Context) (votedFor uint64, err error)
		SetVotedFor(ctx context.Context, votedFor uint64) (err error)
	}

	KVStore interface {
		Get(ctx context.Context, key string) (value string, err error)
		Set(ctx context.Context, key, value string) (err error)
		Del(ctx context.Context, key string) (err error)
	}

	Config interface {
		// general

		GetRequestVoteTimeout() time.Duration
		GetAppendEntriesTimeout() time.Duration

		GetStartDelay() time.Duration
		GetCommitPeriod() time.Duration
		GetElectionTimeout() time.Duration

		// leader

		GetLeaderStalePeriod() time.Duration
		GetLeaderHeartbeatPeriod() time.Duration
	}
)

type (
	leaderState struct {
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
	}

	Raft struct {
		ctx context.Context

		config Config

		raftID core.ServerID

		state  *raftState
		leader *leaderState

		quorum uint32

		requestChan chan interface{}

		servers map[core.ServerID]desc.RaftServiceClient

		entryStore EntryStore
		stateStore StateStore
		kvStore    KVStore

		shutdownChan chan struct{} // channel to signal the raft state to shut down
		wg           sync.WaitGroup
	}
)

func NewRaftState(
	ctx context.Context,
	config Config,
	raftID core.ServerID,
	cluster []core.Node,
	entryStore EntryStore,
	stateStore StateStore,
	kvStore KVStore,
) (*Raft, error) {
	serversCount := len(cluster) + 1         // +1 for the current node
	quorum := uint32((serversCount / 2) + 1) // +1 to get the majority

	rf := &Raft{
		ctx: ctx,

		config: config,

		raftID: raftID,
		quorum: quorum,

		servers: make(map[core.ServerID]desc.RaftServiceClient),

		requestChan: make(chan interface{}),

		entryStore: entryStore,
		stateStore: stateStore,
		kvStore:    kvStore,

		shutdownChan: make(chan struct{}),
	}

	err := rf.initCluster(cluster)
	if err != nil {
		return nil, err
	}

	rf.state, err = newRaftState(ctx, stateStore, entryStore)
	if err != nil {
		return nil, err
	}

	return rf, nil
}

func newLeaderState() *leaderState {
	return &leaderState{
		nextIndex:     xsync.NewMapOf[core.ServerID, uint64](),
		matchIndex:    xsync.NewMapOf[core.ServerID, uint64](),
		lastHeartbeat: xsync.NewMapOf[core.ServerID, time.Time](),
	}
}

func (r *Raft) Run() {
	runOnce.Do(func() {
		r.goFunc(r.run)
		r.goFunc(r.commit)
	})
}

func (r *Raft) run() {
	<-time.After(r.config.GetStartDelay()) // wait X time before the start of the cluster

	for {
		select {
		case <-r.shutdownChan: // stop the raft state
			return
		default:
		}

		switch r.state.GetState() {
		case FollowerState:
			r.runFollower()
		case CandidateState:
			r.runCandidate()
		case LeaderState:
			r.runLeader()
		}
	}
}

func (r *Raft) initCluster(cluster []core.Node) error {
	for _, node := range cluster {
		conn, err := grpc.NewClient(node.Address, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			return err
		}
		r.servers[node.ID] = desc.NewRaftServiceClient(conn)
	}
	return nil
}

func (r *Raft) Get(ctx context.Context, key string) (string, error) {
	if !r.state.IsLeader() {
		return "", core.ErrNotLeader
	}
	return r.kvStore.Get(ctx, key)
}

func (r *Raft) Set(ctx context.Context, key string, value string) error {
	if !r.state.IsLeader() {
		return core.ErrNotLeader
	}

	data := []string{core.Set.String(), key, value}

	request := newReplicateRequest(ctx, data)
	r.requestChan <- request
	return <-request.err
}

func (r *Raft) Del(ctx context.Context, key string) error {
	if !r.state.IsLeader() {
		return core.ErrNotLeader
	}

	data := []string{core.Del.String(), key}

	request := newReplicateRequest(ctx, data)
	r.requestChan <- request
	return <-request.err
}

func (r *Raft) AppendEntries(ctx context.Context, req *desc.AppendEntriesRequest) (*desc.AppendEntriesResponse, error) {
	request := newAppendEntriesRequest(ctx, req)
	r.requestChan <- request
	select {
	case res := <-request.res:
		return res, nil
	case err := <-request.err:
		return nil, err
	}
}

func (r *Raft) RequestVote(ctx context.Context, req *desc.RequestVoteRequest) (*desc.RequestVoteResponse, error) {
	request := newRequestVoteRequest(ctx, req)
	r.requestChan <- request
	select {
	case res := <-request.res:
		return res, nil
	case err := <-request.err:
		return nil, err
	}
}

func (r *Raft) Info(_ context.Context, _ *desc.InfoRequest) (*desc.InfoResponse, error) {
	lastLogIndex, lastLogTerm := r.state.GetLastLog()
	return &desc.InfoResponse{
		Term:         r.state.GetTerm(),
		CommitIndex:  r.state.GetCommitIndex(),
		LastApplied:  r.state.GetLastApplied(),
		LastLogIndex: lastLogIndex,
		LastLogTerm:  lastLogTerm,
		State:        r.state.GetState().String(),
	}, nil
}

func (r *Raft) goFunc(f func()) {
	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		f()
	}()
}

func (r *Raft) Close() error {
	close(r.shutdownChan)
	r.wg.Wait()
	return nil
}
