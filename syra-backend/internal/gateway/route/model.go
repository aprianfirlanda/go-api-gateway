package route

type Status string

const (
	StatusDraft    Status = "draft"
	StatusActive   Status = "active"
	StatusDisabled Status = "disabled"
)

type Route struct {
	ID               string
	TenantID         string
	APIProductID     string
	InboundProtocol  string
	OutboundProtocol string
	Host             string
	Method           string
	Path             string
	UpstreamRef      string
	TemplateRef      string
	TimeoutMs        int
	Status           Status
}
