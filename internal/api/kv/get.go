package kv

import (
	"context"

	"github.com/escalopa/raft-kv/internal/core"
	desc "github.com/escalopa/raft-kv/pkg/kv"
)

func (i *Implementation) Get(ctx context.Context, req *desc.GetRequest) (*desc.GetResponse, error) {
	value, err := i.srv.Get(ctx, req.GetKey())
	if err != nil {
		return nil, core.ToGrpcError(err)
	}
	return &desc.GetResponse{Value: value}, nil
}
