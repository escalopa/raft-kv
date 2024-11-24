package test

import (
	"testing"

	"github.com/dgraph-io/badger/v4"
	"github.com/stretchr/testify/require"
)

func NewDB(t *testing.T) (*badger.DB, func(t *testing.T)) {
	opts := badger.DefaultOptions("").
		WithInMemory(true).
		WithLoggingLevel(badger.INFO)

	db, err := badger.Open(opts)
	require.NoError(t, err)

	closer := func(t *testing.T) {
		require.NoError(t, db.Close())
	}

	return db, closer
}
