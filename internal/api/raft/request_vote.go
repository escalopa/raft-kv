package raft

import (
	"context"

	"github.com/escalopa/raft-kv/internal/core"
	desc "github.com/escalopa/raft-kv/pkg/raft"
)

func (i *Implementation) RequestVote(ctx context.Context, req *desc.RequestVoteRequest) (*desc.RequestVoteResponse, error) {
	resp, err := i.srv.RequestVote(ctx, req)
	if err != nil {
		return nil, core.ToGrpcError(err)
	}
	return resp, nil
}
