package soapxml

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"syra-backend/internal/protocol"
)

func TestAdapterGeneratesSOAPEnvelopeAndExtractsResponse(t *testing.T) {
	var requestBody string
	var soapAction string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		soapAction = r.Header.Get("SOAPAction")
		body, _ := io.ReadAll(r.Body)
		requestBody = string(body)
		w.Header().Set("Content-Type", "text/xml")
		_, _ = w.Write([]byte(`<?xml version="1.0"?>
<soapenv:Envelope xmlns:soapenv="http://schemas.xmlsoap.org/soap/envelope/">
  <soapenv:Body>
    <AuthorizeResponse>
      <responseCode>00</responseCode>
      <authorizationCode>A12345</authorizationCode>
    </AuthorizeResponse>
  </soapenv:Body>
</soapenv:Envelope>`))
	}))
	t.Cleanup(server.Close)

	msg, err := NewAdapter(server.Client()).Call(context.Background(), protocol.UpstreamTarget{
		Protocol: Name,
		BaseURL:  server.URL,
		Metadata: map[string]string{
			"soapAction":       "Authorize",
			"soapOperation":    "AuthorizeRequest",
			"soapNamespace":    "urn:test",
			"soapResponsePath": "AuthorizeResponse",
		},
	}, protocol.CanonicalMessage{
		Fields: map[string]any{
			"amount":     "000000010000",
			"terminalId": "ATM00101",
		},
	})

	require.NoError(t, err)
	require.Equal(t, "Authorize", soapAction)
	require.Contains(t, requestBody, `<soapenv:Envelope`)
	require.Contains(t, requestBody, `xmlns:ns="urn:test"`)
	require.Contains(t, requestBody, `<ns:AuthorizeRequest>`)
	require.Contains(t, requestBody, `<ns:amount>000000010000</ns:amount>`)
	require.Equal(t, "00", msg.Fields["responseCode"])
	require.Equal(t, "A12345", msg.Fields["authorizationCode"])
}
