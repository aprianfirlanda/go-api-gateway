package controlplane

import (
	"encoding/json"
	"time"
)

const (
	StatusActive    = "active"
	StatusDraft     = "draft"
	StatusDisabled  = "disabled"
	StatusSuspended = "suspended"
	StatusRevoked   = "revoked"
)

type Tenant struct {
	ID            string         `json:"id"`
	Name          string         `json:"name"`
	Slug          string         `json:"slug"`
	Status        string         `json:"status"`
	BillingPlanID string         `json:"billingPlanId,omitempty"`
	Metadata      map[string]any `json:"metadata,omitempty"`
	CreatedAt     time.Time      `json:"createdAt"`
	UpdatedAt     time.Time      `json:"updatedAt"`
}

type APIProduct struct {
	ID          string    `json:"id"`
	TenantID    string    `json:"tenantId"`
	Name        string    `json:"name"`
	Slug        string    `json:"slug"`
	Description string    `json:"description,omitempty"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

type Upstream struct {
	ID        string          `json:"id"`
	TenantID  string          `json:"tenantId"`
	Name      string          `json:"name"`
	Protocol  string          `json:"protocol"`
	Config    json.RawMessage `json:"config,omitempty"`
	SecretRef *string         `json:"secretRef,omitempty"`
	Status    string          `json:"status"`
	CreatedAt time.Time       `json:"createdAt"`
	UpdatedAt time.Time       `json:"updatedAt"`
}

type Route struct {
	ID                       string    `json:"id"`
	TenantID                 string    `json:"tenantId"`
	APIProductID             string    `json:"apiProductId"`
	Name                     string    `json:"name"`
	InboundProtocol          string    `json:"inboundProtocol"`
	OutboundProtocol         string    `json:"outboundProtocol"`
	Host                     string    `json:"host"`
	Method                   string    `json:"method"`
	Path                     string    `json:"path"`
	UpstreamID               string    `json:"upstreamId"`
	TransformationTemplateID string    `json:"transformationTemplateId,omitempty"`
	RateLimitPolicyID        string    `json:"rateLimitPolicyId,omitempty"`
	QuotaPolicyID            string    `json:"quotaPolicyId,omitempty"`
	Priority                 int       `json:"priority"`
	TimeoutMs                int       `json:"timeoutMs"`
	RequiredScopes           []string  `json:"requiredScopes,omitempty"`
	HMACEnabled              bool      `json:"hmacEnabled,omitempty"`
	HMACSecret               string    `json:"hmacSecret,omitempty"`
	ReplayWindowSec          int       `json:"replayWindowSec,omitempty"`
	IdempotencyEnabled       bool      `json:"idempotencyEnabled,omitempty"`
	IdempotencyTTLSec        int       `json:"idempotencyTtlSec,omitempty"`
	Status                   string    `json:"status"`
	CreatedAt                time.Time `json:"createdAt"`
	UpdatedAt                time.Time `json:"updatedAt"`
}

type Consumer struct {
	ID          string    `json:"id"`
	TenantID    string    `json:"tenantId"`
	Name        string    `json:"name"`
	Slug        string    `json:"slug"`
	OwnerUserID string    `json:"ownerUserId,omitempty"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

type Credential struct {
	ID         string     `json:"id"`
	TenantID   string     `json:"tenantId"`
	ConsumerID string     `json:"consumerId"`
	Type       string     `json:"type"`
	KeyPrefix  string     `json:"keyPrefix"`
	SecretHash string     `json:"-"`
	Scopes     []string   `json:"scopes,omitempty"`
	Status     string     `json:"status"`
	ExpiresAt  *time.Time `json:"expiresAt,omitempty"`
	CreatedAt  time.Time  `json:"createdAt"`
	UpdatedAt  time.Time  `json:"updatedAt"`
}

type CredentialCreateResponse struct {
	ID        string     `json:"id"`
	Type      string     `json:"type"`
	KeyPrefix string     `json:"keyPrefix"`
	APIKey    string     `json:"apiKey,omitempty"`
	Status    string     `json:"status"`
	ExpiresAt *time.Time `json:"expiresAt,omitempty"`
}

type TransformationTemplate struct {
	ID             string          `json:"id"`
	TenantID       string          `json:"tenantId"`
	APIProductID   string          `json:"apiProductId"`
	Name           string          `json:"name"`
	SourceProtocol string          `json:"sourceProtocol"`
	TargetProtocol string          `json:"targetProtocol"`
	TemplateBody   json.RawMessage `json:"templateBody"`
	Version        int             `json:"version"`
	Status         string          `json:"status"`
	CreatedAt      time.Time       `json:"createdAt"`
	UpdatedAt      time.Time       `json:"updatedAt"`
	PublishedAt    *time.Time      `json:"publishedAt,omitempty"`
}

type AuditEvent struct {
	ID         string          `json:"id"`
	ActorID    string          `json:"actorId"`
	TenantID   string          `json:"tenantId,omitempty"`
	Action     string          `json:"action"`
	Resource   string          `json:"resource"`
	ResourceID string          `json:"resourceId"`
	Metadata   json.RawMessage `json:"metadata,omitempty"`
	OccurredAt time.Time       `json:"occurredAt"`
}

type listResponse[T any] struct {
	Data       []T     `json:"data"`
	NextCursor *string `json:"nextCursor"`
}
