package policy

import (
	"context"
	"sync"
	"time"
)

type InMemoryRateLimiter struct {
	mu       sync.Mutex
	limit    int
	window   time.Duration
	counters map[string]counter
}

type counter struct {
	windowStart time.Time
	count       int
}

func NewInMemoryRateLimiter(limit int, window time.Duration) *InMemoryRateLimiter {
	return &InMemoryRateLimiter{
		limit:    limit,
		window:   window,
		counters: map[string]counter{},
	}
}

func (l *InMemoryRateLimiter) Allow(ctx context.Context, key string, now time.Time) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	if l == nil || l.limit <= 0 || l.window <= 0 {
		return true, nil
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	current := l.counters[key]
	if current.windowStart.IsZero() || now.Sub(current.windowStart) >= l.window {
		current = counter{windowStart: now}
	}
	if current.count >= l.limit {
		l.counters[key] = current
		return false, nil
	}
	current.count++
	l.counters[key] = current
	return true, nil
}
