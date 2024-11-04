package kv

import (
  "context"

  desc "github.com/escalopa/raft-kv/pkg/kv"
  "google.golang.org/grpc/codes"
  "google.golang.org/grpc/status"
)

func (i *Implementation) Get(ctx context.Context, req *desc.GetRequest) (*desc.GetResponse, error) {
  return nil, status.Errorf(codes.Unimplemented, `method "Get" not implemented`)
}
