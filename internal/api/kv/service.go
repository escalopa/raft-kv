package kv

import (
  "github.com/catalystgo/catalystgo"
  desc "github.com/escalopa/raft-kv/pkg/kv"
)

type Implementation struct {
  desc.UnimplementedKVServer
}

func NewKV() *Implementation {
  return &Implementation{}
}

func (i *Implementation) GetDescription() catalystgo.ServiceDesc {
  return desc.NewKVServiceDesc(i)
}
