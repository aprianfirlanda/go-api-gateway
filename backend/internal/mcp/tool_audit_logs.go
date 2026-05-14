package mcp

import (
	"net/http"

	"backend/internal/controlplane"
)

func (s *server) auditLogs(w http.ResponseWriter, r *http.Request) {
	identity, ok := IdentityFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "MCP identity missing")
		return
	}

	tenantID := r.URL.Query().Get("tenantId")
	if tenantID == "" && identity.Role == "tenant_admin" {
		tenantID = identity.TenantID
	}
	if tenantID != "" && !s.canAccessTenant(identity, tenantID) {
		writeError(w, http.StatusForbidden, "forbidden", "Tenant access denied")
		return
	}
	if tenantID == "" && identity.Role != "platform_admin" {
		writeError(w, http.StatusForbidden, "forbidden", "Platform admin required for global audit logs")
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
	filter := controlplane.AuditFilter{
		TenantID: tenantID,
		ActorID:  r.URL.Query().Get("actorId"),
		Action:   r.URL.Query().Get("action"),
		Resource: r.URL.Query().Get("resource"),
		From:     from,
		To:       to,
	}
	items, err := s.store.ListAuditEvents(r.Context(), filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed loading audit logs")
		return
	}
	s.audit(r.Context(), identity, tenantID, "mcp.audit_logs", "audit_log", "")
	writeJSON(w, http.StatusOK, ToolResponse{
		Tool:   "auditLogs",
		Result: items,
	})
}
