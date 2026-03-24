package gc

import (
	"context"
	"log/slog"
	"time"

	"store-node/internal/metrics"
	"store-node/internal/storage"
)

type Worker struct {
	storage   storage.Storage
	metrics   *metrics.Metrics
	logger    *slog.Logger
	interval  time.Duration
	batchSize int
}

func NewWorker(st storage.Storage, metrics *metrics.Metrics, logger *slog.Logger, interval time.Duration, batchSize int) *Worker {
	return &Worker{
		storage:   st,
		metrics:   metrics,
		logger:    logger,
		interval:  interval,
		batchSize: batchSize,
	}
}

func (w *Worker) Run(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.runOnce(ctx)
		}
	}
}

func (w *Worker) runOnce(ctx context.Context) {
	start := time.Now()
	var totalDeleted int
	var scans int

	for {
		scans++
		deleted, err := w.storage.DeleteExpired(ctx, time.Now().UTC().Unix(), w.batchSize)
		if err != nil {
			w.logger.Error("gc failed", "error", err)
			return
		}
		totalDeleted += deleted
		if deleted == 0 || deleted < w.batchSize {
			break
		}
	}

	if totalDeleted > 0 {
		w.metrics.GCDeletedTotal.Add(uint64(totalDeleted))
	}
	w.logger.Info("gc completed", "scans", scans, "deleted", totalDeleted, "elapsed_ms", time.Since(start).Milliseconds())
}
