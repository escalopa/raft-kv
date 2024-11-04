package main

import (
	"context"

	"github.com/catalystgo/catalystgo"
	"github.com/catalystgo/logger/logger"
)

func main() {
	ctx := context.Background()

	app, err := catalystgo.New()
	if err != nil {
		logger.Fatalf(ctx, "init application: %v", err)
	}

	if err := app.Run(); err != nil {
		logger.Fatalf(ctx, "app run: %v", err)
	}
}
