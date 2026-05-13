package policy

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"syra-backend/internal/runtime/state"
)

type DistributedRateLimiter struct {
	state state.Store
}

func NewDistributedRateLimiter(store state.Store) *DistributedRateLimiter {
	return &DistributedRateLimiter{state: store}
}

func (l *DistributedRateLimiter) Allow(ctx context.Context, key string, now time.Time, config RateLimitConfig) (bool, error) {
	if l == nil || l.state == nil {
		return true, nil
	}
	if config.LimitCount <= 0 || config.WindowSeconds <= 0 {
		return true, nil
	}
	switch config.Algorithm {
	case "", "fixed_window":
		return l.allowFixedWindow(ctx, key, now, config)
	case "sliding_window":
		return l.allowSlidingWindow(ctx, key, now, config)
	default:
		return false, fmt.Errorf("unsupported rate limit algorithm %q", config.Algorithm)
	}
}

func (l *DistributedRateLimiter) allowFixedWindow(ctx context.Context, key string, now time.Time, config RateLimitConfig) (bool, error) {
	window := time.Duration(config.WindowSeconds) * time.Second
	windowStart := now.UTC().Truncate(window).Unix()
	stateKey := state.Key{TenantID: "shared", Feature: "rate_limit_fixed", Name: key + ":" + strconv.FormatInt(windowStart, 10)}
	count, err := l.state.Increment(ctx, stateKey, 1, window)
	if err != nil {
		return false, err
	}
	return count <= int64(config.LimitCount+config.BurstCount), nil
}

func (l *DistributedRateLimiter) allowSlidingWindow(ctx context.Context, key string, now time.Time, config RateLimitConfig) (bool, error) {
	// Two-window approximation for sliding behavior.
	window := time.Duration(config.WindowSeconds) * time.Second
	currentStart := now.UTC().Truncate(window)
	prevStart := currentStart.Add(-window)

	currentKey := state.Key{TenantID: "shared", Feature: "rate_limit_sliding", Name: key + ":" + strconv.FormatInt(currentStart.Unix(), 10)}
	prevKey := state.Key{TenantID: "shared", Feature: "rate_limit_sliding", Name: key + ":" + strconv.FormatInt(prevStart.Unix(), 10)}
	current, err := l.state.Increment(ctx, currentKey, 1, window*2)
	if err != nil {
		return false, err
	}
	prevRaw, found, err := l.state.Get(ctx, prevKey)
	if err != nil {
		return false, err
	}
	var prev int64
	if found {
		prev, _ = strconv.ParseInt(prevRaw, 10, 64)
	}
	elapsed := now.Sub(currentStart)
	weight := float64(window-elapsed) / float64(window)
	estimate := float64(current) + float64(prev)*weight
	return estimate <= float64(config.LimitCount+config.BurstCount), nil
}

type DistributedQuotaChecker struct {
	state state.Store
}

func NewDistributedQuotaChecker(store state.Store) *DistributedQuotaChecker {
	return &DistributedQuotaChecker{state: store}
}

func (c *DistributedQuotaChecker) Allow(ctx context.Context, req Request, config QuotaConfig) (bool, error) {
	if c == nil || c.state == nil {
		return true, nil
	}
	if config.QuotaCount <= 0 {
		return true, nil
	}
	periodKey := quotaPeriodKey(config.Period, req.Now.UTC())
	ttl := quotaPeriodTTL(config.Period, req.Now.UTC())
	key := state.Key{
		TenantID: req.TenantID,
		Feature:  "quota_counter",
		Name:     req.ConsumerID + ":" + req.RouteID + ":" + periodKey,
	}
	count, err := c.state.Increment(ctx, key, 1, ttl)
	if err != nil {
		return false, err
	}
	if count <= config.QuotaCount {
		return true, nil
	}
	return config.ExceededBehavior == "allow_overage", nil
}

func quotaPeriodKey(period string, now time.Time) string {
	switch period {
	case "daily":
		return now.Format("2006-01-02")
	case "weekly":
		y, w := now.ISOWeek()
		return fmt.Sprintf("%d-W%02d", y, w)
	default:
		return now.Format("2006-01")
	}
}

func quotaPeriodTTL(period string, now time.Time) time.Duration {
	switch period {
	case "daily":
		next := now.Truncate(24 * time.Hour).Add(24 * time.Hour)
		return next.Sub(now) + time.Hour
	case "weekly":
		daysToMonday := (8 - int(now.Weekday())) % 7
		if daysToMonday == 0 {
			daysToMonday = 7
		}
		next := now.Truncate(24*time.Hour).AddDate(0, 0, daysToMonday)
		return next.Sub(now) + 24*time.Hour
	default:
		next := time.Date(now.Year(), now.Month()+1, 1, 0, 0, 0, 0, now.Location())
		return next.Sub(now) + 24*time.Hour
	}
}
