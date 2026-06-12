package timeouts

import (
	"context"
	"log/slog"
	"time"

	"github.com/sanchit/order-orchestrator/internal/order"
)

func RunWorker(ctx context.Context, svc *order.Service, repo order.Repository, logger *slog.Logger, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			processDue(ctx, svc, repo, logger)
		}
	}
}

func processDue(ctx context.Context, svc *order.Service, repo order.Repository, logger *slog.Logger) {
	due, err := repo.FindDueUnprocessedTimeouts(ctx, time.Now().UTC())
	if err != nil {
		logger.Warn("timeout query failed", "error", err)
		return
	}

	for _, timeout := range due {
		if _, err := svc.TimeoutTransition(ctx, timeout.OrderID, timeout.ExpectedState); err != nil {
			logger.Warn("timeout transition failed", "timeoutId", timeout.ID, "orderId", timeout.OrderID, "error", err)
			continue
		}
		if err := repo.MarkTimeoutProcessed(ctx, timeout.ID); err != nil {
			logger.Warn("timeout mark processed failed", "timeoutId", timeout.ID, "error", err)
		}
	}
}
