package storage

import (
	"github.com/dgraph-io/badger/v4"
	"github.com/escalopa/raft-kv/internal/core"
	"github.com/pkg/errors"
)

func getEntry(txn *badger.Txn, key []byte) (*core.Entry, error) {
	item, err := txn.Get(key)
	if err != nil {
		if errors.Is(err, badger.ErrKeyNotFound) {
			return nil, core.ErrNotFound
		}

		return nil, err
	}

	b, err := item.ValueCopy(nil)
	if err != nil {
		return nil, err
	}

	entry, err := core.EntryFromBytes(b)
	if err != nil {
		return nil, err
	}

	return entry, nil
}

func getUint64(txn *badger.Txn, key []byte) (uint64, error) {
	item, err := txn.Get(key)
	if err != nil {
		if errors.Is(err, badger.ErrKeyNotFound) {
			return 0, core.ErrNotFound
		}

		return 0, err
	}

	b, err := item.ValueCopy(nil)
	if err != nil {
		return 0, err
	}

	return core.KeyToUint(b), nil
}
