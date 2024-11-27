package storage

import (
	"context"

	"github.com/dgraph-io/badger/v4"
)

func NewDB(path string) (*badger.DB, error) {
	opts := badger.
		DefaultOptions(path).
		WithLoggingLevel(badger.WARNING) // TODO: set from app config

	return badger.Open(opts)
}

// isDeadCtx checks if the context is dead (dead == has an error)
func isDeadCtx(ctx context.Context) bool {
	return ctx.Err() != nil
}
