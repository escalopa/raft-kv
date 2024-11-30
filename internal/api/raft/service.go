package raft

import (
	"context"

	"github.com/catalystgo/catalystgo"
	desc "github.com/escalopa/raft-kv/pkg/raft"
)

type Service interface {
	AppendEntries(ctx context.Context, req *desc.AppendEntriesRequest) (*desc.AppendEntriesResponse, error)
	RequestVote(ctx context.Context, req *desc.RequestVoteRequest) (*desc.RequestVoteResponse, error)
	Info(ctx context.Context, req *desc.InfoRequest) (*desc.InfoResponse, error)
}

type Implementation struct {
	srv Service

	desc.UnimplementedRaftServiceServer
}

func NewRaftService(srv Service) *Implementation {
	return &Implementation{srv: srv}
}

func (i *Implementation) GetDescription() catalystgo.ServiceDesc {
	return desc.NewRaftServiceServiceDesc(i)
}
