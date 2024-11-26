package storage

import (
	"context"
	"testing"

	"github.com/escalopa/raft-kv/internal/core"
	"github.com/escalopa/raft-kv/test"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
)

func TestStore_Append(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		entries []*core.Entry
		check   func(t *testing.T, ctx context.Context, store *EntryStore, entries []*core.Entry)
	}{
		{
			name: "append_single_entry",
			entries: []*core.Entry{
				{Term: 1, Index: 1, Data: []string{"SET", "key1", "value1"}},
			},
			check: func(t *testing.T, ctx context.Context, store *EntryStore, entries []*core.Entry) {
				for _, entry := range entries {
					e, err := store.At(ctx, entry.Index)
					require.NoError(t, err)
					require.True(t, entry.IsEqual(e))
				}
			},
		},
		{
			name: "append_multiple_entries",
			entries: []*core.Entry{
				{Term: 1, Index: 1, Data: []string{"SET", "key1", "value1"}},
				{Term: 1, Index: 2, Data: []string{"SET", "key2", "value2"}},
			},
			check: func(t *testing.T, ctx context.Context, store *EntryStore, entries []*core.Entry) {
				for _, entry := range entries {
					e, err := store.At(ctx, entry.Index)
					require.NoError(t, err)
					require.True(t, entry.IsEqual(e))
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			db, closer := test.NewDB(t)
			defer closer(t)

			ctx := context.Background()
			store := NewEntryStore(db)

			err := store.AppendEntries(ctx, tt.entries...)
			require.NoError(t, err)

			tt.check(t, ctx, store, tt.entries)
		})
	}
}

func TestStore_Last(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		entries []*core.Entry
		check   func(t *testing.T, ctx context.Context, store *EntryStore, entries []*core.Entry)
	}{
		{
			name: "last_entry_single",
			entries: []*core.Entry{
				{Term: 1, Index: 1, Data: []string{"SET", "key1", "value1"}},
			},
			check: func(t *testing.T, ctx context.Context, store *EntryStore, entries []*core.Entry) {
				last, err := store.Last(ctx)
				require.NoError(t, err)
				require.True(t, entries[len(entries)-1].IsEqual(last))
			},
		},
		{
			name: "last_entry_multiple",
			entries: []*core.Entry{
				{Term: 1, Index: 1, Data: []string{"SET", "key1", "value1"}},
				{Term: 1, Index: 2, Data: []string{"SET", "key2", "value2"}},
			},
			check: func(t *testing.T, ctx context.Context, store *EntryStore, entries []*core.Entry) {
				last, err := store.Last(ctx)
				require.NoError(t, err)
				require.True(t, entries[len(entries)-1].IsEqual(last))
			},
		},
		{
			name:    "last_entry_not_found",
			entries: []*core.Entry{},
			check: func(t *testing.T, ctx context.Context, store *EntryStore, _ []*core.Entry) {
				_, err := store.Last(ctx)
				require.Error(t, err)
				require.True(t, errors.Is(err, core.ErrNotFound))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			db, closer := test.NewDB(t)
			defer closer(t)

			ctx := context.Background()
			store := NewEntryStore(db)

			err := store.AppendEntries(ctx, tt.entries...)
			require.NoError(t, err)

			tt.check(t, ctx, store, tt.entries)
		})
	}
}

func TestStore_At(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		entry *core.Entry
		check func(t *testing.T, ctx context.Context, store *EntryStore, entry *core.Entry)
	}{
		{
			name:  "at_single_entry",
			entry: &core.Entry{Term: 1, Index: 1, Data: []string{"SET", "key1", "value1"}},
			check: func(t *testing.T, ctx context.Context, store *EntryStore, entry *core.Entry) {
				e, err := store.At(ctx, entry.Index)
				require.NoError(t, err)
				require.True(t, entry.IsEqual(e))
			},
		},
		{
			name:  "at_entry_not_found",
			entry: &core.Entry{Term: 1, Index: 1, Data: []string{"SET", "key1", "value1"}},
			check: func(t *testing.T, ctx context.Context, store *EntryStore, _ *core.Entry) {
				_, err := store.At(ctx, 999)
				require.Error(t, err)
				require.True(t, errors.Is(err, core.ErrNotFound))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			db, closer := test.NewDB(t)
			defer closer(t)

			ctx := context.Background()
			store := NewEntryStore(db)

			err := store.AppendEntries(ctx, tt.entry)
			require.NoError(t, err)

			tt.check(t, ctx, store, tt.entry)
		})
	}
}

func TestStore_Range(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		entries []*core.Entry
		start   uint64
		end     uint64
		check   func(t *testing.T, ctx context.Context, store *EntryStore, entries []*core.Entry)
	}{
		{
			name: "range_single_entry",
			entries: []*core.Entry{
				{Term: 1, Index: 1, Data: []string{"SET", "key1", "value1"}},
			},
			start: 1,
			end:   1,
			check: func(t *testing.T, ctx context.Context, store *EntryStore, entries []*core.Entry) {
				rangedEntries, err := store.Range(ctx, 1, 1)
				require.NoError(t, err)
				require.Equal(t, len(entries), len(rangedEntries))
				for i, entry := range entries {
					require.True(t, entry.IsEqual(rangedEntries[i]))
				}
			},
		},
		{
			name: "range_multiple_entries",
			entries: []*core.Entry{
				{Term: 1, Index: 1, Data: []string{"SET", "key1", "value1"}},
				{Term: 1, Index: 2, Data: []string{"DEL", "key2", " value2"}},
				{Term: 1, Index: 3, Data: []string{"SET", "key3", "value3"}},
			},
			start: 1,
			end:   3,
			check: func(t *testing.T, ctx context.Context, store *EntryStore, entries []*core.Entry) {
				rangedEntries, err := store.Range(ctx, 1, 3)
				require.NoError(t, err)
				require.Equal(t, len(entries), len(rangedEntries))
				for i, entry := range entries {
					require.True(t, entry.IsEqual(rangedEntries[i]))
				}
			},
		},
		{
			name: "range_entries_not_found",
			entries: []*core.Entry{
				{Term: 1, Index: 1, Data: []string{"SET", "key1", "value1"}},
			},
			start: 999,
			end:   1000,
			check: func(t *testing.T, ctx context.Context, store *EntryStore, _ []*core.Entry) {
				_, err := store.Range(ctx, 999, 1000)
				require.NoError(t, err)
			},
		},
		{
			name: "range_entries_partial_not_found",
			entries: []*core.Entry{
				{Term: 1, Index: 1, Data: []string{"SET", "key1", "value1"}},
			},
			start: 1,
			end:   1000,
			check: func(t *testing.T, ctx context.Context, store *EntryStore, entries []*core.Entry) {
				rangedEntries, err := store.Range(ctx, 1, 3)
				require.NoError(t, err)
				require.Equal(t, len(entries), len(rangedEntries))
				for i, entry := range entries {
					require.True(t, entry.IsEqual(rangedEntries[i]))
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			db, closer := test.NewDB(t)
			defer closer(t)

			ctx := context.Background()
			store := NewEntryStore(db)

			err := store.AppendEntries(ctx, tt.entries...)
			require.NoError(t, err)

			tt.check(t, ctx, store, tt.entries)
		})
	}
}
