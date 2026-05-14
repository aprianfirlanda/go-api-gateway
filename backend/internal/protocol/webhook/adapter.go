package webhook

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"backend/internal/protocol"
	restprotocol "backend/internal/protocol/rest"
)

const Name = "webhook"

type Adapter struct {
	client *http.Client
}

func NewAdapter(client *http.Client) *Adapter {
	if client == nil {
		client = http.DefaultClient
	}
	return &Adapter{client: client}
}

func (a *Adapter) Name() string { return Name }

func (a *Adapter) Call(ctx context.Context, target protocol.UpstreamTarget, msg protocol.CanonicalMessage) (protocol.CanonicalMessage, error) {
	if strings.ToLower(target.Protocol) != Name {
		return protocol.CanonicalMessage{}, fmt.Errorf("unsupported upstream protocol %q", target.Protocol)
	}
	method := strings.ToUpper(strings.TrimSpace(target.Metadata["webhookMethod"]))
	if method == "" {
		method = http.MethodPost
	}
	path := strings.TrimSpace(target.Metadata["webhookPath"])
	targetURL, err := buildTargetURL(target.BaseURL, path)
	if err != nil {
		return protocol.CanonicalMessage{}, err
	}
	payload := map[string]any{
		"event":   target.Metadata["webhookEventType"],
		"routeId": msg.RouteID,
		"data":    msg.Fields,
	}
	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return protocol.CanonicalMessage{}, fmt.Errorf("encode webhook payload: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, method, targetURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return protocol.CanonicalMessage{}, fmt.Errorf("build webhook request: %w", err)
	}
	restprotocol.CopyForwardHeaders(req.Header, msg.Headers)
	req.Header.Set("Content-Type", "application/json")
	if eventType := strings.TrimSpace(target.Metadata["webhookEventType"]); eventType != "" {
		req.Header.Set("X-Webhook-Event", eventType)
	}
	if secret := strings.TrimSpace(target.Metadata["webhookSecret"]); secret != "" {
		hash := sha256.Sum256(bodyBytes)
		mac := hmac.New(sha256.New, []byte(secret))
		_, _ = mac.Write(hash[:])
		req.Header.Set("X-Webhook-Signature", "sha256="+hex.EncodeToString(mac.Sum(nil)))
	}
	resp, err := a.client.Do(req)
	if err != nil {
		return protocol.CanonicalMessage{}, err
	}
	defer resp.Body.Close()
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return protocol.CanonicalMessage{}, fmt.Errorf("read webhook response: %w", err)
	}
	fields := map[string]any{}
	if len(responseBody) > 0 {
		if err := json.Unmarshal(responseBody, &fields); err != nil {
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
		Method:         msg.Method,
		Path:           msg.Path,
		RawQuery:       msg.RawQuery,
		Headers:        resp.Header.Clone(),
		Fields:         fields,
		Metadata:       map[string]any{},
		Body:           io.NopCloser(bytes.NewReader(responseBody)),
		StatusCode:     resp.StatusCode,
		SensitiveKeys:  msg.SensitiveKeys,
	}, nil
}

func buildTargetURL(baseURL, path string) (string, error) {
	base, err := url.Parse(baseURL)
	if err != nil {
		return "", fmt.Errorf("parse upstream base url: %w", err)
	}
	if base.Scheme == "" || base.Host == "" {
		return "", fmt.Errorf("upstream base url must include scheme and host")
	}
	base.Path = joinPath(base.Path, path)
	base.RawQuery = ""
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
