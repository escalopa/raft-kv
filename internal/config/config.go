package config

import (
	"context"
	"math/rand/v2"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/escalopa/raft-kv/internal/core"
	"github.com/pkg/errors"
)

const (
	raftIDEnv      = "RAFT_ID"
	raftClusterEnv = "RAFT_CLUSTER"

	// general
	raftCommitPeriod = "RAFT_COMMIT_PERIOD"

	// follower
	raftElectionDelayPeriod   = "RAFT_ELECTION_DELAY_PERIOD"
	raftElectionTimeoutPeriod = "RAFT_ELECTION_TIMEOUT_PERIOD"

	// leader
	raftHeartbeatPeriod           = "RAFT_HEARTBEAT_PERIOD"
	raftLeaderStalePeriod         = "RAFT_LEADER_STALE_PERIOD"
	raftLeaderCheckStepDownPeriod = "RAFT_LEADER_CHECK_STEP_DOWN_PERIOD"

	// db
	badgerEntryPathEnv = "BADGER_ENTRY_PATH"
	badgerStatePathEnv = "BADGER_STATE_PATH"
	badgerKVPathEnv    = "BADGER_KV_PATH"
)

type (
	LeaderConfig struct {
		StalePeriod         int64
		HeartbeatPeriod     int64
		CheckStepDownPeriod int64
	}

	RaftConfig struct {
		ID      core.ServerID
		Cluster []core.Node

		CommitPeriod int64

		ElectionDelay   int64
		ElectionTimeout int64

		Leader LeaderConfig
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
	return time.Duration(randIn2X(ac.Raft.CommitPeriod)) * time.Millisecond
}

func (ac *AppConfig) GetElectionDelayPeriod() time.Duration {
	return time.Duration(randIn2X(ac.Raft.ElectionDelay)) * time.Millisecond
}

func (ac *AppConfig) GetElectionTimeoutPeriod() time.Duration {
	return time.Duration(randIn2X(ac.Raft.ElectionTimeout)) * time.Millisecond
}

func (ac *AppConfig) GetHeartbeatPeriod() time.Duration {
	return time.Duration(randIn2X(ac.Raft.Leader.HeartbeatPeriod)) * time.Millisecond
}

func (ac *AppConfig) GetLeaderStalePeriod() time.Duration {
	return time.Duration(randIn2X(ac.Raft.Leader.StalePeriod)) * time.Millisecond
}

func (ac *AppConfig) GetLeaderCheckStepDownPeriod() time.Duration {
	return time.Duration(randIn2X(ac.Raft.Leader.CheckStepDownPeriod)) * time.Millisecond
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

	// Parse Time Periods
	commitPeriod, err := parseTimePeriod(raftCommitPeriod)
	if err != nil {
		return nil, err
	}

	electionDelay, err := parseTimePeriod(raftElectionDelayPeriod)
	if err != nil {
		return nil, err
	}

	electionTimeout, err := parseTimePeriod(raftElectionTimeoutPeriod)
	if err != nil {
		return nil, err
	}

	heartbeatPeriod, err := parseTimePeriod(raftHeartbeatPeriod)
	if err != nil {
		return nil, err
	}

	leaderStalePeriod, err := parseTimePeriod(raftLeaderStalePeriod)
	if err != nil {
		return nil, err
	}

	checkStepDownPeriod, err := parseTimePeriod(raftLeaderCheckStepDownPeriod)
	if err != nil {
		return nil, err
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
			ID:      core.ServerID(raftID),
			Cluster: nodes,

			CommitPeriod: commitPeriod,

			ElectionDelay:   electionDelay,
			ElectionTimeout: electionTimeout,

			Leader: LeaderConfig{
				StalePeriod:         leaderStalePeriod,
				HeartbeatPeriod:     heartbeatPeriod,
				CheckStepDownPeriod: checkStepDownPeriod,
			},
		},
		Badger: BadgerConfig{
			EntryPath: entryPath,
			StatePath: statePath,
			KVPath:    kvPath,
		},
	}

	return appCfg, nil
}

func parseTimePeriod(env string) (int64, error) {
	timeMS, err := strconv.ParseInt(os.Getenv(env), 10, 64)
	if err != nil {
		return 0, errors.Errorf("parse %s: %v", env, err)
	}
	return timeMS, nil
}

// randIn2X returns a random number in the range [min, 2*min)
func randIn2X(min int64) int64 {
	return min + rand.Int64N(min)
}
