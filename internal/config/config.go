package config

import (
	"context"
	"math/rand/v2"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/catalystgo/logger/logger"
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
	logLevel = envCfg{env: "LOG_LEVEL", def: "INFO"} // log level env key (default: "INFO")

	raftIDEnv      = "RAFT_ID"
	raftClusterEnv = "RAFT_CLUSTER"

	// general
	requestVoteTimeoutEnv   = envCfg{env: "RAFT_REQUEST_VOTE_TIMEOUT", def: "150"}
	appendEntriesTimeoutEnv = envCfg{env: "RAFT_APPEND_ENTRIES_TIMEOUT", def: "150"}

	raftCommitPeriodEnv = envCfg{env: "RAFT_COMMIT_PERIOD", def: "50"}

	// election
	raftStartDelayEnv      = envCfg{env: "RAFT_START_DELAY", def: "1500"}
	raftElectionTimeoutEnv = envCfg{env: "RAFT_ELECTION_TIMEOUT", def: "300"}

	// leader
	raftStalePeriod     = envCfg{env: "RAFT_STALE_PERIOD", def: "200"}
	raftHeartbeatPeriod = envCfg{env: "RAFT_HEARTBEAT_PERIOD", def: "50"}

	// db
	badgerEntryPath = envCfg{env: "BADGER_ENTRY_PATH", def: "/data/entry"}
	badgerStatePath = envCfg{env: "BADGER_STATE_PATH", def: "/data/state"}
	badgerKVPath    = envCfg{env: "BADGER_KV_PATH", def: "/data/kv"}
)

type (
	LeaderConfig struct {
		StalePeriod     int64
		HeartbeatPeriod int64
	}

	RaftConfig struct {
		ID      core.ServerID
		Cluster []core.Node

		CommitPeriod         int64
		AppendEntriesTimeout int64
		RequestVoteTimeout   int64

		StartDelay      int64
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

func (ac *AppConfig) GetRequestVoteTimeout() time.Duration {
	return time.Duration(ac.Raft.RequestVoteTimeout) * time.Millisecond
}

func (ac *AppConfig) GetAppendEntriesTimeout() time.Duration {
	return time.Duration(ac.Raft.AppendEntriesTimeout) * time.Millisecond
}

func (ac *AppConfig) GetStartDelay() time.Duration {
	return time.Duration(randIn2X(ac.Raft.StartDelay)) * time.Millisecond
}

func (ac *AppConfig) GetCommitPeriod() time.Duration {
	return time.Duration(randIn2X(ac.Raft.CommitPeriod)) * time.Millisecond
}

func (ac *AppConfig) GetElectionTimeout() time.Duration {
	return time.Duration(randIn2X(ac.Raft.ElectionTimeout)) * time.Millisecond
}

func (ac *AppConfig) GetLeaderStalePeriod() time.Duration {
	return time.Duration(randIn2X(ac.Raft.Leader.StalePeriod)) * time.Millisecond
}

func (ac *AppConfig) GetLeaderHeartbeatPeriod() time.Duration {
	return time.Duration(randIn2X(ac.Raft.Leader.HeartbeatPeriod)) * time.Millisecond
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
	commitPeriod, err := parseTimePeriod(raftCommitPeriodEnv)
	if err != nil {
		return nil, err
	}

	appendEntriesTimeout, err := parseTimePeriod(appendEntriesTimeoutEnv)
	if err != nil {
		return nil, err
	}

	requestVoteTimeout, err := parseTimePeriod(requestVoteTimeoutEnv)
	if err != nil {
		return nil, err
	}

	startDelay, err := parseTimePeriod(raftStartDelayEnv)
	if err != nil {
		return nil, err
	}

	electionTimeout, err := parseTimePeriod(raftElectionTimeoutEnv)
	if err != nil {
		return nil, err
	}

	heartbeatPeriod, err := parseTimePeriod(raftHeartbeatPeriod)
	if err != nil {
		return nil, err
	}

	stalePeriod, err := parseTimePeriod(raftStalePeriod)
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

			StartDelay:      startDelay,
			ElectionTimeout: electionTimeout,

			Leader: LeaderConfig{
				StalePeriod:     stalePeriod,
				HeartbeatPeriod: heartbeatPeriod,
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
	lvl, err := zapcore.ParseLevel(logLevel.value())
	if err == nil {
		return lvl
	}
	logger.WarnKV(context.Background(), "parse log_level", "error", err, "value", logLevel.value())
	return zapcore.InfoLevel // default
}
