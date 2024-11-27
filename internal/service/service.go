package service

import (
	"context"
	"sync"

	"github.com/escalopa/raft-kv/internal/service/internal"

	"github.com/escalopa/raft-kv/internal/core"
	desc "github.com/escalopa/raft-kv/pkg/raft"
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
	ctx context.Context

	raftID core.ServerID

	state  *internal.StateFacade
	leader *internal.LeaderFacade

	quorum   uint32
	leaderID core.ServerID

	appendEntriesChan chan *appendEntriesRequest
	requestVoteChan   chan *requestVoteRequest
	replicateChan     chan *replicateRequest

	stateUpdateChan chan<- core.StateUpdate

	servers map[core.ServerID]desc.RaftServiceClient

	heartbeat chan struct{}

	entryStore EntryStore
	stateStore StateStore
	kvStore    KVStore
}

func NewRaftState(
	ctx context.Context,
	raftID core.ServerID,
	cluster []core.Node,
	entryStore EntryStore,
	stateStore StateStore,
	kvStore KVStore,
) (*RaftState, error) {
	serversCount := len(cluster) + 1         // +1 for the current node
	quorum := uint32((serversCount / 2) + 1) // +1 to get the majority

	rf := &RaftState{
		ctx: ctx,

		raftID: raftID,

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

	stateUpdateChan := make(chan core.StateUpdate)
	rf.stateUpdateChan = stateUpdateChan

	rf.state = internal.NewStateFacade(ctx, stateStore, stateUpdateChan)
	rf.leader = internal.NewLeaderFacade(raftID, rf.servers, entryStore, rf.state, stateUpdateChan)
	// TODO: think how to stop leader on state change
	// TODO: think how and when to commit entries

	go rf.processAppendEntries()
	go rf.processRequestVote()
	go rf.processReplicate()

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

func (rf *RaftState) updateState(update core.StateUpdate) {
	update.Done = make(chan struct{})
	rf.stateUpdateChan <- update
	<-update.Done // wait for the update to be processed
}

func (rf *RaftState) resetElectionTimer() {
	rf.heartbeat <- struct{}{}
}
