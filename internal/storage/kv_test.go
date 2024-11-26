package storage

import (
	"context"
	"testing"

	"github.com/escalopa/raft-kv/internal/core"
	"github.com/escalopa/raft-kv/test"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
)

func TestDB_Get(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		key   string
		value string
		check func(t *testing.T, ctx context.Context, store *KVStore)
	}{
		{
			name:  "get_existing_key",
			key:   "key1",
			value: "value1",
			check: func(t *testing.T, ctx context.Context, store *KVStore) {
				val, err := store.Get(ctx, "key1")
				require.NoError(t, err)
				require.Equal(t, "value1", val)
			},
		},
		{
			name:  "get_non_existing_key",
			key:   "key2",
			value: "",
			check: func(t *testing.T, ctx context.Context, store *KVStore) {
				_, err := store.Get(ctx, "key2")
				require.Error(t, err)
				require.True(t, errors.Is(err, core.ErrNotFound))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			badgerDB, closer := test.NewDB(t)
			defer closer(t)

			ctx := context.Background()
			store := NewKVStore(badgerDB)

			if isSetRequired(tt.value) {
				err := store.Set(ctx, tt.key, tt.value)
				require.NoError(t, err)
			}

			tt.check(t, ctx, store)
		})
	}
}

func TestDB_Set(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		key   string
		value string
		check func(t *testing.T, ctx context.Context, store *KVStore)
	}{
		{
			name:  "set_key_value",
			key:   "key1",
			value: "value1",
			check: func(t *testing.T, ctx context.Context, store *KVStore) {
				val, err := store.Get(ctx, "key1")
				require.NoError(t, err)
				require.Equal(t, "value1", val)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			badgerDB, closer := test.NewDB(t)
			defer closer(t)

			ctx := context.Background()
			store := NewKVStore(badgerDB)

			err := store.Set(ctx, tt.key, tt.value)
			require.NoError(t, err)

			tt.check(t, ctx, store)
		})
	}
}

func TestDB_Del(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		key   string
		value string
		check func(t *testing.T, ctx context.Context, store *KVStore)
	}{
		{
			name:  "delete_existing_key",
			key:   "key1",
			value: "value1",
			check: func(t *testing.T, ctx context.Context, store *KVStore) {
				err := store.Del(ctx, "key1")
				require.NoError(t, err)

				_, err = store.Get(ctx, "key1")
				require.Error(t, err)
				require.True(t, errors.Is(err, core.ErrNotFound))
			},
		},
		{
			name:  "delete_non_existing_key",
			key:   "key2",
			value: "",
			check: func(t *testing.T, ctx context.Context, store *KVStore) {
				err := store.Del(ctx, "key2")
				require.NoError(t, err)

				_, err = store.Get(ctx, "key2")
				require.Error(t, err)
				require.True(t, errors.Is(err, core.ErrNotFound))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			badgerDB, closer := test.NewDB(t)
			defer closer(t)

			ctx := context.Background()
			store := NewKVStore(badgerDB)

			if isSetRequired(tt.value) {
				err := store.Set(ctx, tt.key, tt.value)
				require.NoError(t, err)
			}

			tt.check(t, ctx, store)
		})
	}
}

func isSetRequired(value string) bool {
	return value != ""
}
