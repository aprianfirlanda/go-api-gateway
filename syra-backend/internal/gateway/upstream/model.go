package upstream

type Protocol string

const (
	ProtocolREST    Protocol = "rest"
	ProtocolISO8583 Protocol = "iso8583"
)

type Upstream struct {
	ID               string
	TenantID         string
	Name             string
	Protocol         Protocol
	BaseURL          string
	ISO8583ProfileID string
}
