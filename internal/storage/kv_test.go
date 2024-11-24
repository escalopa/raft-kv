package storage

import (
	"testing"

	"github.com/pkg/errors"

	"github.com/escalopa/raft-kv/internal/core"
	"github.com/escalopa/raft-kv/test"
	"github.com/stretchr/testify/require"
)

func TestDB_Get(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		key   string
		value string
		check func(t *testing.T, db *KVStore)
	}{
		{
			name:  "get_existing_key",
			key:   "key1",
			value: "value1",
			check: func(t *testing.T, db *KVStore) {
				val, err := db.Get("key1")
				require.NoError(t, err)
				require.Equal(t, "value1", val)
			},
		},
		{
			name:  "get_non_existing_key",
			key:   "key2",
			value: "",
			check: func(t *testing.T, db *KVStore) {
				_, err := db.Get("key2")
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

			db := NewKVStore(badgerDB)

			if isSetRequired(tt.value) {
				err := db.Set(tt.key, tt.value)
				require.NoError(t, err)
			}

			tt.check(t, db)
		})
	}
}

func TestDB_Set(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		key   string
		value string
		check func(t *testing.T, db *KVStore)
	}{
		{
			name:  "set_key_value",
			key:   "key1",
			value: "value1",
			check: func(t *testing.T, db *KVStore) {
				val, err := db.Get("key1")
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

			db := NewKVStore(badgerDB)

			err := db.Set(tt.key, tt.value)
			require.NoError(t, err)

			tt.check(t, db)
		})
	}
}

func TestDB_Del(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		key   string
		value string
		check func(t *testing.T, db *KVStore)
	}{
		{
			name:  "delete_existing_key",
			key:   "key1",
			value: "value1",
			check: func(t *testing.T, db *KVStore) {
				err := db.Del("key1")
				require.NoError(t, err)

				_, err = db.Get("key1")
				require.Error(t, err)
				require.True(t, errors.Is(err, core.ErrNotFound))
			},
		},
		{
			name:  "delete_non_existing_key",
			key:   "key2",
			value: "",
			check: func(t *testing.T, db *KVStore) {
				err := db.Del("key2")
				require.NoError(t, err)

				_, err = db.Get("key2")
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

			db := NewKVStore(badgerDB)

			if isSetRequired(tt.value) {
				err := db.Set(tt.key, tt.value)
				require.NoError(t, err)
			}

			tt.check(t, db)
		})
	}
}

func isSetRequired(value string) bool {
	return value != ""
}
