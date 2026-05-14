package mcp

import (
	"time"

	"backend/internal/billing"
	"backend/internal/controlplane"
)

type Config struct {
	AuthToken string
	Now       func() time.Time
	Store     controlplane.Repository
	Usage     billing.UsageEventStore
}

type HealthResponse struct {
	Service   string    `json:"service"`
	Status    string    `json:"status"`
	Timestamp time.Time `json:"timestamp"`
}

type ErrorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type ToolResponse struct {
	Tool   string `json:"tool"`
	Result any    `json:"result"`
}

type Identity struct {
	ActorID  string
	Role     string
	TenantID string
}
