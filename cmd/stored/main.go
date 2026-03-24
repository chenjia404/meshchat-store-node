package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"syscall"
	"time"

	"store-node/internal/app"
	"store-node/internal/config"
	appLog "store-node/internal/log"
	"store-node/internal/netutil"
)

func main() {
	var configPath string
	var port int
	var announceIP string
	flag.StringVar(&configPath, "config", "", "yaml config path")
	flag.IntVar(&port, "port", 0, "override default listen port for tcp and quic")
	flag.StringVar(&announceIP, "announce-ip", "", "override the IP advertised by libp2p")
	flag.Parse()

	logger := appLog.New()
	cfg, err := config.Load(configPath)
	if err != nil {
		logger.Error("load config failed", "error", err)
		os.Exit(1)
	}
	if port > 0 {
		cfg.Node.ListenAddrs = config.DefaultListenAddrs(port)
	}
	if announceIP != "" {
		cfg.Node.AnnounceAddrs, err = config.BuildAnnounceAddrs(cfg.Node.ListenAddrs, announceIP)
		if err != nil {
			logger.Error("build announce addrs failed", "error", err)
			os.Exit(1)
		}
	} else if len(cfg.Node.AnnounceAddrs) == 0 {
		resolveCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		detectedIP, detectErr := netutil.DetectPublicIP(resolveCtx, nil, nil)
		cancel()
		if detectErr != nil {
			logger.Warn("detect public ip failed, continue without announce addrs", "error", detectErr)
		} else {
			cfg.Node.AnnounceAddrs, err = config.BuildAnnounceAddrs(cfg.Node.ListenAddrs, detectedIP)
			if err != nil {
				logger.Error("build detected announce addrs failed", "public_ip", detectedIP, "error", err)
				os.Exit(1)
			}
			logger.Info("detected public ip for announce addrs", "public_ip", detectedIP)
		}
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
