package service

import (
	"context"
	"sync"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/escalopa/raft-kv/internal/core"
	desc "github.com/escalopa/raft-kv/pkg/raft"
)

var (
	initServersOnce = sync.Once{}
)

type EntryStore interface {
	AppendEntry(entries ...*core.Entry) (err error)
	Last() (entry *core.Entry, err error)
	At(uint64) (entry *core.Entry, err error)
	Range(uint64, uint64) (entries []*core.Entry, err error)
}

type StateStore interface {
	GetTerm() (term uint64, err error)
	SetTerm(term uint64) (err error)
	GetVoted() (votedFor uint64, err error)
	SetVoted(votedFor uint64) (err error)
	GetCommit() (commitIndex uint64, err error)
	SetCommit(commitIndex uint64) (err error)
	GetLastApplied() (lastApplied uint64, err error)
	SetLastApplied(lastApplied uint64) (err error)
}

type KVStore interface {
	Get(key string) (value string, err error)
	Set(key, value string) (err error)
	Del(key string) (err error)
}

type State struct {
	raftID core.ServerID

	// Persistent state

	term        uint64
	votedFor    uint64
	commitIndex uint64
	lastApplied uint64
	leaderID    core.ServerID

	stateMutex sync.RWMutex

	// Leader attributes

	cluster    []core.Node
	servers    map[core.ServerID]desc.RaftServiceClient
	nextIndex  map[core.ServerID]uint64 // For each server, index of the next log entry to send to that server
	matchIndex map[core.ServerID]uint64 // For each server, index of highest log entry known to be replicated on server

	// Data stores

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
) (*State, error) {
	s := &State{
		raftID:     raftID,
		cluster:    cluster,
		entryStore: entryStore,
		stateStore: stateStore,
		kvStore:    kvStore,
		servers:    make(map[core.ServerID]desc.RaftServiceClient),
		nextIndex:  make(map[core.ServerID]uint64),
		matchIndex: make(map[core.ServerID]uint64),
	}

	err := s.initServers()
	if err != nil {
		return nil, err
	}

	return s, nil
}

func (s *State) AppendEntry(ctx context.Context, req *desc.AppendEntryRequest) (*desc.AppendEntryResponse, error) {
	return &desc.AppendEntryResponse{}, nil
}

func (s *State) RequestVote(ctx context.Context, req *desc.RequestVoteRequest) (*desc.RequestVoteResponse, error) {
	return &desc.RequestVoteResponse{}, nil
}

func (s *State) Get(ctx context.Context, key string) (string, error) {
	return "", nil
}

func (s *State) Set(ctx context.Context, key string, value string) error {
	return nil
}

func (s *State) Del(ctx context.Context, key string) error {
	return nil
}

func (s *State) IsLeader() bool {
	return s.leaderID == s.raftID
}

// initServers initializes gRPC clients for all servers in the cluster
func (s *State) initServers() error {
	var (
		conn *grpc.ClientConn
		err  error
	)

	fn := func() {
		for _, node := range s.cluster {
			conn, err = grpc.NewClient(node.Address, grpc.WithTransportCredentials(insecure.NewCredentials()))
			if err != nil {
				return
			}
			s.servers[node.ID] = desc.NewRaftServiceClient(conn)
		}
	}

	initServersOnce.Do(fn)
	return err
}
