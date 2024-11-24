package kv

import (
	"context"

	"github.com/escalopa/raft-kv/internal/core"
	desc "github.com/escalopa/raft-kv/pkg/kv"
)

func (i *Implementation) Del(ctx context.Context, req *desc.DelRequest) (*desc.DelResponse, error) {
	err := i.srv.Del(ctx, req.GetKey())
	if err != nil {
		return nil, core.ToGrpcError(err)
	}
	return &desc.DelResponse{}, nil
}
