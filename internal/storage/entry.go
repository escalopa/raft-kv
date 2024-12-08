package storage

import (
	"context"

	"github.com/dgraph-io/badger/v4"
	"github.com/escalopa/raft-kv/internal/core"
	"github.com/pkg/errors"
)

var (
	logKey = []byte("log_key")
)

type EntryStore struct {
	db *badger.DB
}

func NewEntryStore(db *badger.DB) *EntryStore {
	return &EntryStore{db: db}
}

func (es *EntryStore) AppendEntries(ctx context.Context, entries ...*core.Entry) error {
	if isDeadCtx(ctx) {
		return ctx.Err()
	}

	if len(entries) == 0 {
		return nil
	}

	return es.db.Update(func(txn *badger.Txn) error {
		var (
			err  error
			data []byte
		)

		// append the entries
		for _, entry := range entries {
			key := core.UintToKey(entry.Index)
			data, err = entry.ToBytes()
			if err != nil {
				return err
			}

			err = txn.Set(key, data)
			if err != nil {
				return err
			}
		}

		// update the last log key
		last := entries[len(entries)-1]
		err = txn.Set(logKey, core.UintToKey(last.Index))
		if err != nil {
			return err
		}

		return nil
	})
}

func (es *EntryStore) Last(ctx context.Context) (entry *core.Entry, err error) {
	if isDeadCtx(ctx) {
		return nil, ctx.Err()
	}

	err = es.db.View(func(txn *badger.Txn) error {
		index, err := getUint64(txn, logKey)
		if err != nil {
			return err
		}

		entry, err = getEntry(txn, core.UintToKey(index))
		if err != nil {
			return err
		}

		return nil
	})
	return
}

func (es *EntryStore) At(ctx context.Context, index uint64) (entry *core.Entry, err error) {
	if isDeadCtx(ctx) {
		return nil, ctx.Err()
	}

	err = es.db.View(func(txn *badger.Txn) error {
		entry, err = getEntry(txn, core.UintToKey(index))
		return err
	})
	return
}

func (es *EntryStore) Range(ctx context.Context, start, end uint64) (entries []*core.Entry, err error) {
	if isDeadCtx(ctx) {
		return nil, ctx.Err()
	}

	err = es.db.View(func(txn *badger.Txn) error {
		if end < start {
			return nil
		}

		size := end - start + 1

		entries = make([]*core.Entry, 0, size)
		for i := range size {
			index := start + i

			entry, err := getEntry(txn, core.UintToKey(index))
			if err != nil {
				if errors.Is(err, core.ErrNotFound) {
					return nil // stop the iteration
				}

				return err
			}

			entries = append(entries, entry)
		}

		return nil
	})
	return
}
