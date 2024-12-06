package main

import (
	"context"
	"io"

	"github.com/catalystgo/catalystgo"
	"github.com/catalystgo/catalystgo/closer"
	"github.com/catalystgo/logger/logger"
	"github.com/escalopa/raft-kv/internal/api/kv"
	"github.com/escalopa/raft-kv/internal/api/raft"
	"github.com/escalopa/raft-kv/internal/config"
	"github.com/escalopa/raft-kv/internal/service"
	"github.com/escalopa/raft-kv/internal/storage"
	"google.golang.org/grpc/grpclog"
)

func main() {
	logger.SetLevel(config.LogLevel())
	grpclog.SetLoggerV2(grpclog.NewLoggerV2(io.Discard, io.Discard, io.Discard)) // TODO: remove

	app, err := catalystgo.New()
	if err != nil {
		logger.Fatalf(context.Background(), "init app: %v", err)
	}

	// Init App config
	appCfg, err := config.NewAppConfig(app.Ctx())
	if err != nil {
		logger.Fatalf(app.Ctx(), "init app config: %v", err)
	}

	// Init Entry store
	entryDB, err := storage.NewDB(appCfg.Badger.EntryPath)
	if err != nil {
		logger.Fatalf(app.Ctx(), "init entry db: %v", err)
	}

	closer.Add(entryDB.Close)
	entryStore := storage.NewEntryStore(entryDB)

	// Init State store
	stateDB, err := storage.NewDB(appCfg.Badger.StatePath)
	if err != nil {
		logger.Fatalf(app.Ctx(), "init state db: %v", err)
	}

	closer.Add(stateDB.Close)
	stateStore := storage.NewStateStore(stateDB)

	// Init KV store
	kvDB, err := storage.NewDB(appCfg.Badger.KVPath)
	if err != nil {
		logger.Fatalf(app.Ctx(), "init kv db: %v", err)
	}

	closer.Add(kvDB.Close)
	kvStore := storage.NewKVStore(kvDB)

	// Init Raft state
	raftState, err := service.NewRaftState(
		app.Ctx(),
		appCfg,
		appCfg.Raft.ID,
		appCfg.Raft.Cluster,
		entryStore,
		stateStore,
		kvStore,
	)
	if err != nil {
		logger.Fatalf(app.Ctx(), "init raft state: %v", err)
	}

	raftState.Run()
	closer.Add(raftState.Close)

	kvService := kv.NewKVService(raftState)
	raftService := raft.NewRaftService(raftState)

	services := []catalystgo.Service{
		kvService,
		raftService,
	}

	if err := app.Run(services...); err != nil {
		logger.Fatalf(app.Ctx(), "app run: %v", err)
	}
}
