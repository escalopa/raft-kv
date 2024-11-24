package main

import (
	"context"

	"github.com/catalystgo/catalystgo"
	"github.com/catalystgo/logger/logger"
	"github.com/escalopa/raft-kv/internal/config"
	"github.com/escalopa/raft-kv/internal/storage"
)

func main() {
	ctx := context.Background()

	app, err := catalystgo.New()
	if err != nil {
		logger.Fatalf(ctx, "init app: %v", err)
	}

	// Init App config
	appCfg, err := config.NewAppConfig(ctx)
	if err != nil {
		logger.Fatalf(ctx, "init app config: %v", err)
	}

	// Init Entry store
	localEntryDB, err := storage.NewDB(appCfg.Badger.EntryPath)
	if err != nil {
		logger.Fatalf(ctx, "init entry db: %v", err)
	}
	entryStore := storage.NewEntryStore(localEntryDB)

	// Init State store
	localStateDB, err := storage.NewDB(appCfg.Badger.EntryPath)
	if err != nil {
		logger.Fatalf(ctx, "init state db: %v", err)
	}
	stateStore := storage.NewEntryStore(localStateDB)

	// Init KV store
	localKVDB, err := storage.NewDB(appCfg.Badger.KVPath)
	if err != nil {
		logger.Fatalf(ctx, "init kv db: %v", err)
	}
	kvStore := storage.NewKVStore(localKVDB)

	_, _, _ = entryStore, stateStore, kvStore // TODO: use them in raft server

	if err := app.Run(); err != nil {
		logger.Fatalf(ctx, "app run: %v", err)
	}
}
