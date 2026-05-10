package upstream

type Protocol string

const (
	ProtocolREST Protocol = "rest"
)

type Upstream struct {
	ID       string
	TenantID string
	Name     string
	Protocol Protocol
	BaseURL  string
}
