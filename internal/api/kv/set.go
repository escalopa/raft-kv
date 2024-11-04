package kv

import (
  "context"

  desc "github.com/escalopa/raft-kv/pkg/kv"
  "google.golang.org/grpc/codes"
  "google.golang.org/grpc/status"
)

func (i *Implementation) Set(ctx context.Context, req *desc.SetRequest) (*desc.SetResponse, error) {
  return nil, status.Errorf(codes.Unimplemented, `method "Set" not implemented`)
}
