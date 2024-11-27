package storage

import (
	"context"

	"github.com/dgraph-io/badger/v4"
	"github.com/escalopa/raft-kv/internal/core"
	"github.com/pkg/errors"
)

type KVStore struct {
	db *badger.DB
}

func NewKVStore(db *badger.DB) *KVStore {
	return &KVStore{db: db}
}

func (kvs *KVStore) Get(ctx context.Context, key string) (string, error) {
	if isDeadCtx(ctx) {
		return "", ctx.Err()
	}

	var value []byte

	err := kvs.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(key))
		if err != nil {
			if errors.Is(err, badger.ErrKeyNotFound) {
				return core.ErrNotFound
			}

			return err
		}

		value, err = item.ValueCopy(nil)
		if err != nil {
			return err
		}

		return nil
	})

	return string(value), err
}

func (kvs *KVStore) Set(ctx context.Context, key, value string) error {
	if isDeadCtx(ctx) {
		return ctx.Err()
	}

	return kvs.db.Update(func(txn *badger.Txn) error {
		return txn.Set([]byte(key), []byte(value))
	})
}

func (kvs *KVStore) Del(ctx context.Context, key string) error {
	if isDeadCtx(ctx) {
		return ctx.Err()
	}

	return kvs.db.Update(func(txn *badger.Txn) error {
		return txn.Delete([]byte(key))
	})
}
