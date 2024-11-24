package kv

import (
	"context"

	"github.com/catalystgo/catalystgo"
	desc "github.com/escalopa/raft-kv/pkg/kv"
)

type Service interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key string, value string) error
	Del(ctx context.Context, key string) error
}

type Implementation struct {
	srv Service

	desc.UnimplementedKVServiceServer
}

func NewKVService(srv Service) *Implementation {
	return &Implementation{srv: srv}
}

func (i *Implementation) GetDescription() catalystgo.ServiceDesc {
	return desc.NewKVServiceServiceDesc(i)
}
