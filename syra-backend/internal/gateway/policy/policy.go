package policy

import (
	"context"
	"errors"
	"net"
	"time"
)

var (
	ErrBlockedIP       = errors.New("blocked ip")
	ErrRequestTooLarge = errors.New("request too large")
	ErrRateLimited     = errors.New("rate limited")
	ErrQuotaExceeded   = errors.New("quota exceeded")
)

type Request struct {
	TenantID   string
	ConsumerID string
	RouteID    string
	Protocol   string
	RemoteAddr string
	SizeBytes  int64
	Now        time.Time
}

type Policy interface {
	Name() string
	Evaluate(ctx context.Context, req Request) error
}

type Pipeline struct {
	policies []Policy
}

func NewPipeline(policies ...Policy) *Pipeline {
	return &Pipeline{policies: append([]Policy(nil), policies...)}
}

func (p *Pipeline) Evaluate(ctx context.Context, req Request) error {
	if p == nil {
		return nil
	}
	if req.Now.IsZero() {
		req.Now = time.Now().UTC()
	}
	for _, policy := range p.policies {
		if err := policy.Evaluate(ctx, req); err != nil {
			return err
		}
	}
	return nil
}

type IPAllowlistPolicy struct {
	allowed []*net.IPNet
}

func NewIPAllowlistPolicy(cidrs ...string) (*IPAllowlistPolicy, error) {
	p := &IPAllowlistPolicy{}
	for _, cidr := range cidrs {
		_, network, err := net.ParseCIDR(cidr)
		if err != nil {
			ip := net.ParseIP(cidr)
			if ip == nil {
				return nil, err
			}
			bits := 32
			if ip.To4() == nil {
				bits = 128
			}
			network = &net.IPNet{IP: ip, Mask: net.CIDRMask(bits, bits)}
		}
		p.allowed = append(p.allowed, network)
	}
	return p, nil
}

func (p *IPAllowlistPolicy) Name() string { return "ip_allowlist" }

func (p *IPAllowlistPolicy) Evaluate(ctx context.Context, req Request) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if len(p.allowed) == 0 {
		return nil
	}
	ip := parseRemoteIP(req.RemoteAddr)
	if ip == nil {
		return ErrBlockedIP
	}
	for _, network := range p.allowed {
		if network.Contains(ip) {
			return nil
		}
	}
	return ErrBlockedIP
}

type RequestSizeLimitPolicy struct {
	limit int64
}

func NewRequestSizeLimitPolicy(limit int64) *RequestSizeLimitPolicy {
	return &RequestSizeLimitPolicy{limit: limit}
}

func (p *RequestSizeLimitPolicy) Name() string { return "request_size_limit" }

func (p *RequestSizeLimitPolicy) Evaluate(ctx context.Context, req Request) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if p.limit > 0 && req.SizeBytes > p.limit {
		return ErrRequestTooLarge
	}
	return nil
}

type RateLimiter interface {
	Allow(ctx context.Context, key string, now time.Time) (bool, error)
}

type RateLimitPolicy struct {
	limiter RateLimiter
}

func NewRateLimitPolicy(limiter RateLimiter) *RateLimitPolicy {
	return &RateLimitPolicy{limiter: limiter}
}

func (p *RateLimitPolicy) Name() string { return "rate_limit" }

func (p *RateLimitPolicy) Evaluate(ctx context.Context, req Request) error {
	if p.limiter == nil {
		return nil
	}
	ok, err := p.limiter.Allow(ctx, req.TenantID+":"+req.ConsumerID+":"+req.RouteID, req.Now)
	if err != nil {
		return err
	}
	if !ok {
		return ErrRateLimited
	}
	return nil
}

type QuotaChecker interface {
	Allow(ctx context.Context, req Request) (bool, error)
}

type QuotaPolicy struct {
	checker QuotaChecker
}

func NewQuotaPolicy(checker QuotaChecker) *QuotaPolicy {
	return &QuotaPolicy{checker: checker}
}

func (p *QuotaPolicy) Name() string { return "quota" }

func (p *QuotaPolicy) Evaluate(ctx context.Context, req Request) error {
	if p.checker == nil {
		return nil
	}
	ok, err := p.checker.Allow(ctx, req)
	if err != nil {
		return err
	}
	if !ok {
		return ErrQuotaExceeded
	}
	return nil
}

func parseRemoteIP(remoteAddr string) net.IP {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err == nil {
		return net.ParseIP(host)
	}
	return net.ParseIP(remoteAddr)
}
