package raft

import (
	"context"

	"github.com/escalopa/raft-kv/internal/core"
	desc "github.com/escalopa/raft-kv/pkg/raft"
)

func (i *Implementation) Info(ctx context.Context, req *desc.InfoRequest) (*desc.InfoResponse, error) {
	resp, err := i.srv.Info(ctx, req)
	if err != nil {
		return nil, core.ToGrpcError(err)
	}
	return resp, nil
}
