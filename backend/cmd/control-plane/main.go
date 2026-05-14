package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	controlplaneapp "backend/internal/app/controlplane"
	"backend/internal/config"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg := config.Load()
	logger := newLogger(cfg.LogLevel)

	app, err := controlplaneapp.New(cfg, logger)
	if err != nil {
		logger.Error("build control plane app", slog.Any("error", err))
		os.Exit(1)
	}
	defer app.Close()

	errCh := make(chan error, 1)
	go func() {
		logger.Info("starting control plane", slog.String("addr", cfg.ControlPlaneAddr))
		errCh <- app.Server.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
	case err := <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("control plane stopped", slog.Any("error", err))
			os.Exit(1)
		}
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()

	if err := app.Server.Shutdown(shutdownCtx); err != nil {
		logger.Error("shutdown control plane", slog.Any("error", err))
		os.Exit(1)
	}

	logger.Info("control plane stopped")
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
