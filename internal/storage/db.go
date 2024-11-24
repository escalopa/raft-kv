package storage

import (
	"github.com/dgraph-io/badger/v4"
)

func NewDB(path string) (*badger.DB, error) {
	opts := badger.
		DefaultOptions(path).
		WithLoggingLevel(badger.WARNING) // TODO: set from app config

	return badger.Open(opts)
}
