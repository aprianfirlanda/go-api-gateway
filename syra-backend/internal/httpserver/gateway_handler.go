package httpserver

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"syra-backend/internal/auth"
	"syra-backend/internal/billing"
	"syra-backend/internal/gateway/policy"
	"syra-backend/internal/gateway/route"
	"syra-backend/internal/gateway/upstream"
	"syra-backend/internal/observability"
	"syra-backend/internal/protocol"
	"syra-backend/internal/protocol/iso8583"
	restprotocol "syra-backend/internal/protocol/rest"
	"syra-backend/internal/runtime/state"
	"syra-backend/internal/transform"
	"syra-backend/pkg/ids"
)

type GatewayHandler struct {
	routes       route.Registry
	upstreams    upstream.Store
	adapters     *protocol.Registry
	templates    transform.Store
	transforms   *transform.Engine
	policies     *policy.Pipeline
	runtimeState state.Store
	usageEvents  billing.UsageEventStore
	metrics      *observability.Metrics
	logger       *slog.Logger
}

func NewGatewayHandler(routes route.Registry, upstreams upstream.Store, adapters *protocol.Registry, templates transform.Store, transforms *transform.Engine, policies *policy.Pipeline, usageEvents billing.UsageEventStore, metrics *observability.Metrics, runtimeState state.Store, logger *slog.Logger) *GatewayHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &GatewayHandler{
		routes:       routes,
		upstreams:    upstreams,
		adapters:     adapters,
		templates:    templates,
		transforms:   transforms,
		policies:     policies,
		runtimeState: runtimeState,
		usageEvents:  usageEvents,
		metrics:      metrics,
		logger:       logger,
	}
}

