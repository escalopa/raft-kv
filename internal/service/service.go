package service

import (
	"context"
	"sync"

	"github.com/escalopa/raft-kv/internal/core"
	"github.com/escalopa/raft-kv/internal/service/internal"
	desc "github.com/escalopa/raft-kv/pkg/raft"
	"github.com/pkg/errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var (
	initServersOnce = sync.Once{}
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

		GetVoted(ctx context.Context) (votedFor uint64, err error)
		SetVoted(ctx context.Context, votedFor uint64) (err error)

		GetCommit(ctx context.Context) (commitIndex uint64, err error)
		SetCommit(ctx context.Context, commitIndex uint64) (err error)

		GetLastApplied(ctx context.Context) (lastApplied uint64, err error)
		SetLastApplied(ctx context.Context, lastApplied uint64) (err error)
	}

	KVStore interface {
		Get(ctx context.Context, key string) (value string, err error)
		Set(ctx context.Context, key, value string) (err error)
		Del(ctx context.Context, key string) (err error)
	}
)

type RaftState struct {
	ctx    context.Context
	cancel context.CancelFunc

	raftID core.ServerID

	state *internal.StateFacade

	quorum uint32

	appendEntriesChan chan *appendEntriesRequest
	requestVoteChan   chan *requestVoteRequest
	replicateChan     chan *replicateRequest

	stateUpdateChan chan<- *core.StateUpdate

	servers map[core.ServerID]desc.RaftServiceClient

	heartbeat chan struct{}

	entryStore EntryStore
	stateStore StateStore
	kvStore    KVStore

	wg sync.WaitGroup
}

func NewRaftState(
	ctx context.Context,
	raftID core.ServerID,
	cluster []core.Node,
	entryStore EntryStore,
	stateStore StateStore,
	kvStore KVStore,
) (*RaftState, error) {
	ctx, cancel := context.WithCancel(ctx)

	serversCount := len(cluster) + 1         // +1 for the current node
	quorum := uint32((serversCount / 2) + 1) // +1 to get the majority

	rf := &RaftState{
		ctx:    ctx,
		cancel: cancel,

		raftID: raftID,

		servers: make(map[core.ServerID]desc.RaftServiceClient),

		appendEntriesChan: make(chan *appendEntriesRequest),
		requestVoteChan:   make(chan *requestVoteRequest),
		replicateChan:     make(chan *replicateRequest),

		quorum: quorum,

		heartbeat: make(chan struct{}),

		entryStore: entryStore,
		stateStore: stateStore,
		kvStore:    kvStore,
	}

	err := rf.initServers(cluster)
	if err != nil {
		return nil, err
	}

	stateUpdateChan := make(chan *core.StateUpdate)
	rf.stateUpdateChan = stateUpdateChan

	rf.state, err = internal.NewStateFacade(ctx, stateStore, stateUpdateChan)
	if err != nil {
		return nil, err
	}

	leaderFacade := internal.NewLeaderFacade(raftID, rf.servers, entryStore, rf.state, stateUpdateChan)
	rf.state.SetLeader(leaderFacade)

	rf.wg.Add(5)

	go rf.processAppendEntries()
	go rf.processRequestVote()
	go rf.processReplicate()
	go rf.processCommit()
	go rf.processElection()

	return rf, nil
}

func (rf *RaftState) initServers(cluster []core.Node) error {
	var (
		conn *grpc.ClientConn
		err  error
	)

	fn := func() {
		for _, node := range cluster {
			conn, err = grpc.NewClient(node.Address, grpc.WithTransportCredentials(insecure.NewCredentials()))
			if err != nil {
				return
			}
			rf.servers[node.ID] = desc.NewRaftServiceClient(conn)
		}
	}

	initServersOnce.Do(fn)
	return err
}

func (rf *RaftState) Get(ctx context.Context, key string) (string, error) {
	if !rf.state.IsLeader() {
		return "", core.ErrNotLeader
	}
	return rf.kvStore.Get(ctx, key)
}

func (rf *RaftState) Set(ctx context.Context, key string, value string) error {
	if !rf.state.IsLeader() {
		return core.ErrNotLeader
	}

	data := []string{core.Set.String(), key, value}

	request := newReplicateRequest(ctx, data)
	rf.replicateChan <- request
	return <-request.err
}

func (rf *RaftState) Del(ctx context.Context, key string) error {
	if !rf.state.IsLeader() {
		return core.ErrNotLeader
	}

	data := []string{core.Del.String(), key}

	request := newReplicateRequest(ctx, data)
	rf.replicateChan <- request
	return <-request.err
}

func (rf *RaftState) AppendEntries(ctx context.Context, req *desc.AppendEntriesRequest) (*desc.AppendEntriesResponse, error) {
	request := newAppendEntriesRequest(ctx, req)
	rf.appendEntriesChan <- request
	select {
	case res := <-request.res:
		return res, nil
	case err := <-request.err:
		return nil, err
	}
}

func (rf *RaftState) RequestVote(ctx context.Context, req *desc.RequestVoteRequest) (*desc.RequestVoteResponse, error) {
	request := newRequestVoteRequest(ctx, req)
	rf.requestVoteChan <- request
	select {
	case res := <-request.res:
		return res, nil
	case err := <-request.err:
		return nil, err
	}
}

func (rf *RaftState) Info(ctx context.Context, _ *desc.InfoRequest) (*desc.InfoResponse, error) {
	entry, err := rf.entryStore.Last(ctx)
	if err != nil {
		if !errors.Is(err, core.ErrNotFound) {
			return nil, err
		}
		entry = &core.Entry{}
	}

	return &desc.InfoResponse{
		Term:         rf.state.GetTerm(),
		CommitIndex:  rf.state.GetCommitIndex(),
		LastApplied:  rf.state.GetLastApplied(),
		LastLogIndex: entry.Index,
		LastLogTerm:  entry.Term,
		State:        rf.state.GetState().String(),
	}, nil
}

func (rf *RaftState) sendStateUpdate(update core.StateUpdate) {
	update.Done = make(chan struct{})
	rf.stateUpdateChan <- &update
	<-update.Done // wait for the update to be processed
}

func (rf *RaftState) resetElectionTimer() {
	select {
	case rf.heartbeat <- struct{}{}:
	default: // drop the heartbeat if it's not currently needed
	}
}

func (rf *RaftState) Close() {
	rf.cancel()
	rf.wg.Wait()
}
