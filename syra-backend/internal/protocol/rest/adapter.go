package rest

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"syra-backend/internal/protocol"
)

const Name = "rest"

type Adapter struct {
	client *http.Client
}

func NewAdapter(client *http.Client) *Adapter {
	if client == nil {
		client = http.DefaultClient
	}
	return &Adapter{client: client}
}

func (a *Adapter) Name() string {
	return Name
}

func (a *Adapter) Decode(ctx context.Context, req protocol.InboundRequest) (protocol.CanonicalMessage, error) {
	if err := ctx.Err(); err != nil {
		return protocol.CanonicalMessage{}, err
	}
	if req.HTTPRequest == nil {
		return protocol.CanonicalMessage{}, fmt.Errorf("http request is required")
	}

	fields := map[string]any{}
	body := req.HTTPRequest.Body
	if body != nil {
		bodyBytes, err := io.ReadAll(body)
		if err != nil {
			return protocol.CanonicalMessage{}, fmt.Errorf("read request body: %w", err)
		}
		_ = body.Close()
		if len(bodyBytes) > 0 {
			if err := json.Unmarshal(bodyBytes, &fields); err != nil {
				fields = map[string]any{}
			}
		}
		body = io.NopCloser(bytes.NewReader(bodyBytes))
	}

	return protocol.CanonicalMessage{
		TenantID:       req.TenantID,
		ConsumerID:     req.ConsumerID,
		CredentialID:   req.CredentialID,
		APIProductID:   req.APIProductID,
		RouteID:        req.RouteID,
		SourceProtocol: req.SourceProtocol,
		TargetProtocol: req.TargetProtocol,
		Operation:      req.Operation,
		Method:         req.HTTPRequest.Method,
		Path:           req.HTTPRequest.URL.Path,
		RawQuery:       req.HTTPRequest.URL.RawQuery,
		Headers:        req.HTTPRequest.Header.Clone(),
		Fields:         fields,
		Metadata:       map[string]any{},
		Body:           body,
	}, nil
}

func (a *Adapter) Encode(ctx context.Context, msg protocol.CanonicalMessage) (protocol.OutboundResponse, error) {
	if err := ctx.Err(); err != nil {
		return protocol.OutboundResponse{}, err
	}

	status := msg.StatusCode
	if status == 0 {
		status = http.StatusOK
	}
	if msg.Headers == nil {
		msg.Headers = http.Header{}
	}

	body := msg.Body
	headers := msg.Headers
	if headers == nil {
		headers = http.Header{}
	}
	if body == nil && msg.Fields != nil {
		bodyBytes, err := json.Marshal(msg.Fields)
		if err != nil {
			return protocol.OutboundResponse{}, fmt.Errorf("encode response json: %w", err)
		}
		body = io.NopCloser(bytes.NewReader(bodyBytes))
		if headers.Get("Content-Type") == "" {
			headers.Set("Content-Type", "application/json")
		}
	}

	return protocol.OutboundResponse{
		StatusCode: status,
		Headers:    headers,
		Body:       body,
	}, nil
}

func (a *Adapter) Call(ctx context.Context, target protocol.UpstreamTarget, msg protocol.CanonicalMessage) (protocol.CanonicalMessage, error) {
	if strings.ToLower(target.Protocol) != Name {
		return protocol.CanonicalMessage{}, fmt.Errorf("unsupported upstream protocol %q", target.Protocol)
	}

	targetURL, err := buildTargetURL(target.BaseURL, msg.Path, msg.RawQuery)
	if err != nil {
		return protocol.CanonicalMessage{}, err
	}

	body := msg.Body
	if body == nil && msg.Fields != nil {
		bodyBytes, err := json.Marshal(msg.Fields)
		if err != nil {
			return protocol.CanonicalMessage{}, fmt.Errorf("encode upstream request json: %w", err)
		}
		body = io.NopCloser(bytes.NewReader(bodyBytes))
		if msg.Headers == nil {
			msg.Headers = http.Header{}
		}
		if msg.Headers.Get("Content-Type") == "" {
			msg.Headers.Set("Content-Type", "application/json")
		}
	}

	req, err := http.NewRequestWithContext(ctx, msg.Method, targetURL, body)
	if err != nil {
		return protocol.CanonicalMessage{}, fmt.Errorf("build upstream request: %w", err)
	}

	CopyForwardHeaders(req.Header, msg.Headers)

	resp, err := a.client.Do(req)
	if err != nil {
		return protocol.CanonicalMessage{}, err
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return protocol.CanonicalMessage{}, fmt.Errorf("read upstream response: %w", err)
	}

	fields := map[string]any{}
	if len(bodyBytes) > 0 {
		if err := json.Unmarshal(bodyBytes, &fields); err != nil {
			fields = map[string]any{}
		}
	}

	return protocol.CanonicalMessage{
		TenantID:       msg.TenantID,
		ConsumerID:     msg.ConsumerID,
		CredentialID:   msg.CredentialID,
		APIProductID:   msg.APIProductID,
		RouteID:        msg.RouteID,
		SourceProtocol: msg.SourceProtocol,
		TargetProtocol: msg.TargetProtocol,
		Operation:      msg.Operation,
		Method:         msg.Method,
		Path:           msg.Path,
		RawQuery:       msg.RawQuery,
		Headers:        resp.Header.Clone(),
		Fields:         fields,
		Metadata:       map[string]any{},
		Body:           io.NopCloser(bytes.NewReader(bodyBytes)),
		StatusCode:     resp.StatusCode,
		SensitiveKeys:  msg.SensitiveKeys,
	}, nil
}

func buildTargetURL(baseURL string, path string, rawQuery string) (string, error) {
	base, err := url.Parse(baseURL)
	if err != nil {
		return "", fmt.Errorf("parse upstream base url: %w", err)
	}
	if base.Scheme == "" || base.Host == "" {
		return "", fmt.Errorf("upstream base url must include scheme and host")
	}

	base.Path = joinPath(base.Path, path)
	base.RawQuery = rawQuery
	base.Fragment = ""

	return base.String(), nil
}

func joinPath(basePath string, requestPath string) string {
	basePath = strings.TrimRight(basePath, "/")
	if requestPath == "" {
		requestPath = "/"
	}
	if !strings.HasPrefix(requestPath, "/") {
		requestPath = "/" + requestPath
	}
	return basePath + requestPath
}

func CopyForwardHeaders(dst http.Header, src http.Header) {
	for name, values := range src {
		if !IsForwardedRequestHeader(name) {
			continue
		}
		for _, value := range values {
			dst.Add(name, value)
		}
	}
}

func IsForwardedRequestHeader(name string) bool {
	if IsHopByHopHeader(name) {
		return false
	}

	switch http.CanonicalHeaderKey(name) {
	case "Authorization", "Proxy-Authorization", "X-Api-Key":
		return false
	default:
		return true
	}
}

func IsHopByHopHeader(name string) bool {
	switch http.CanonicalHeaderKey(name) {
	case "Connection",
		"Keep-Alive",
		"Proxy-Authenticate",
		"Proxy-Authorization",
		"Te",
		"Trailer",
		"Transfer-Encoding",
		"Upgrade":
		return true
	default:
		return false
	}
}
