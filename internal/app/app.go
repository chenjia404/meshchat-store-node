package app

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	corehost "github.com/libp2p/go-libp2p/core/host"

	"store-node/internal/auth"
	"store-node/internal/config"
	gcworker "store-node/internal/gc"
	"store-node/internal/metrics"
	"store-node/internal/p2p"
	"store-node/internal/ratelimit"
	"store-node/internal/service"
	pebblestore "store-node/internal/storage/pebble"
)

type App struct {
	cfg     config.Config
	logger  *slog.Logger
	metrics *metrics.Metrics
	host    corehost.Host
	store   *pebblestore.Store
	server  *p2p.Server
	cancel  context.CancelFunc
}

func New(cfg config.Config, logger *slog.Logger) (*App, error) {
	if err := os.MkdirAll(cfg.Store.DataDir, 0o755); err != nil {
		return nil, err
	}
	store, err := pebblestore.Open(filepath.Clean(cfg.Store.DataDir))
	if err != nil {
		return nil, fmt.Errorf("open pebble: %w", err)
	}

	identityKeyPath := cfg.Node.IdentityKeyPath
	if identityKeyPath == "" {
		identityKeyPath = filepath.Join(cfg.Store.DataDir, "node_identity.key")
	}
	identityKey, err := p2p.LoadOrCreateIdentityKey(identityKeyPath)
	if err != nil {
		store.Close()
		return nil, fmt.Errorf("load node identity: %w", err)
	}

	host, err := p2p.NewHost(cfg.Node.ListenAddrs, cfg.Node.AnnounceAddrs, identityKey)
	if err != nil {
		store.Close()
		return nil, fmt.Errorf("create host: %w", err)
	}

	metricsObj := metrics.New()
	verifier := auth.NewVerifier(cfg.Store.DefaultTTLSec)
	remoteLimiter := ratelimit.New(cfg.RateLimit.PerSenderPerMinute)
	senderLimiter := ratelimit.New(cfg.RateLimit.PerSenderPerMinute)

	storeSvc := service.NewStoreService(
		store,
		verifier,
		remoteLimiter,
		senderLimiter,
		metricsObj,
		logger,
		cfg.Store.DefaultTTLSec,
		cfg.Store.MaxTTLSec,
		cfg.Store.MaxMessageSize,
		cfg.Store.MaxMessagesPerRecipient,
		cfg.Store.MaxBytesPerRecipient,
	)
	fetchSvc := service.NewFetchService(store, metricsObj, logger, cfg.Store.FetchLimitMax)
	ackSvc := service.NewAckService(store, verifier, metricsObj, logger)

	server := p2p.NewServer(host, logger, frameLimits(cfg), protocolTimeouts(), storeSvc, fetchSvc, ackSvc)
	server.Register()

	return &App{
		cfg:     cfg,
		logger:  logger,
		metrics: metricsObj,
		host:    host,
		store:   store,
		server:  server,
	}, nil
}

func (a *App) Start(ctx context.Context) {
	runCtx, cancel := context.WithCancel(ctx)
	a.cancel = cancel
	worker := gcworker.NewWorker(a.store, a.metrics, a.logger, time.Duration(a.cfg.GC.IntervalSec)*time.Second, a.cfg.GC.BatchSize)
	go worker.Run(runCtx)
}

func (a *App) Close() error {
	if a.cancel != nil {
		a.cancel()
	}
	if a.host != nil {
		if err := a.host.Close(); err != nil {
			return err
		}
	}
	if a.store != nil {
		return a.store.Close()
	}
	return nil
}

func (a *App) Host() corehost.Host {
	return a.host
}

func frameLimits(cfg config.Config) p2p.FrameLimits {
	const smallFrame = 16 * 1024
	const messageOverhead = 64 * 1024
	const rpcEnvelopeOverhead = 4096

	storeRequest := cfg.Store.MaxMessageSize + messageOverhead
	fetchResponse := cfg.Store.MaxMessageSize*cfg.Store.FetchLimitMax + messageOverhead
	if fetchResponse < smallFrame {
		fetchResponse = smallFrame
	}
	rpcReq := storeRequest + rpcEnvelopeOverhead
	if rpcReq < smallFrame+rpcEnvelopeOverhead {
		rpcReq = smallFrame + rpcEnvelopeOverhead
	}
	rpcResp := fetchResponse + rpcEnvelopeOverhead
	return p2p.FrameLimits{
		RPCRequest:  uint32(rpcReq),
		RPCResponse: uint32(rpcResp),
	}
}

func protocolTimeouts() p2p.Timeouts {
	return p2p.Timeouts{
		Read:    15 * time.Second,
		Write:   15 * time.Second,
		Handler: 30 * time.Second,
	}
}
