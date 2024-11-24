package storage

import (
	"github.com/dgraph-io/badger/v4"
	"github.com/escalopa/raft-kv/internal/core"
)

var (
	termKey    = []byte("term_key")
	commitKey  = []byte("commit_key")
	appliedKey = []byte("applied_key")
	votedKey   = []byte("voted_key")
)

type StateStore struct {
	db *badger.DB
}

func NewStateStore(db *badger.DB) *StateStore {
	return &StateStore{db: db}
}

func (ss *StateStore) SetTerm(term uint64) error {
	return ss.db.Update(func(txn *badger.Txn) error {
		return txn.Set(termKey, core.UintToKey(term))
	})
}

func (ss *StateStore) GetTerm() (term uint64, err error) {
	err = ss.db.View(func(txn *badger.Txn) error {
		term, err = getUint64(txn, termKey)
		return err
	})
	return
}

func (ss *StateStore) SetCommit(index uint64) error {
	return ss.db.Update(func(txn *badger.Txn) error {
		return txn.Set(commitKey, core.UintToKey(index))
	})
}

func (ss *StateStore) GetCommit() (index uint64, err error) {
	err = ss.db.View(func(txn *badger.Txn) error {
		index, err = getUint64(txn, commitKey)
		return err
	})
	return
}

func (ss *StateStore) SetVoted(index uint64) error {
	return ss.db.Update(func(txn *badger.Txn) error {
		return txn.Set(votedKey, core.UintToKey(index))
	})
}

func (ss *StateStore) GetVoted() (index uint64, err error) {
	err = ss.db.View(func(txn *badger.Txn) error {
		index, err = getUint64(txn, votedKey)
		return err
	})
	return
}

func (ss *StateStore) SetLastApplied(index uint64) error {
	return ss.db.Update(func(txn *badger.Txn) error {
		return txn.Set(appliedKey, core.UintToKey(index))
	})
}

func (ss *StateStore) GetLastApplied() (index uint64, err error) {
	err = ss.db.View(func(txn *badger.Txn) error {
		index, err = getUint64(txn, appliedKey)
		return err
	})
	return
}
