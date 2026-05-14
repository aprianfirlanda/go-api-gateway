package httpserver

import (
	"encoding/json"
	"net/http"

	"backend/internal/ports/input"
)

type HealthHandler struct {
	service input.HealthService
}

func NewHealthHandler(service input.HealthService) *HealthHandler {
	return &HealthHandler{service: service}
}

func (h *HealthHandler) Liveness(w http.ResponseWriter, r *http.Request) {
	if err := h.service.Liveness(r.Context()); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "unhealthy"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "healthy"})
}

func (h *HealthHandler) Readiness(w http.ResponseWriter, r *http.Request) {
	if err := h.service.Readiness(r.Context()); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			"status": "not_ready",
			"error":  err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, status int, code string, message string) {
	writeJSON(w, status, map[string]map[string]string{
		"error": {
			"code":    code,
			"message": message,
		},
	})
}
