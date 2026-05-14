package mcp

import (
	"crypto/rand"
	"encoding/base64"
	"net/http"
	"time"

	"backend/internal/auth"
)

func (s *server) rotateAPIKey(w http.ResponseWriter, r *http.Request) {
	identity, ok := IdentityFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "MCP identity missing")
		return
	}

	var req struct {
		TenantID     string `json:"tenantId"`
		CredentialID string `json:"credentialId"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "validation_error", "Invalid request body")
		return
	}
	if req.TenantID == "" || req.CredentialID == "" {
		writeError(w, http.StatusBadRequest, "validation_error", "tenantId and credentialId are required")
		return
	}
	if !s.canAccessTenant(identity, req.TenantID) {
		writeError(w, http.StatusForbidden, "forbidden", "Tenant access denied")
		return
	}

	credential, err := s.store.GetCredential(r.Context(), req.TenantID, req.CredentialID)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "Credential not found")
		return
	}
	keyPrefix, secret, err := newAPIKeyParts()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed generating API key")
		return
	}
	secretHash, err := auth.HashSecret(secret)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed hashing API key")
		return
	}
	credential.KeyPrefix = keyPrefix
	credential.SecretHash = secretHash
	credential.Status = "active"
	credential.UpdatedAt = s.now()
	if err := s.store.UpdateCredential(r.Context(), credential); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed rotating API key")
		return
	}
	s.audit(r.Context(), identity, req.TenantID, "mcp.rotate_api_key", "credential", credential.ID)

	writeJSON(w, http.StatusOK, ToolResponse{
		Tool: "rotateApiKey",
		Result: map[string]any{
			"id":        credential.ID,
			"type":      credential.Type,
			"keyPrefix": keyPrefix,
			"apiKey":    keyPrefix + "." + secret,
			"status":    credential.Status,
		},
	})
}

func newAPIKeyParts() (string, string, error) {
	prefixBytes := make([]byte, 9)
	secretBytes := make([]byte, 24)
	if _, err := rand.Read(prefixBytes); err != nil {
		return "", "", err
	}
	if _, err := rand.Read(secretBytes); err != nil {
		return "", "", err
	}
	prefix := "gw_live_" + base64.RawURLEncoding.EncodeToString(prefixBytes)
	secret := base64.RawURLEncoding.EncodeToString(secretBytes)
	return prefix, secret, nil
}

func parseRFC3339(value string) (*time.Time, error) {
	if value == "" {
		return nil, nil
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}
