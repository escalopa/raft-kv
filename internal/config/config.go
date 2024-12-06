package config

import (
	"context"
	"github.com/catalystgo/logger/logger"
	"math/rand/v2"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/escalopa/raft-kv/internal/core"
	"github.com/pkg/errors"
	"go.uber.org/zap/zapcore"
)

type envCfg struct {
	env string // env key
	def string // default value
}

func (e *envCfg) value() string {
	value := os.Getenv(e.env)
	if value == "" {
		return e.def
	}
	return value
}

var (
	logLevelEnv = "LOG_LEVEL" // log level env key (default: "WARN")

	raftIDEnv      = "RAFT_ID"
	raftClusterEnv = "RAFT_CLUSTER"

	// general
	raftCommitPeriod           = envCfg{env: "RAFT_COMMIT_PERIOD", def: "50"}
	appendEntriesTimeoutPeriod = envCfg{env: "RAFT_APPEND_ENTRIES_TIMEOUT", def: "150"}
	requestVoteTimeoutPeriod   = envCfg{env: "RAFT_REQUEST_VOTE_TIMEOUT", def: "150"}

	// follower
	raftElectionDelayPeriod   = envCfg{env: "RAFT_ELECTION_DELAY_PERIOD", def: "3000"}
	raftElectionTimeoutPeriod = envCfg{env: "RAFT_ELECTION_TIMEOUT_PERIOD", def: "300"}

	// leader
	raftHeartbeatPeriod        = envCfg{env: "RAFT_HEARTBEAT_PERIOD", def: "50"}
	raftLeaderStalePeriod      = envCfg{env: "RAFT_LEADER_STALE_PERIOD", def: "200"}
	raftLeaderStaleCheckPeriod = envCfg{env: "RAFT_LEADER_STALE_CHECK_PERIOD", def: "100"}

	// db
	badgerEntryPath = envCfg{env: "BADGER_ENTRY_PATH", def: "/data/entry"}
	badgerStatePath = envCfg{env: "BADGER_STATE_PATH", def: "/data/state"}
	badgerKVPath    = envCfg{env: "BADGER_KV_PATH", def: "/data/kv"}
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

		AppendEntriesTimeout int64
		RequestVoteTimeout   int64

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

func (ac *AppConfig) GetAppendEntriesTimeout() time.Duration {
	return time.Duration(ac.Raft.AppendEntriesTimeout) * time.Millisecond
}

func (ac *AppConfig) GetRequestVoteTimeout() time.Duration {
	return time.Duration(ac.Raft.RequestVoteTimeout) * time.Millisecond
}

func (ac *AppConfig) GetElectionDelay() time.Duration {
	return time.Duration(randIn2X(ac.Raft.ElectionDelay)) * time.Millisecond
}

func (ac *AppConfig) GetElectionTimeout() time.Duration {
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

	appendEntriesTimeout, err := parseTimePeriod(appendEntriesTimeoutPeriod)
	if err != nil {
		return nil, err
	}

	requestVoteTimeout, err := parseTimePeriod(requestVoteTimeoutPeriod)
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

	checkStepDownPeriod, err := parseTimePeriod(raftLeaderStaleCheckPeriod)
	if err != nil {
		return nil, err
	}

	// Parse Badger DB paths
	entryPath := badgerEntryPath.value()
	statePath := badgerStatePath.value()
	kvPath := badgerKVPath.value()

	appCfg := &AppConfig{
		Raft: RaftConfig{
			ID:      core.ServerID(raftID),
			Cluster: nodes,

			CommitPeriod:         commitPeriod,
			AppendEntriesTimeout: appendEntriesTimeout,
			RequestVoteTimeout:   requestVoteTimeout,

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

	logger.InfoKV(ctx, "config dump", "config", appCfg)

	return appCfg, nil
}

func parseTimePeriod(cfg envCfg) (int64, error) {
	timeMS, err := strconv.ParseInt(cfg.value(), 10, 64)
	if err != nil {
		return 0, errors.Errorf("parse %s: %v", cfg.env, err)
	}
	return timeMS, nil
}

// randIn2X returns a random number in the range [min, 2*min)
func randIn2X(min int64) int64 {
	return min + rand.Int64N(min)
}

func LogLevel() zapcore.Level {
	lvl, err := zapcore.ParseLevel(os.Getenv(logLevelEnv))
	if err == nil {
		return lvl
	}
	return zapcore.WarnLevel // default
}