func (h *GatewayHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := time.Now().UTC()
	usageEvent := billing.UsageEvent{
		EventID:        ids.New(),
		SourceProtocol: restprotocol.Name,
		Status:         billing.StatusFailed,
		OccurredAt:     start,
	}
	upstreamCalled := false
	defer func() {
		h.emitUsageEvent(r.Context(), usageEvent, start, upstreamCalled)
	}()

	writeAttemptError := func(status int, code string, message string, eventStatus string) {
		usageEvent.HTTPStatus = status
		usageEvent.Status = eventStatus
		writeError(w, status, code, message)
	}

	principal, ok := auth.PrincipalFromContext(r.Context())
	if !ok {
		writeAttemptError(http.StatusUnauthorized, "unauthorized", "Missing principal", billing.StatusRejected)
		return
	}
	usageEvent.TenantID = principal.TenantID
	usageEvent.ConsumerID = principal.ConsumerID

	matchedRoute, err := h.routes.Match(r.Context(), route.MatchRequest{
		TenantID: principal.TenantID,
		Host:     r.Host,
		Method:   r.Method,
		Path:     r.URL.Path,
	})
	if err != nil {
		if errors.Is(err, route.ErrNotFound) {
			writeAttemptError(http.StatusNotFound, "route_not_found", "Route not found", billing.StatusRejected)
			return
		}
		writeAttemptError(http.StatusInternalServerError, "internal_error", "Internal server error", billing.StatusFailed)
		return
	}
	usageEvent.APIProductID = matchedRoute.APIProductID
	usageEvent.RouteID = matchedRoute.ID
	if err := h.enforceScopes(matchedRoute, principal); err != nil {
		writeAttemptError(http.StatusForbidden, "forbidden", "Missing required scope", billing.StatusRejected)
		return
	}
	ctx := r.Context()
	bodyBytes, err := readRequestBody(r)
	if err != nil {
		writeAttemptError(http.StatusBadRequest, "invalid_request", "Invalid request body", billing.StatusRejected)
		return
	}
	if err := h.enforceHMAC(ctx, r, matchedRoute, principal, bodyBytes); err != nil {
		writeAttemptError(http.StatusForbidden, "forbidden", "Invalid request signature", billing.StatusRejected)
		return
	}
	if err := h.enforceIdempotency(ctx, r, matchedRoute, principal, bodyBytes); err != nil {
		writeAttemptError(http.StatusConflict, "idempotency_conflict", err.Error(), billing.StatusRejected)
		return
	}

	sourceProtocol := protocolName(matchedRoute.InboundProtocol, restprotocol.Name)
	usageEvent.SourceProtocol = sourceProtocol
	if err := h.evaluatePolicy(r.Context(), policy.Request{
		TenantID:   principal.TenantID,
		ConsumerID: principal.ConsumerID,
		RouteID:    matchedRoute.ID,
		Protocol:   sourceProtocol,
		RemoteAddr: r.RemoteAddr,
		SizeBytes:  r.ContentLength,
	}); err != nil {
		usageEvent.Status = billing.StatusRejected
		usageEvent.HTTPStatus = policyHTTPStatus(err)
		if errors.Is(err, policy.ErrRateLimited) {
			h.metrics.IncRateLimitReject(principal.TenantID, matchedRoute.ID)
		}
		if errors.Is(err, policy.ErrQuotaExceeded) {
			h.metrics.IncQuotaReject(principal.TenantID, matchedRoute.ID)
		}
		writePolicyError(w, err)
		return
	}

	target, err := h.upstreams.Find(r.Context(), principal.TenantID, matchedRoute.UpstreamRef)
	if err != nil {
		if errors.Is(err, upstream.ErrNotFound) {
			writeAttemptError(http.StatusBadGateway, "upstream_not_found", "Upstream not found", billing.StatusFailed)
			return
		}
		writeAttemptError(http.StatusInternalServerError, "internal_error", "Internal server error", billing.StatusFailed)
		return
	}

	targetProtocol := protocolName(matchedRoute.OutboundProtocol, string(target.Protocol))
	usageEvent.TargetProtocol = targetProtocol

	protocolAdapter, ok := h.adapters.Protocol(sourceProtocol)
	if !ok {
		h.metrics.IncProtocolAdapterError(sourceProtocol, "decode")
		writeAttemptError(http.StatusBadGateway, "protocol_adapter_not_found", "Protocol adapter not found", billing.StatusFailed)
		return
	}

	upstreamAdapter, ok := h.adapters.Upstream(targetProtocol)
	if !ok {
		h.metrics.IncProtocolAdapterError(targetProtocol, "upstream")
		writeAttemptError(http.StatusBadGateway, "upstream_adapter_not_found", "Upstream adapter not found", billing.StatusFailed)
		return
	}

	cancel := func() {}
	if matchedRoute.TimeoutMs > 0 {
		ctx, cancel = context.WithTimeout(ctx, time.Duration(matchedRoute.TimeoutMs)*time.Millisecond)
	}
	defer cancel()

	msg, err := protocolAdapter.Decode(ctx, protocol.InboundRequest{
		HTTPRequest:    r.WithContext(ctx),
		TenantID:       principal.TenantID,
		ConsumerID:     principal.ConsumerID,
		CredentialID:   principal.CredentialID,
		APIProductID:   matchedRoute.APIProductID,
		RouteID:        matchedRoute.ID,
		SourceProtocol: sourceProtocol,
		TargetProtocol: targetProtocol,
	})
	if err != nil {
		h.metrics.IncProtocolAdapterError(sourceProtocol, "decode")
		writeAttemptError(http.StatusBadRequest, "decode_error", "Request could not be decoded", billing.StatusFailed)
		return
	}

	var template transform.Template
	if matchedRoute.TemplateRef != "" {
		if h.templates == nil {
			writeAttemptError(http.StatusBadGateway, "template_not_found", "Transformation template not found", billing.StatusFailed)
			return
		}
		template, err = h.templates.Find(ctx, principal.TenantID, matchedRoute.TemplateRef)
		if err != nil {
			if errors.Is(err, transform.ErrTemplateNotFound) {
				writeAttemptError(http.StatusBadGateway, "template_not_found", "Transformation template not found", billing.StatusFailed)
				return
			}
			writeAttemptError(http.StatusInternalServerError, "internal_error", "Internal server error", billing.StatusFailed)
			return
		}
		if template.Status != transform.StatusPublished {
			writeAttemptError(http.StatusBadGateway, "template_not_published", "Transformation template is not published", billing.StatusFailed)
			return
		}
		msg, err = h.transforms.DryRun(ctx, template, transform.DirectionRequest, msg)
		if err != nil {
			writeAttemptError(http.StatusBadRequest, "transformation_error", "Request transformation failed", billing.StatusFailed)
			return
		}
	}

	upstreamCalled = true
	upstreamMsg, err := upstreamAdapter.Call(ctx, protocol.UpstreamTarget{
		ID:       target.ID,
		TenantID: target.TenantID,
		Protocol: string(target.Protocol),
		BaseURL:  target.BaseURL,
		Metadata: map[string]string{
			"iso8583ProfileId": target.ISO8583ProfileID,
			"soapAction":       target.SOAPAction,
			"soapOperation":    target.SOAPOperation,
			"soapNamespace":    target.SOAPNamespace,
			"soapResponsePath": target.SOAPResponsePath,
		},
	}, msg)
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			if targetProtocol == iso8583.Name {
				h.metrics.IncISO8583Timeout(principal.TenantID, matchedRoute.ID)
			}
			writeAttemptError(http.StatusGatewayTimeout, "upstream_timeout", "Upstream request timed out", billing.StatusTimeout)
			return
		}
		h.metrics.IncProtocolAdapterError(targetProtocol, "upstream")
		writeAttemptError(http.StatusBadGateway, "upstream_error", "Upstream request failed", billing.StatusFailed)
		return
	}
	if upstreamMsg.StatusCode != 0 {
		usageEvent.UpstreamStatus = strconv.Itoa(upstreamMsg.StatusCode)
	}

	if matchedRoute.TemplateRef != "" {
		upstreamMsg, err = h.transforms.DryRun(ctx, template, transform.DirectionResponse, upstreamMsg)
		if err != nil {
			writeAttemptError(http.StatusBadGateway, "transformation_error", "Response transformation failed", billing.StatusFailed)
			return
		}
	}

	outbound, err := protocolAdapter.Encode(ctx, upstreamMsg)
	if err != nil {
		h.metrics.IncProtocolAdapterError(sourceProtocol, "encode")
		writeAttemptError(http.StatusBadGateway, "encode_error", "Response could not be encoded", billing.StatusFailed)
		return
	}

	usageEvent.HTTPStatus = outbound.StatusCode
	if usageEvent.HTTPStatus == 0 {
		usageEvent.HTTPStatus = http.StatusOK
	}
	if usageEvent.HTTPStatus >= http.StatusInternalServerError {
		usageEvent.Status = billing.StatusFailed
	} else {
		usageEvent.Status = billing.StatusSuccess
	}
	writeOutboundResponse(w, outbound)
}

