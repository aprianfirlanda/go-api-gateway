package main

import (
	"context"
	"log/slog"
	"os"

	"backend/internal/config"
	"backend/internal/storage/postgres"
)

func main() {
	cfg := config.Load()
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if cfg.DatabaseURL == "" {
		logger.Error("database url is required", slog.String("env", "DATABASE_URL"))
		os.Exit(1)
	}
	if err := postgres.Migrate(context.Background(), cfg.DatabaseURL, "migrations"); err != nil {
		logger.Error("run migrations", slog.Any("error", err))
		os.Exit(1)
	}
	logger.Info("migrations completed")
}
