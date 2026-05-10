package httpserver

import (
	"errors"
	"net/http"
	"strings"

	"syra-backend/internal/auth"
)

func APIKeyAuth(store auth.CredentialStore) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rawKey, ok := apiKeyFromRequest(r)
			if !ok {
				writeError(w, http.StatusUnauthorized, "unauthorized", "Missing API key")
				return
			}

			principal, err := auth.AuthenticateAPIKey(r.Context(), store, rawKey)
			if err != nil {
				switch {
				case errors.Is(err, auth.ErrCredentialForbidden):
					writeError(w, http.StatusForbidden, "forbidden", "Credential is not allowed")
				default:
					writeError(w, http.StatusUnauthorized, "unauthorized", "Invalid API key")
				}
				return
			}

			next.ServeHTTP(w, r.WithContext(auth.ContextWithPrincipal(r.Context(), principal)))
		})
	}
}

func apiKeyFromRequest(r *http.Request) (string, bool) {
	if value := strings.TrimSpace(r.Header.Get("X-API-Key")); value != "" {
		return value, true
	}

	const prefix = "ApiKey "
	value := r.Header.Get("Authorization")
	if !strings.HasPrefix(value, prefix) {
		return "", false
	}

	key := strings.TrimSpace(strings.TrimPrefix(value, prefix))
	return key, key != ""
}
