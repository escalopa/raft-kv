package storage

import (
	"context"
	"testing"

	"github.com/escalopa/raft-kv/test"
	"github.com/stretchr/testify/require"
)

func TestStore_SetTerm(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		term uint64
	}{
		{
			name: "set_term",
			term: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			db, closer := test.NewDB(t)
			defer closer(t)

			ctx := context.Background()
			store := NewStateStore(db)

			err := store.SetTerm(ctx, tt.term)
			require.NoError(t, err)

			storedTerm, err := store.GetTerm(ctx)
			require.NoError(t, err)
			require.Equal(t, uint64(1), storedTerm)

		})
	}
}

func TestStore_SetCommit(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		commit uint64
	}{
		{
			name:   "set_commit",
			commit: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			db, closer := test.NewDB(t)
			defer closer(t)

			ctx := context.Background()
			store := NewStateStore(db)

			err := store.SetCommit(ctx, tt.commit)
			require.NoError(t, err)

			storedCommit, err := store.GetCommit(ctx)
			require.NoError(t, err)
			require.Equal(t, uint64(1), storedCommit)

		})
	}
}

func TestStore_SetVoted(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		voted uint64
	}{
		{
			name:  "set_voted",
			voted: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			db, closer := test.NewDB(t)
			defer closer(t)

			ctx := context.Background()
			store := NewStateStore(db)

			err := store.SetVoted(ctx, tt.voted)
			require.NoError(t, err)

			storedVoted, err := store.GetVoted(ctx)
			require.NoError(t, err)
			require.Equal(t, uint64(1), storedVoted)

		})
	}
}

func TestStore_SetApplied(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		applied uint64
	}{
		{
			name:    "set_applied",
			applied: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			db, closer := test.NewDB(t)
			defer closer(t)

			ctx := context.Background()
			store := NewStateStore(db)

			err := store.SetLastApplied(ctx, tt.applied)
			require.NoError(t, err)

			storedApplied, err := store.GetLastApplied(ctx)
			require.NoError(t, err)
			require.Equal(t, uint64(1), storedApplied)

		})
	}
}
