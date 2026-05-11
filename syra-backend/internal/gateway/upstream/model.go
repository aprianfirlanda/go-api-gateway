package upstream

type Protocol string

const (
	ProtocolREST    Protocol = "rest"
	ProtocolISO8583 Protocol = "iso8583"
	ProtocolSOAPXML Protocol = "soap_xml"
)

type Upstream struct {
	ID               string
	TenantID         string
	Name             string
	Protocol         Protocol
	BaseURL          string
	ISO8583ProfileID string
	SOAPAction       string
	SOAPOperation    string
	SOAPNamespace    string
	SOAPResponsePath string
}
