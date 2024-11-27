package service

import (
	"context"

	desc "github.com/escalopa/raft-kv/pkg/raft"
)

type appendEntriesRequest struct {
	ctx context.Context
	req *desc.AppendEntriesRequest
	res chan *desc.AppendEntriesResponse
	err chan error
}

func newAppendEntriesRequest(ctx context.Context, req *desc.AppendEntriesRequest) *appendEntriesRequest {
	return &appendEntriesRequest{
		ctx: ctx,
		req: req,
		res: make(chan *desc.AppendEntriesResponse),
		err: make(chan error),
	}
}

type requestVoteRequest struct {
	ctx context.Context
	req *desc.RequestVoteRequest
	res chan *desc.RequestVoteResponse
	err chan error
}

func newRequestVoteRequest(ctx context.Context, req *desc.RequestVoteRequest) *requestVoteRequest {
	return &requestVoteRequest{
		ctx: ctx,
		req: req,
		res: make(chan *desc.RequestVoteResponse),
		err: make(chan error),
	}
}

type replicateRequest struct {
	ctx  context.Context
	data []string
	err  chan error
}

func newReplicateRequest(ctx context.Context, data []string) *replicateRequest {
	return &replicateRequest{
		ctx:  ctx,
		data: data,
		err:  make(chan error),
	}
}
