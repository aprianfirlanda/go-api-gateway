package protocol

import (
	"io"
	"net/http"
)

type CanonicalMessage struct {
	TenantID       string
	ConsumerID     string
	CredentialID   string
	APIProductID   string
	RouteID        string
	SourceProtocol string
	TargetProtocol string
	Operation      string
	Method         string
	Path           string
	RawQuery       string
	Headers        http.Header
	Fields         map[string]any
	Metadata       map[string]any
	Body           io.ReadCloser
	StatusCode     int
	SensitiveKeys  []string
}

type InboundRequest struct {
	HTTPRequest    *http.Request
	TenantID       string
	ConsumerID     string
	CredentialID   string
	APIProductID   string
	RouteID        string
	SourceProtocol string
	TargetProtocol string
	Operation      string
}

type OutboundResponse struct {
	StatusCode int
	Headers    http.Header
	Body       io.ReadCloser
}

type UpstreamTarget struct {
	ID       string
	TenantID string
	Protocol string
	BaseURL  string
	Metadata map[string]string
}
