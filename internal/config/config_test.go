package config

import (
	"context"
	"os"
	"testing"

	"github.com/escalopa/raft-kv/internal/core"

	"github.com/stretchr/testify/require"
)

func TestNewAppConfig(t *testing.T) {
	t.Skip() // TODO: fix test

	t.Parallel()

	tests := []struct {
		name    string
		envVars map[string]string
		check   func(t *testing.T, cfg *AppConfig, err error)
	}{
		{
			name: "valid_config",
			envVars: map[string]string{
				"RAFT_ID":           "1",
				"RAFT_CLUSTER":      "2@127.0.0.1:8080,3@127.0.0.1:8081",
				"BADGER_ENTRY_PATH": "/tmp/badger/entry",
				"BADGER_STATE_PATH": "/tmp/badger/state",
				"BADGER_KV_PATH":    "/tmp/badger/kv",
			},
			check: func(t *testing.T, cfg *AppConfig, err error) {
				require.NoError(t, err)
				require.Equal(t, core.ServerID(1), cfg.Raft.ID)
				require.Equal(t, 2, len(cfg.Raft.Cluster))
				require.Equal(t, "/tmp/badger/entry", cfg.Badger.EntryPath)
				require.Equal(t, "/tmp/badger/state", cfg.Badger.StatePath)
				require.Equal(t, "/tmp/badger/kv", cfg.Badger.KVPath)
			},
		},
		{
			name: "missing_raft_id",
			envVars: map[string]string{
				"RAFT_CLUSTER":      "1@127.0.0.1:8080,2@127.0.0.1:8081",
				"BADGER_ENTRY_PATH": "/tmp/badger/entry",
				"BADGER_STATE_PATH": "/tmp/badger/state",
				"BADGER_KV_PATH":    "/tmp/badger/kv",
			},
			check: func(t *testing.T, _ *AppConfig, err error) {
				require.Error(t, err)
			},
		},
		{
			name: "invalid_raft_id",
			envVars: map[string]string{
				"RAFT_ID":           "invalid",
				"RAFT_CLUSTER":      "1@127.0.0.1:8080,2@127.0.0.1:8081",
				"BADGER_STATE_PATH": "/tmp/badger/state",
				"BADGER_KV_PATH":    "/tmp/badger/kv",
			},
			check: func(t *testing.T, _ *AppConfig, err error) {
				require.Error(t, err)
			},
		},
		{
			name: "missing_raft_cluster",
			envVars: map[string]string{
				"RAFT_ID":           "1",
				"BADGER_STATE_PATH": "/tmp/badger/state",
				"BADGER_KV_PATH":    "/tmp/badger/kv",
			},
			check: func(t *testing.T, _ *AppConfig, err error) {
				require.Error(t, err)
			},
		},
		{
			name: "missing_badger_entry_path",
			envVars: map[string]string{
				"RAFT_ID":             "1",
				"RAFT_CLUSTER":        "1@127.0.0.1:8080,2@127.0.0.1:8081",
				"BADGER_STORAGE_PATH": "/tmp/badger/storage",
			},
			check: func(t *testing.T, _ *AppConfig, err error) {
				require.Error(t, err)
			},
		},
		{
			name: "missing_badger_storage_path",
			envVars: map[string]string{
				"RAFT_ID":           "1",
				"RAFT_CLUSTER":      "1@127.0.0.1:8080,2@127.0.0.1:8081",
				"BADGER_ENTRY_PATH": "/tmp/badger/entry",
			},
			check: func(t *testing.T, _ *AppConfig, err error) {
				require.Error(t, err)
			},
		},
		{
			name: "invalid_node_address",
			envVars: map[string]string{
				"RAFT_ID":           "1",
				"RAFT_CLUSTER":      "1127.0.0.1,2@127.0.0.1:8081",
				"BADGER_STATE_PATH": "/tmp/badger/state",
				"BADGER_KV_PATH":    "/tmp/badger/kv",
			},
			check: func(t *testing.T, _ *AppConfig, err error) {
				require.Error(t, err)
			},
		},
		{
			name: "duplicate_node_id",
			envVars: map[string]string{
				"RAFT_ID":           "1",
				"RAFT_CLUSTER":      "1@127.0.0.1:8080,1@127.0.0.1:8081",
				"BADGER_STATE_PATH": "/tmp/badger/state",
				"BADGER_KV_PATH":    "/tmp/badger/kv",
			},
			check: func(t *testing.T, _ *AppConfig, err error) {
				require.Error(t, err)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			for k, v := range tt.envVars {
				require.NoError(t, os.Setenv(k, v))
			}

			cfg, err := NewAppConfig(context.Background())
			tt.check(t, cfg, err)

			for k := range tt.envVars {
				require.NoError(t, os.Unsetenv(k))
			}
		})
	}
}
