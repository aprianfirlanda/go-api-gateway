package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	gatewayapp "syra-backend/internal/app/gateway"
	"syra-backend/internal/config"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg := config.Load()
	logger := newLogger(cfg.LogLevel)

	app, err := gatewayapp.New(cfg, logger)
	if err != nil {
		logger.Error("build gateway app", slog.Any("error", err))
		os.Exit(1)
	}
	defer app.Close()

	errCh := make(chan error, 1)
	go func() {
		logger.Info("starting gateway", slog.String("addr", cfg.GatewayAddr))
		errCh <- app.Server.ListenAndServe()
	}()
	if app.ConfigReload != nil {
		app.ConfigReload.Start(ctx, cfg.ConfigReloadInterval)
	}

	select {
	case <-ctx.Done():
	case err := <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("gateway stopped", slog.Any("error", err))
			os.Exit(1)
		}
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()

	if err := app.Server.Shutdown(shutdownCtx); err != nil {
		logger.Error("shutdown gateway", slog.Any("error", err))
		os.Exit(1)
	}

	logger.Info("gateway stopped")
}

func newLogger(level string) *slog.Logger {
	var slogLevel slog.Level
	switch level {
	case "debug":
		slogLevel = slog.LevelDebug
	case "warn":
		slogLevel = slog.LevelWarn
	case "error":
		slogLevel = slog.LevelError
	default:
		slogLevel = slog.LevelInfo
	}

	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slogLevel}))
}
