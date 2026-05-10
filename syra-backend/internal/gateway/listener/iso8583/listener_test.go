package iso8583listener

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"syra-backend/internal/gateway/policy"
	"syra-backend/internal/gateway/upstream"
	"syra-backend/internal/protocol/iso8583"
	restprotocol "syra-backend/internal/protocol/rest"
	"syra-backend/internal/transform"
)

func TestISO8583InboundFlow(t *testing.T) {
	codec := iso8583.NewInternalCodec()
	profile := testProfile()
	var restBody map[string]any

	restServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "/authorizations", r.URL.Path)
		require.NoError(t, json.NewDecoder(r.Body).Decode(&restBody))

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"responseCode":"00","authorizationCode":"A12345","stan":"123456"}`))
	}))
	t.Cleanup(restServer.Close)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = listener.Close() })

	server := NewServer(Profile{
		ID:               "listener_1",
		TenantID:         "tenant_1",
		ConsumerID:       "switch_1",
		APIProductID:     "product_1",
		RouteID:          "iso_inbound_route",
		ISO8583ProfileID: profile.ID,
		TemplateID:       "template_1",
		RESTUpstream: upstream.Upstream{
			ID:       "rest_1",
			TenantID: "tenant_1",
			Protocol: upstream.ProtocolREST,
			BaseURL:  restServer.URL,
		},
		RESTMethod: http.MethodPost,
		RESTPath:   "/authorizations",
		Timeout:    time.Second,
	}, codec, iso8583.NewInMemoryProfileStore(profile), transform.NewInMemoryStore(testTemplate()), transform.NewEngine(), restprotocol.NewAdapter(http.DefaultClient), policy.NewPipeline())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		_ = server.Serve(ctx, listener)
	}()

	request, err := codec.Pack(profile, map[string]any{
		"mti": "0100",
		"2":   "4111111111111111",
		"3":   "000000",
		"4":   "000000010000",
		"11":  "123456",
		"41":  "ATM00101",
		"49":  "360",
	})
	require.NoError(t, err)

	conn, err := net.Dial("tcp", listener.Addr().String())
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	_, err = conn.Write(request)
	require.NoError(t, err)

	buffer := make([]byte, 1024)
	n, err := conn.Read(buffer)
	require.NoError(t, err)

	responseFields, err := codec.Unpack(profile, buffer[:n])
	require.NoError(t, err)
	require.Equal(t, "0110", responseFields["mti"])
	require.Equal(t, "123456", responseFields["11"])
	require.Equal(t, "A12345", responseFields["38"])
	require.Equal(t, "00", responseFields["39"])

	require.Equal(t, "4111111111111111", restBody["pan"])
	require.Equal(t, "000000010000", restBody["amount"])
	require.Equal(t, "360", restBody["currency"])
	require.Equal(t, "ATM00101", restBody["terminalId"])
}

func TestISO8583InboundRejectsMalformedMessage(t *testing.T) {
	codec := iso8583.NewInternalCodec()
	profile := testProfile()
	server := NewServer(Profile{
		ID:               "listener_1",
		TenantID:         "tenant_1",
		ISO8583ProfileID: profile.ID,
		TemplateID:       "template_1",
	}, codec, iso8583.NewInMemoryProfileStore(profile), transform.NewInMemoryStore(testTemplate()), transform.NewEngine(), restprotocol.NewAdapter(http.DefaultClient), policy.NewPipeline())

	_, err := server.HandleMessage(context.Background(), strings.NewReader("\x00\x04bad"))

	require.Error(t, err)
}

func TestISO8583InboundAppliesSharedPolicy(t *testing.T) {
	codec := iso8583.NewInternalCodec()
	profile := testProfile()
	server := NewServer(Profile{
		ID:               "listener_1",
		TenantID:         "tenant_1",
		ConsumerID:       "switch_1",
		RouteID:          "iso_inbound_route",
		ISO8583ProfileID: profile.ID,
		TemplateID:       "template_1",
	}, codec, iso8583.NewInMemoryProfileStore(profile), transform.NewInMemoryStore(testTemplate()), transform.NewEngine(), restprotocol.NewAdapter(http.DefaultClient), policy.NewPipeline(policy.NewRequestSizeLimitPolicy(5)))

	request, err := codec.Pack(profile, map[string]any{
		"mti": "0100",
		"3":   "000000",
	})
	require.NoError(t, err)

	_, err = server.HandleMessage(context.Background(), strings.NewReader(string(request)), "127.0.0.1:1234")

	require.ErrorIs(t, err, policy.ErrRequestTooLarge)
}

func testTemplate() transform.Template {
	return transform.Template{
		ID:             "template_1",
		TenantID:       "tenant_1",
		APIProductID:   "product_1",
		Name:           "iso8583-to-rest-card-auth",
		SourceProtocol: iso8583.Name,
		TargetProtocol: restprotocol.Name,
		Version:        1,
		Status:         transform.StatusPublished,
		Request: transform.Section{
			Fields: map[string]string{
				"pan":        "$.fields.2",
				"amount":     "$.fields.4",
				"currency":   "$.fields.49",
				"terminalId": "$.fields.41",
				"stan":       "$.fields.11",
			},
			Sensitive: []string{"pan"},
		},
		Response: transform.Section{
			Fields: map[string]string{
				"mti": "'0110'",
				"11":  "$.fields.stan",
				"38":  "$.fields.authorizationCode",
				"39":  "$.fields.responseCode",
			},
		},
	}
}

func testProfile() iso8583.Profile {
	return iso8583.Profile{
		ID:           "profile_1",
		MTI:          "0100",
		ResponseMTI:  "0110",
		LengthHeader: true,
		Fields: map[int]iso8583.FieldSpec{
			2:  {ID: 2, Type: iso8583.FieldLLVAR, Length: 19, Sensitive: true},
			3:  {ID: 3, Type: iso8583.FieldFixed, Length: 6},
			4:  {ID: 4, Type: iso8583.FieldFixed, Length: 12},
			11: {ID: 11, Type: iso8583.FieldFixed, Length: 6},
			38: {ID: 38, Type: iso8583.FieldFixed, Length: 6},
			39: {ID: 39, Type: iso8583.FieldFixed, Length: 2},
			41: {ID: 41, Type: iso8583.FieldFixed, Length: 8},
			49: {ID: 49, Type: iso8583.FieldFixed, Length: 3},
		},
		SensitiveKeys: []string{"2"},
	}
}
