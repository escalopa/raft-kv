package kv

import (
	"context"

	"github.com/escalopa/raft-kv/internal/core"
	desc "github.com/escalopa/raft-kv/pkg/kv"
)

func (i *Implementation) Set(ctx context.Context, req *desc.SetRequest) (*desc.SetResponse, error) {
	err := i.srv.Set(ctx, req.GetKey(), req.GetValue())
	if err != nil {
		return nil, core.ToGrpcError(err)
	}
	return &desc.SetResponse{}, nil
}
