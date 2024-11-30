package raft

import (
	"context"

	"github.com/escalopa/raft-kv/internal/core"
	desc "github.com/escalopa/raft-kv/pkg/raft"
)

func (i *Implementation) AppendEntries(ctx context.Context, req *desc.AppendEntriesRequest) (*desc.AppendEntriesResponse, error) {
	resp, err := i.srv.AppendEntries(ctx, req)
	if err != nil {
		return nil, core.ToGrpcError(err)
	}
	return resp, nil
}
