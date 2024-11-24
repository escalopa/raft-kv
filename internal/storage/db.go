package storage

import (
	"github.com/dgraph-io/badger/v4"
)

func NewDB(path string) (*badger.DB, error) {
	opts := badger.DefaultOptions(path)
	return badger.Open(opts)
}
