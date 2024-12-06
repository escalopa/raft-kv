package storage

import (
	"context"

	"github.com/dgraph-io/badger/v4"
	"github.com/escalopa/raft-kv/internal/core"
)

var (
	termKey     = []byte("term_key")
	commitKey   = []byte("commit_key")
	appliedKey  = []byte("last_applied_key")
	votedFroKey = []byte("voted_for_key")
)

type StateStore struct {
	db *badger.DB
}

func NewStateStore(db *badger.DB) *StateStore {
	return &StateStore{db: db}
}

func (ss *StateStore) SetTerm(ctx context.Context, term uint64) error {
	if isDeadCtx(ctx) {
		return ctx.Err()
	}

	return ss.db.Update(func(txn *badger.Txn) error {
		return txn.Set(termKey, core.UintToKey(term))
	})
}

func (ss *StateStore) GetTerm(ctx context.Context) (term uint64, err error) {
	if isDeadCtx(ctx) {
		return 0, ctx.Err()
	}

	err = ss.db.View(func(txn *badger.Txn) error {
		term, err = getUint64(txn, termKey)
		return err
	})
	return
}

func (ss *StateStore) SetCommit(ctx context.Context, index uint64) error {
	if isDeadCtx(ctx) {
		return ctx.Err()
	}

	return ss.db.Update(func(txn *badger.Txn) error {
		return txn.Set(commitKey, core.UintToKey(index))
	})
}

func (ss *StateStore) GetCommit(ctx context.Context) (index uint64, err error) {
	if isDeadCtx(ctx) {
		return 0, ctx.Err()
	}

	err = ss.db.View(func(txn *badger.Txn) error {
		index, err = getUint64(txn, commitKey)
		return err
	})
	return
}

func (ss *StateStore) SetLastApplied(ctx context.Context, index uint64) error {
	if isDeadCtx(ctx) {
		return ctx.Err()
	}

	return ss.db.Update(func(txn *badger.Txn) error {
		return txn.Set(appliedKey, core.UintToKey(index))
	})
}

func (ss *StateStore) GetLastApplied(ctx context.Context) (index uint64, err error) {
	if isDeadCtx(ctx) {
		return 0, ctx.Err()
	}

	err = ss.db.View(func(txn *badger.Txn) error {
		index, err = getUint64(txn, appliedKey)
		return err
	})
	return
}

func (ss *StateStore) SetVotedFor(ctx context.Context, serverID uint64) error {
	if isDeadCtx(ctx) {
		return ctx.Err()
	}

	return ss.db.Update(func(txn *badger.Txn) error {
		return txn.Set(votedFroKey, core.UintToKey(serverID))
	})
}

func (ss *StateStore) GetVotedFor(ctx context.Context) (serverID uint64, err error) {
	if isDeadCtx(ctx) {
		return 0, ctx.Err()
	}

	err = ss.db.View(func(txn *badger.Txn) error {
		serverID, err = getUint64(txn, votedFroKey)
		return err
	})
	return
}
