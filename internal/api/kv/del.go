package kv

import (
  "context"

  desc "github.com/escalopa/raft-kv/pkg/kv"
  "google.golang.org/grpc/codes"
  "google.golang.org/grpc/status"
)

func (i *Implementation) Del(ctx context.Context, req *desc.DelRequest) (*desc.DelResponse, error) {
  return nil, status.Errorf(codes.Unimplemented, `method "Del" not implemented`)
}
