package httpserver

import (
	"errors"
	"net/http"

	"syra-backend/internal/auth"
	"syra-backend/internal/gateway/route"
)

type GatewayHandler struct {
	routes route.Registry
}

func NewGatewayHandler(routes route.Registry) *GatewayHandler {
	return &GatewayHandler{routes: routes}
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

	writeJSON(w, http.StatusOK, map[string]string{
		"status":       "matched",
		"tenantId":     principal.TenantID,
		"consumerId":   principal.ConsumerID,
		"credentialId": principal.CredentialID,
		"routeId":      matchedRoute.ID,
	})
}