func (h *GatewayHandler) enforceScopes(r route.Route, principal auth.Principal) error {
	if len(r.RequiredScopes) == 0 {
		return nil
	}
	granted := map[string]struct{}{}
	for _, scope := range principal.Scopes {
		granted[scope] = struct{}{}
	}
	for _, required := range r.RequiredScopes {
		if _, ok := granted[required]; !ok {
			return fmt.Errorf("missing scope %s", required)
		}
	}
	return nil
}

func (h *GatewayHandler) enforceHMAC(ctx context.Context, req *http.Request, r route.Route, principal auth.Principal, body []byte) error {
	if !r.HMACEnabled {
		return nil
	}
	signatureHeader := strings.TrimSpace(req.Header.Get("X-Signature"))
	timestampHeader := strings.TrimSpace(req.Header.Get("X-Timestamp"))
	nonce := strings.TrimSpace(req.Header.Get("X-Nonce"))
	if signatureHeader == "" || timestampHeader == "" || nonce == "" || r.HMACSecret == "" {
		return fmt.Errorf("missing signature headers")
	}
	timestamp, err := time.Parse(time.RFC3339, timestampHeader)
	if err != nil {
		return err
	}
	window := time.Duration(r.ReplayWindowSec) * time.Second
	if window <= 0 {
		window = 5 * time.Minute
	}
	now := time.Now().UTC()
	if now.Sub(timestamp) > window || timestamp.Sub(now) > window {
		return fmt.Errorf("stale timestamp")
	}
	hash := sha256.Sum256(body)
	bodyHash := hex.EncodeToString(hash[:])
	canonical := strings.Join([]string{
		req.Method,
		req.URL.Path,
		req.URL.RawQuery,
		timestampHeader,
		nonce,
		bodyHash,
	}, "\n")
	mac := hmac.New(sha256.New, []byte(r.HMACSecret))
	_, _ = mac.Write([]byte(canonical))
	expected := "hmac-sha256=" + hex.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(expected), []byte(signatureHeader)) {
		return fmt.Errorf("invalid signature")
	}
	if h.runtimeState != nil {
		nonceKey := state.Key{
			TenantID: principal.TenantID,
			Feature:  "replay_nonce",
			Name:     principal.ConsumerID + ":" + principal.CredentialID + ":" + nonce,
		}
		ok, err := h.runtimeState.CompareAndSet(ctx, nonceKey, "", "1", window)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("replayed nonce")
		}
	}
	return nil
}

