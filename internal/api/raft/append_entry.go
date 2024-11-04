package raft

import (
  "context"

  desc "github.com/escalopa/raft-kv/pkg/raft"
  "google.golang.org/grpc/codes"
  "google.golang.org/grpc/status"
)

func (i *Implementation) AppendEntry(ctx context.Context, req *desc.AppendEntryRequest) (*desc.AppendEntryResponse, error) {
  return nil, status.Errorf(codes.Unimplemented, `method "AppendEntry" not implemented`)
}
