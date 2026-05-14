package mcp

import (
	"net/http"
)

func (s *server) listTenants(w http.ResponseWriter, r *http.Request) {
	identity, ok := IdentityFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "MCP identity missing")
		return
	}
	if identity.Role != "platform_admin" {
		writeError(w, http.StatusForbidden, "forbidden", "Platform admin required")
		return
	}
	items, err := s.store.ListTenants(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to list tenants")
		return
	}
	s.audit(r.Context(), identity, "", "mcp.list_tenants", "tenant", "")
	writeJSON(w, http.StatusOK, ToolResponse{
		Tool:   "listTenants",
		Result: items,
	})
}
