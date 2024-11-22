package main

import (
	"flare-indexer/indexer/context"
	"flare-indexer/indexer/migrations"
	"flare-indexer/indexer/runner"
	"flare-indexer/indexer/shared"
	"flare-indexer/logger"
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	flags := context.ParseIndexerFlags()

	if flags.Version {
		fmt.Printf("Flare P-chain indexer version %s\n", shared.ApplicationVersion)
		return
	}

	ctx, err := context.BuildContext(flags)
	if err != nil {
		fmt.Printf("%v\n", err)
		return
	}

	logger.Info("Starting Flare indexer application version %s", shared.ApplicationVersion)

	err = migrations.Container.ExecuteAll(ctx.DB())
	if err != nil {
		fmt.Printf("%v\n", err)
		return
	}

	cancelChan := make(chan os.Signal, 1)
	signal.Notify(cancelChan, os.Interrupt, syscall.SIGTERM)

	// Prometheus metrics
	shared.InitMetricsServer(&ctx.Config().Metrics)

	runner.Start(ctx)

	<-cancelChan
	logger.Info("Stopped flare indexer")

}
