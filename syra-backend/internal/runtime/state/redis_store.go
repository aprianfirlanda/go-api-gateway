package state

import (
	"context"
	"log/slog"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"syra-backend/internal/observability"
)

type RedisStore struct {
	client     *goredis.Client
	namespacer Namespacer
	metrics    *observability.Metrics
	logger     *slog.Logger
}

func NewRedisStore(client *goredis.Client, namespacer Namespacer, metrics *observability.Metrics, logger *slog.Logger) *RedisStore {
	if logger == nil {
		logger = slog.Default()
	}
	return &RedisStore{
		client:     client,
		namespacer: namespacer,
		metrics:    metrics,
		logger:     logger,
	}
}

func (s *RedisStore) Set(ctx context.Context, key Key, value string, ttl time.Duration) error {
	err := s.client.Set(ctx, s.namespacer.Format(key), value, ttl).Err()
	s.observe("set", err)
	return err
}

func (s *RedisStore) Get(ctx context.Context, key Key) (string, bool, error) {
	value, err := s.client.Get(ctx, s.namespacer.Format(key)).Result()
	if err == goredis.Nil {
		s.observe("get", nil)
		return "", false, nil
	}
	s.observe("get", err)
	if err != nil {
		return "", false, err
	}
	return value, true, nil
}

func (s *RedisStore) Increment(ctx context.Context, key Key, delta int64, ttl time.Duration) (int64, error) {
	fullKey := s.namespacer.Format(key)
	next, err := s.client.IncrBy(ctx, fullKey, delta).Result()
	if err != nil {
		s.observe("incr", err)
		return 0, err
	}
	if ttl > 0 {
		expires, ttlErr := s.client.TTL(ctx, fullKey).Result()
		if ttlErr != nil {
			s.observe("ttl", ttlErr)
			return 0, ttlErr
		}
		if expires < 0 {
			if err := s.client.Expire(ctx, fullKey, ttl).Err(); err != nil {
				s.observe("expire", err)
				return 0, err
			}
		}
	}
	s.observe("incr", nil)
	return next, nil
}

func (s *RedisStore) Expire(ctx context.Context, key Key, ttl time.Duration) error {
	err := s.client.Expire(ctx, s.namespacer.Format(key), ttl).Err()
	s.observe("expire", err)
	return err
}

func (s *RedisStore) Delete(ctx context.Context, key Key) error {
	err := s.client.Del(ctx, s.namespacer.Format(key)).Err()
	s.observe("delete", err)
	return err
}

func (s *RedisStore) CompareAndSet(ctx context.Context, key Key, expected, next string, ttl time.Duration) (bool, error) {
	fullKey := s.namespacer.Format(key)
	applied := false
	err := s.client.Watch(ctx, func(tx *goredis.Tx) error {
		current, err := tx.Get(ctx, fullKey).Result()
		if err != nil && err != goredis.Nil {
			return err
		}
		if err == goredis.Nil {
			current = ""
		}
		if current != expected {
			return nil
		}
		_, execErr := tx.TxPipelined(ctx, func(pipe goredis.Pipeliner) error {
			pipe.Set(ctx, fullKey, next, ttl)
			return nil
		})
		if execErr == nil {
			applied = true
		}
		return execErr
	}, fullKey)
	s.observe("cas", err)
	if err != nil {
		return false, err
	}
	return applied, nil
}

func (s *RedisStore) observe(op string, err error) {
	if s.metrics != nil {
		s.metrics.ObserveRedis(op, err == nil)
	}
	if err != nil {
		s.logger.Warn("redis runtime state operation failed", slog.String("operation", op), slog.Any("error", err))
	}
}
