package mcp

import (
	"net/http"
	"strconv"

	"backend/internal/billing"
)

func (s *server) usageReport(w http.ResponseWriter, r *http.Request) {
	identity, ok := IdentityFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "MCP identity missing")
		return
	}
	if s.usage == nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Usage store unavailable")
		return
	}

	tenantID := r.URL.Query().Get("tenantId")
	if tenantID == "" && identity.Role == "tenant_admin" {
		tenantID = identity.TenantID
	}
	if tenantID == "" {
		writeError(w, http.StatusBadRequest, "validation_error", "tenantId is required")
		return
	}
	if !s.canAccessTenant(identity, tenantID) {
		writeError(w, http.StatusForbidden, "forbidden", "Tenant access denied")
		return
	}

	from, err := parseRFC3339(r.URL.Query().Get("from"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "validation_error", "invalid from")
		return
	}
	to, err := parseRFC3339(r.URL.Query().Get("to"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "validation_error", "invalid to")
		return
	}
	limit := 50
	if raw := r.URL.Query().Get("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed <= 0 {
			writeError(w, http.StatusBadRequest, "validation_error", "invalid limit")
			return
		}
		limit = parsed
	}

	filter := billing.UsageEventFilter{
		TenantID:       tenantID,
		RouteID:        r.URL.Query().Get("routeId"),
		ConsumerID:     r.URL.Query().Get("consumerId"),
		Status:         r.URL.Query().Get("status"),
		SourceProtocol: r.URL.Query().Get("sourceProtocol"),
		TargetProtocol: r.URL.Query().Get("targetProtocol"),
		From:           from,
		To:             to,
	}
	page, err := s.usage.ListPage(r.Context(), filter, limit, r.URL.Query().Get("cursor"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed loading usage report")
		return
	}
	s.audit(r.Context(), identity, tenantID, "mcp.usage_report", "usage", "")
	writeJSON(w, http.StatusOK, ToolResponse{
		Tool: "usageReport",
		Result: map[string]any{
			"data":       page.Data,
			"nextCursor": page.NextCursor,
		},
	})
}
