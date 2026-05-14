package runtimeconfig

import (
	"fmt"
	"time"

	"backend/internal/auth"
	"backend/internal/gateway/policy"
	"backend/internal/gateway/route"
	"backend/internal/gateway/upstream"
	"backend/internal/protocol/iso8583"
	"backend/internal/transform"
)

type Snapshot struct {
	Version     int64
	Checksum    string
	Status      string
	PublishedAt time.Time
	Routes      []route.Route
	APIProducts []policy.APIProductBinding
	Upstreams   []upstream.Upstream
	Credentials []auth.APIKeyCredential
	Templates   []transform.Template
	Profiles    []iso8583.Profile
	RateLimits  []policy.RateLimitConfig
	Quotas      []policy.QuotaConfig
}

func (s Snapshot) Validate() error {
	if s.Version <= 0 {
		return fmt.Errorf("config snapshot version is required")
	}
	upstreamsByTenant := map[string]map[string]struct{}{}
	for _, item := range s.Upstreams {
		if item.ID == "" || item.TenantID == "" || item.Protocol == "" {
			return fmt.Errorf("upstream id, tenant, and protocol are required")
		}
		if upstreamsByTenant[item.TenantID] == nil {
			upstreamsByTenant[item.TenantID] = map[string]struct{}{}
		}
		upstreamsByTenant[item.TenantID][item.ID] = struct{}{}
	}
	for _, item := range s.Routes {
		if item.ID == "" || item.TenantID == "" {
			return fmt.Errorf("route id and tenant are required")
		}
		if item.Status == route.StatusActive {
			if item.InboundProtocol == "" || item.OutboundProtocol == "" || item.UpstreamRef == "" {
				return fmt.Errorf("active route %s has incomplete protocol or upstream config", item.ID)
			}
			if _, ok := upstreamsByTenant[item.TenantID][item.UpstreamRef]; !ok {
				return fmt.Errorf("active route %s references missing upstream %s", item.ID, item.UpstreamRef)
			}
		}
	}
	for _, credential := range s.Credentials {
		if credential.ID == "" || credential.TenantID == "" || credential.KeyPrefix == "" || credential.SecretHash == "" {
			return fmt.Errorf("credential id, tenant, key prefix, and secret hash are required")
		}
	}
	for _, template := range s.Templates {
		if template.ID == "" || template.TenantID == "" {
			return fmt.Errorf("template id and tenant are required")
		}
	}
	for _, profile := range s.Profiles {
		if profile.ID == "" {
			return fmt.Errorf("iso8583 profile id is required")
		}
	}
	return nil
}
