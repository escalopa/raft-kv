package service

import (
	"context"
	"sync"

	"github.com/escalopa/raft-kv/internal/core"
	desc "github.com/escalopa/raft-kv/pkg/raft"
)

type appendEntriesRequest struct {
	ctx context.Context
	req *desc.AppendEntriesRequest
	res chan *desc.AppendEntriesResponse
	err chan error
}

func (aer appendEntriesRequest) reply(res *desc.AppendEntriesResponse, err error) {
	if err == nil {
		aer.res <- res
		return
	}
	aer.err <- err
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

func (rvr requestVoteRequest) reply(res *desc.RequestVoteResponse, err error) {
	if err == nil {
		rvr.res <- res
		return
	}
	rvr.err <- err
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

func (rr replicateRequest) reply(err error) {
	rr.err <- err
}

func newReplicateRequest(ctx context.Context, data []string) *replicateRequest {
	return &replicateRequest{
		ctx:  ctx,
		data: data,
		err:  make(chan error),
	}
}

type replicateResponse struct {
	raftID core.ServerID
	res    *desc.AppendEntriesResponse
}

var tryCloseLock = sync.Mutex{} // tryCloseLock is used to prevent multiple close calls on the same channel

func tryClose(ch chan struct{}) {
	tryCloseLock.Lock()
	defer tryCloseLock.Unlock()

	select {
	case <-ch: // already closed
	default:
		close(ch)
	}
}
