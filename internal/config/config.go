package config

import (
	"context"
	"encoding/json"
	"math/rand/v2"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/catalystgo/logger/logger"
	"github.com/escalopa/raft-kv/internal/core"
	"github.com/pkg/errors"
)

const (
	raftIDEnv      = "RAFT_ID"
	raftClusterEnv = "RAFT_CLUSTER"

	raftCommitPeriod    = "RAFT_COMMIT_PERIOD"
	raftHeartbeatPeriod = "RAFT_HEARTBEAT_PERIOD"

	raftElectionDelay   = "RAFT_ELECTION_DELAY"
	raftElectionTimeout = "RAFT_ELECTION_TIMEOUT"

	badgerEntryPathEnv = "BADGER_ENTRY_PATH"
	badgerStatePathEnv = "BADGER_STATE_PATH"
	badgerKVPathEnv    = "BADGER_KV_PATH"
)

var (
	defaultElectionDelay   = TimeRange{Min: 500, Max: 1000}
	defaultElectionTimeout = TimeRange{Min: 150, Max: 300}
	defaultHeartbeatPeriod = TimeRange{Min: 50, Max: 100}
)

type (
	TimeRange struct {
		Min int `json:"min"`
		Max int `json:"max"`
	}

	RaftConfig struct {
		ID      core.ServerID
		Cluster []core.Node

		CommitPeriod    TimeRange
		ElectionDelay   TimeRange
		ElectionTimeout TimeRange
		HeartbeatPeriod TimeRange
	}

	BadgerConfig struct {
		EntryPath string // Path to the entry (log) storage
		StatePath string // Path to the raft state storage
		KVPath    string // Path to the state machine storage
	}

	AppConfig struct {
		Raft   RaftConfig
		Badger BadgerConfig
	}
)

func (ac *AppConfig) GetCommitPeriod() time.Duration {
	return time.Duration(randInRange(ac.Raft.CommitPeriod.Min, ac.Raft.CommitPeriod.Max)) * time.Millisecond
}

func (ac *AppConfig) GetElectionDelay() time.Duration {
	return time.Duration(randInRange(ac.Raft.ElectionDelay.Min, ac.Raft.ElectionDelay.Max)) * time.Millisecond
}

func (ac *AppConfig) GetElectionTimeout() time.Duration {
	return time.Duration(randInRange(ac.Raft.ElectionTimeout.Min, ac.Raft.ElectionTimeout.Max)) * time.Millisecond
}

func (ac *AppConfig) GetHeartbeatPeriod() time.Duration {
	return time.Duration(randInRange(ac.Raft.HeartbeatPeriod.Min, ac.Raft.HeartbeatPeriod.Max)) * time.Millisecond
}

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

	// Parse CommitPeriod, ElectionDelay, ElectionTimeout, HeartbeatPeriod
	commitPeriod := parseTimeRange(ctx, raftCommitPeriod, TimeRange{Min: 50, Max: 100})
	electionDelay := parseTimeRange(ctx, raftElectionDelay, defaultElectionDelay)
	electionTimeout := parseTimeRange(ctx, raftElectionTimeout, defaultElectionTimeout)
	heartbeatPeriod := parseTimeRange(ctx, raftHeartbeatPeriod, defaultHeartbeatPeriod)

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
			ID:      core.ServerID(raftID),
			Cluster: nodes,

			CommitPeriod:    commitPeriod,
			ElectionDelay:   electionDelay,
			ElectionTimeout: electionTimeout,
			HeartbeatPeriod: heartbeatPeriod,
		},
		Badger: BadgerConfig{
			EntryPath: entryPath,
			StatePath: statePath,
			KVPath:    kvPath,
		},
	}

	return appCfg, nil
}

func parseTimeRange(ctx context.Context, envKey string, defaultValue TimeRange) TimeRange {
	envValue := os.Getenv(envKey)
	if envValue == "" {
		return defaultValue
	}

	var tr TimeRange
	err := json.Unmarshal([]byte(envValue), &tr)
	if err != nil {
		logger.ErrorKV(ctx, "parse time range", "error", err, "env_key", envKey, "env_value", envValue)
		return defaultValue
	}

	return tr
}

func randInRange(min, max int) int {
	return min + rand.IntN(max-min)
}
