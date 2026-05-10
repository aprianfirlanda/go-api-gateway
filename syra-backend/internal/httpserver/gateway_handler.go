package httpserver

import (
	"context"
	"errors"
	"io"
	"net/http"
	"time"

	"syra-backend/internal/auth"
	"syra-backend/internal/gateway/route"
	"syra-backend/internal/gateway/upstream"
	"syra-backend/internal/protocol"
	restprotocol "syra-backend/internal/protocol/rest"
	"syra-backend/internal/transform"
)

type GatewayHandler struct {
	routes     route.Registry
	upstreams  upstream.Store
	adapters   *protocol.Registry
	templates  transform.Store
	transforms *transform.Engine
}

func NewGatewayHandler(routes route.Registry, upstreams upstream.Store, adapters *protocol.Registry, templates transform.Store, transforms *transform.Engine) *GatewayHandler {
	return &GatewayHandler{
		routes:     routes,
		upstreams:  upstreams,
		adapters:   adapters,
		templates:  templates,
		transforms: transforms,
	}
}

func (h *GatewayHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	principal, ok := auth.PrincipalFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "Missing principal")
		return
	}

	matchedRoute, err := h.routes.Match(r.Context(), route.MatchRequest{
		TenantID: principal.TenantID,
		Host:     r.Host,
		Method:   r.Method,
		Path:     r.URL.Path,
	})
	if err != nil {
		if errors.Is(err, route.ErrNotFound) {
			writeError(w, http.StatusNotFound, "route_not_found", "Route not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal_error", "Internal server error")
		return
	}

	target, err := h.upstreams.Find(r.Context(), principal.TenantID, matchedRoute.UpstreamRef)
	if err != nil {
		if errors.Is(err, upstream.ErrNotFound) {
			writeError(w, http.StatusBadGateway, "upstream_not_found", "Upstream not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal_error", "Internal server error")
		return
	}

	sourceProtocol := protocolName(matchedRoute.InboundProtocol, restprotocol.Name)
	targetProtocol := protocolName(matchedRoute.OutboundProtocol, string(target.Protocol))

	protocolAdapter, ok := h.adapters.Protocol(sourceProtocol)
	if !ok {
		writeError(w, http.StatusBadGateway, "protocol_adapter_not_found", "Protocol adapter not found")
		return
	}

	upstreamAdapter, ok := h.adapters.Upstream(targetProtocol)
	if !ok {
		writeError(w, http.StatusBadGateway, "upstream_adapter_not_found", "Upstream adapter not found")
		return
	}

	ctx := r.Context()
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
		writeError(w, http.StatusBadRequest, "decode_error", "Request could not be decoded")
		return
	}

	var template transform.Template
	if matchedRoute.TemplateRef != "" {
		if h.templates == nil {
			writeError(w, http.StatusBadGateway, "template_not_found", "Transformation template not found")
			return
		}
		template, err = h.templates.Find(ctx, principal.TenantID, matchedRoute.TemplateRef)
		if err != nil {
			if errors.Is(err, transform.ErrTemplateNotFound) {
				writeError(w, http.StatusBadGateway, "template_not_found", "Transformation template not found")
				return
			}
			writeError(w, http.StatusInternalServerError, "internal_error", "Internal server error")
			return
		}
		msg, err = h.transforms.DryRun(ctx, template, transform.DirectionRequest, msg)
		if err != nil {
			writeError(w, http.StatusBadRequest, "transformation_error", "Request transformation failed")
			return
		}
	}

	upstreamMsg, err := upstreamAdapter.Call(ctx, protocol.UpstreamTarget{
		ID:       target.ID,
		TenantID: target.TenantID,
		Protocol: string(target.Protocol),
		BaseURL:  target.BaseURL,
		Metadata: map[string]string{
			"iso8583ProfileId": target.ISO8583ProfileID,
		},
	}, msg)
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			writeError(w, http.StatusGatewayTimeout, "upstream_timeout", "Upstream request timed out")
			return
		}
		writeError(w, http.StatusBadGateway, "upstream_error", "Upstream request failed")
		return
	}

	if matchedRoute.TemplateRef != "" {
		upstreamMsg, err = h.transforms.DryRun(ctx, template, transform.DirectionResponse, upstreamMsg)
		if err != nil {
			writeError(w, http.StatusBadGateway, "transformation_error", "Response transformation failed")
			return
		}
	}

	outbound, err := protocolAdapter.Encode(ctx, upstreamMsg)
	if err != nil {
		writeError(w, http.StatusBadGateway, "encode_error", "Response could not be encoded")
		return
	}

	writeOutboundResponse(w, outbound)
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
