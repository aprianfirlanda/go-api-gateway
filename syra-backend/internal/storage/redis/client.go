package redis

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

type Config struct {
	Addr    string
	Timeout time.Duration
}

func Open(ctx context.Context, cfg Config, logger *slog.Logger) (*goredis.Client, error) {
	if cfg.Addr == "" {
		return nil, fmt.Errorf("redis addr is required")
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 2 * time.Second
	}
	if logger == nil {
		logger = slog.Default()
	}
	client := goredis.NewClient(&goredis.Options{
		Addr:         cfg.Addr,
		DialTimeout:  cfg.Timeout,
		ReadTimeout:  cfg.Timeout,
		WriteTimeout: cfg.Timeout,
	})
	pingCtx, cancel := context.WithTimeout(ctx, cfg.Timeout)
	defer cancel()
	if err := client.Ping(pingCtx).Err(); err != nil {
		_ = client.Close()
		logger.Error("redis ping failed", slog.String("addr", cfg.Addr), slog.Any("error", err))
		return nil, err
	}
	logger.Info("redis connected", slog.String("addr", cfg.Addr), slog.Duration("timeout", cfg.Timeout))
	return client, nil
}
