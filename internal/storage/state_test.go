package storage

import (
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

			store := NewStateStore(db)

			err := store.SetTerm(tt.term)
			require.NoError(t, err)

			storedTerm, err := store.GetTerm()
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

			store := NewStateStore(db)

			err := store.SetCommit(tt.commit)
			require.NoError(t, err)

			storedCommit, err := store.GetCommit()
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

			store := NewStateStore(db)

			err := store.SetVoted(tt.voted)
			require.NoError(t, err)

			storedVoted, err := store.GetVoted()
			require.NoError(t, err)
			require.Equal(t, uint64(1), storedVoted)

		})
	}
}
