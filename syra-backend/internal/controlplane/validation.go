package controlplane

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"syra-backend/internal/protocol/graphql"
	"syra-backend/internal/protocol/iso8583"
	restprotocol "syra-backend/internal/protocol/rest"
	"syra-backend/internal/protocol/soapxml"
	"syra-backend/internal/protocol/webhook"
)

func validateUpstreamConfig(protocolName string, raw json.RawMessage) error {
	if len(raw) == 0 {
		return fmt.Errorf("config is required")
	}
	cfg := map[string]any{}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return fmt.Errorf("invalid upstream config")
	}
	baseURL, _ := cfg["baseUrl"].(string)
	switch strings.ToLower(strings.TrimSpace(protocolName)) {
	case restprotocol.Name:
		if strings.TrimSpace(baseURL) == "" {
			return fmt.Errorf("config.baseUrl is required")
		}
		return nil
	case iso8583.Name:
		profileID, _ := cfg["profileId"].(string)
		if strings.TrimSpace(profileID) == "" {
			return fmt.Errorf("config.profileId is required for iso8583")
		}
		host, _ := cfg["host"].(string)
		if strings.TrimSpace(baseURL) == "" && strings.TrimSpace(host) == "" {
			return fmt.Errorf("config.baseUrl or config.host is required for iso8583")
		}
		return nil
	case soapxml.Name:
		if strings.TrimSpace(baseURL) == "" {
			return fmt.Errorf("config.baseUrl is required")
		}
		return nil
	case graphql.Name:
		if strings.TrimSpace(baseURL) == "" {
			return fmt.Errorf("config.baseUrl is required")
		}
		query, _ := cfg["query"].(string)
		if strings.TrimSpace(query) == "" {
			return fmt.Errorf("config.query is required for graphql")
		}
		return nil
	case webhook.Name:
		if strings.TrimSpace(baseURL) == "" {
			return fmt.Errorf("config.baseUrl is required")
		}
		method, _ := cfg["method"].(string)
		if method != "" {
			switch strings.ToUpper(strings.TrimSpace(method)) {
			case http.MethodPost, http.MethodPut, http.MethodPatch:
			default:
				return fmt.Errorf("config.method must be POST, PUT, or PATCH for webhook")
			}
		}
		return nil
	default:
		return fmt.Errorf("unsupported protocol %q", protocolName)
	}
}

func validateRouteProtocols(inbound, outbound string) error {
	if !strings.EqualFold(strings.TrimSpace(inbound), restprotocol.Name) {
		return fmt.Errorf("unsupported inboundProtocol %q", inbound)
	}
	switch strings.ToLower(strings.TrimSpace(outbound)) {
	case restprotocol.Name, iso8583.Name, soapxml.Name, graphql.Name, webhook.Name:
		return nil
	default:
		return fmt.Errorf("unsupported outboundProtocol %q", outbound)
	}
}

func validateRouteUpstreamCompatibility(item Route, up Upstream) error {
	if !strings.EqualFold(item.OutboundProtocol, up.Protocol) {
		return fmt.Errorf("route outboundProtocol %q must match upstream protocol %q", item.OutboundProtocol, up.Protocol)
	}
	return validateUpstreamConfig(up.Protocol, up.Config)
}
