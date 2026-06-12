package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sanchit/order-orchestrator/internal/events"
	"github.com/sanchit/order-orchestrator/internal/idempotency"
	"github.com/sanchit/order-orchestrator/internal/order"
	"github.com/sanchit/order-orchestrator/internal/timeouts"
	"github.com/sanchit/order-orchestrator/internal/web"
	"github.com/sanchit/order-orchestrator/internal/ws"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	repo := order.NewMemoryRepository()
	publisher := events.NewLogPublisher(logger)
	idem := idempotency.NewMemoryStore()
	hub := ws.NewHub()
	service := order.NewService(repo, publisher, idem, hub, logger)
	handler := order.NewHandler(service, hub)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	web.RegisterRoutes(mux)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go timeouts.RunWorker(ctx, service, repo, logger, time.Minute)

	server := &http.Server{
		Addr:              env("ADDR", ":8080"),
		Handler:           requestLogger(logger, mux),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		logger.Info("server starting", "addr", server.Addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server failed", "error", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("server shutdown failed", "error", err)
	}
}

func env(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

func requestLogger(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		next.ServeHTTP(w, r)
		logger.Info("request completed", "method", r.Method, "path", r.URL.Path, "duration", time.Since(started))
	})
}
