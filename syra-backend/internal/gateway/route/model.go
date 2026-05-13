package route

type Status string

const (
	StatusDraft    Status = "draft"
	StatusActive   Status = "active"
	StatusDisabled Status = "disabled"
)

type Route struct {
	ID                 string
	TenantID           string
	APIProductID       string
	InboundProtocol    string
	OutboundProtocol   string
	Host               string
	Method             string
	Path               string
	UpstreamRef        string
	TemplateRef        string
	RateLimitPolicyID  string
	QuotaPolicyID      string
	TimeoutMs          int
	RequiredScopes     []string
	HMACEnabled        bool
	HMACSecret         string
	ReplayWindowSec    int
	IdempotencyEnabled bool
	IdempotencyTTLSec  int
	Status             Status
}
