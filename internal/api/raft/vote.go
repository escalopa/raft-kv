package raft

import (
  "context"

  desc "github.com/escalopa/raft-kv/pkg/raft"
  "google.golang.org/grpc/codes"
  "google.golang.org/grpc/status"
)

func (i *Implementation) Vote(ctx context.Context, req *desc.VoteRequest) (*desc.VoteResponse, error) {
  return nil, status.Errorf(codes.Unimplemented, `method "Vote" not implemented`)
}
