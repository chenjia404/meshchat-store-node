package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"syscall"

	"store-node/internal/app"
	"store-node/internal/config"
	appLog "store-node/internal/log"
)

func main() {
	var configPath string
	flag.StringVar(&configPath, "config", "", "yaml config path")
	flag.Parse()

	logger := appLog.New()
	cfg, err := config.Load(configPath)
	if err != nil {
		logger.Error("load config failed", "error", err)
		os.Exit(1)
	}

	instance, err := app.New(cfg, logger)
	if err != nil {
		logger.Error("create app failed", "error", err)
		os.Exit(1)
	}
	defer instance.Close()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	instance.Start(ctx)
	host := instance.Host()
	for _, addr := range host.Addrs() {
		logger.Info("store node listening", "peer_id", host.ID().String(), "addr", addr.String()+"/p2p/"+host.ID().String())
	}

	<-ctx.Done()
	logger.Info("store node shutting down")
}
