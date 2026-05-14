package state

import (
	"context"
	"time"
)

type Key struct {
	TenantID string
	Feature  string
	Name     string
}

type Store interface {
	Set(ctx context.Context, key Key, value string, ttl time.Duration) error
	Get(ctx context.Context, key Key) (string, bool, error)
	Increment(ctx context.Context, key Key, delta int64, ttl time.Duration) (int64, error)
	Expire(ctx context.Context, key Key, ttl time.Duration) error
	Delete(ctx context.Context, key Key) error
	CompareAndSet(ctx context.Context, key Key, expected, next string, ttl time.Duration) (bool, error)
}

type Namespacer struct {
	Environment string
	Version     string
}

func (n Namespacer) Format(key Key) string {
	env := n.Environment
	if env == "" {
		env = "dev"
	}
	version := n.Version
	if version == "" {
		version = "v1"
	}
	return env + ":tenant:" + key.TenantID + ":feature:" + key.Feature + ":version:" + version + ":" + key.Name
}