func (h *GatewayHandler) enforceIdempotency(ctx context.Context, req *http.Request, r route.Route, principal auth.Principal, body []byte) error {
	if !r.IdempotencyEnabled {
		return nil
	}
	switch req.Method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
	default:
		return nil
	}
	key := strings.TrimSpace(req.Header.Get("Idempotency-Key"))
	if key == "" {
		return fmt.Errorf("missing idempotency key")
	}
	if h.runtimeState == nil {
		return nil
	}
	requestHash := sha256.Sum256(append([]byte(req.Method+":"+req.URL.Path+"?"+req.URL.RawQuery+":"), body...))
	hashValue := hex.EncodeToString(requestHash[:])
	ttl := time.Duration(r.IdempotencyTTLSec) * time.Second
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	stateKey := state.Key{
		TenantID: principal.TenantID,
		Feature:  "idempotency",
		Name:     r.ID + ":" + principal.ConsumerID + ":" + key,
	}
	ok, err := h.runtimeState.CompareAndSet(ctx, stateKey, "", hashValue, ttl)
	if err != nil {
		return err
	}
	if ok {
		return nil
	}
	existing, found, err := h.runtimeState.Get(ctx, stateKey)
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("idempotency conflict")
	}
	if existing == hashValue {
		return fmt.Errorf("duplicate idempotency key")
	}
	return fmt.Errorf("idempotency key reused with different request")
}

func readRequestBody(req *http.Request) ([]byte, error) {
	if req.Body == nil {
		return nil, nil
	}
	body, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}
	req.Body = io.NopCloser(bytes.NewReader(body))
	return body, nil
}

func (h *GatewayHandler) emitUsageEvent(ctx context.Context, event billing.UsageEvent, start time.Time, upstreamCalled bool) {
	if h.usageEvents == nil {
		return
	}
	if event.HTTPStatus == 0 {
		event.HTTPStatus = http.StatusInternalServerError
	}
	event.LatencyMs = time.Since(start).Milliseconds()
	event.Billable = billing.BillableForStatus(event.Status, upstreamCalled)
	if err := h.usageEvents.Save(context.WithoutCancel(ctx), event); err != nil {
		h.metrics.IncBillingEventFailure(event.TenantID, event.RouteID)
		h.logger.ErrorContext(ctx, "billing usage event write failed", slog.String("tenant_id", event.TenantID), slog.String("route_id", event.RouteID), slog.Any("error", err))
	}
}

func (h *GatewayHandler) evaluatePolicy(ctx context.Context, req policy.Request) error {
	if req.SizeBytes < 0 {
		req.SizeBytes = 0
	}
	return h.policies.Evaluate(ctx, req)
}

func protocolName(value string, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func writeOutboundResponse(w http.ResponseWriter, resp protocol.OutboundResponse) {
	if resp.Body != nil {
		defer resp.Body.Close()
	}

	for name, values := range resp.Headers {
		if restprotocol.IsHopByHopHeader(name) {
			continue
		}
		for _, value := range values {
			w.Header().Add(name, value)
		}
	}

	status := resp.StatusCode
	if status == 0 {
		status = http.StatusOK
	}
	w.WriteHeader(status)

	if resp.Body != nil {
		_, _ = io.Copy(w, resp.Body)
	}
}

func writePolicyError(w http.ResponseWriter, err error) {
	writeError(w, policyHTTPStatus(err), policyErrorCode(err), policyErrorMessage(err))
}

func policyHTTPStatus(err error) int {
	switch {
	case errors.Is(err, policy.ErrBlockedIP):
		return http.StatusForbidden
	case errors.Is(err, policy.ErrRequestTooLarge):
		return http.StatusRequestEntityTooLarge
	case errors.Is(err, policy.ErrRateLimited):
		return http.StatusTooManyRequests
	case errors.Is(err, policy.ErrQuotaExceeded):
		return http.StatusForbidden
	default:
		return http.StatusInternalServerError
	}
}

func policyErrorCode(err error) string {
	switch {
	case errors.Is(err, policy.ErrBlockedIP):
		return "blocked_ip"
	case errors.Is(err, policy.ErrRequestTooLarge):
		return "request_too_large"
	case errors.Is(err, policy.ErrRateLimited):
		return "rate_limited"
	case errors.Is(err, policy.ErrQuotaExceeded):
		return "quota_exceeded"
	default:
		return "policy_error"
	}
}

func policyErrorMessage(err error) string {
	switch {
	case errors.Is(err, policy.ErrBlockedIP):
		return "IP address is not allowed"
	case errors.Is(err, policy.ErrRequestTooLarge):
		return "Request is too large"
	case errors.Is(err, policy.ErrRateLimited):
		return "Rate limit exceeded"
	case errors.Is(err, policy.ErrQuotaExceeded):
		return "Quota exceeded"
	default:
		return "Policy evaluation failed"
	}
}
