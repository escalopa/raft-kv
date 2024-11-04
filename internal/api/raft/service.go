package raft

import (
  "github.com/catalystgo/catalystgo"
  desc "github.com/escalopa/raft-kv/pkg/raft"
)

type Implementation struct {
  desc.UnimplementedRaftServer
}

func NewRaft() *Implementation {
  return &Implementation{}
}

func (i *Implementation) GetDescription() catalystgo.ServiceDesc {
  return desc.NewRaftServiceDesc(i)
}
