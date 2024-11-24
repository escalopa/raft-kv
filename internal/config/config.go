package config

import (
	"context"
	"os"
	"strconv"
	"strings"

	"github.com/pkg/errors"
)

const (
	raftIDEnv      = "RAFT_ID"
	raftClusterEnv = "RAFT_CLUSTER"

	badgerEntryPathEnv = "BADGER_ENTRY_PATH"
	badgerStatePathEnv = "BADGER_STATE_PATH"
	badgerKVPathEnv    = "BADGER_KV_PATH"
)

type (
	// Node represents a single node in the Raft cluster
	// Passed as `ID@Address` in the RAFT_CLUSTER environment variable
	// where RAFT_CLUSTER is a comma-separated list of nodes
	Node struct {
		ID      uint64
		Address string // IP:Port
	}

	RaftConfig struct {
		ID      uint64
		Cluster []Node
	}

	BadgerConfig struct {
		EntryPath string // Path to the entry (log) storage
		StatePath string // Path to the raft state storage
		KVPath    string // Path to the state machine storage
	}

	AppConfig struct {
		// Raft configuration
		Raft RaftConfig
		// Badger configuration
		Badger BadgerConfig
	}
)

func NewAppConfig(ctx context.Context) (*AppConfig, error) {
	// Parse RaftID
	raftIDStr := os.Getenv(raftIDEnv)
	if raftIDStr == "" {
		return nil, errors.Errorf("missing %s", raftIDEnv)
	}

	raftID, err := strconv.ParseUint(raftIDStr, 10, 64)
	if err != nil {
		return nil, errors.Errorf("invalid %s: %v", raftIDEnv, err)
	}

	// Parse RaftCluster
	cluster := strings.Split(os.Getenv(raftClusterEnv), ",")
	if len(cluster) == 0 {
		return nil, errors.Errorf("missing %s", raftClusterEnv)
	}

	nodes, err := parseClusterConfig(ctx, raftID, cluster)
	if err != nil {
		return nil, errors.Errorf("parse cluster config: %v", err)
	}

	// Parse BadgerEntryPath
	entryPath := os.Getenv(badgerEntryPathEnv)
	if entryPath == "" {
		return nil, errors.Errorf("missing %s", badgerEntryPathEnv)
	}

	// Parse BadgerStatePath
	statePath := os.Getenv(badgerStatePathEnv)
	if statePath == "" {
		return nil, errors.Errorf("missing %s", badgerStatePathEnv)
	}

	// Parse BadgerKVPath
	kvPath := os.Getenv(badgerKVPathEnv)
	if kvPath == "" {
		return nil, errors.Errorf("missing %s", badgerKVPathEnv)
	}

	appCfg := &AppConfig{
		Raft: RaftConfig{
			ID:      raftID,
			Cluster: nodes,
		},
		Badger: BadgerConfig{
			EntryPath: entryPath,
			StatePath: statePath,
			KVPath:    kvPath,
		},
	}

	return appCfg, nil
}
