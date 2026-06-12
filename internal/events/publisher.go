package events

import (
	"context"
	"log/slog"

	"github.com/sanchit/order-orchestrator/internal/order"
)

type LogPublisher struct {
	logger *slog.Logger
}

func NewLogPublisher(logger *slog.Logger) *LogPublisher {
	return &LogPublisher{logger: logger}
}

func (p *LogPublisher) Publish(_ context.Context, event order.OrderStateChangedEvent) error {
	p.logger.Info("order state changed",
		"orderId", event.OrderID,
		"from", event.From,
		"to", event.To,
		"timestamp", event.Timestamp,
	)
	return nil
}
