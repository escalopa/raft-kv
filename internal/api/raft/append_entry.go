package raft

import (
	"context"

	"github.com/escalopa/raft-kv/internal/core"
	desc "github.com/escalopa/raft-kv/pkg/raft"
)

func (i *Implementation) AppendEntry(ctx context.Context, req *desc.AppendEntryRequest) (*desc.AppendEntryResponse, error) {
	resp, err := i.srv.AppendEntry(ctx, req)
	if err != nil {
		return nil, core.ToGrpcError(err)
	}
	return resp, nil
}
