package httpserver

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"syra-backend/internal/auth"
	"syra-backend/internal/billing"
	restprotocol "syra-backend/internal/protocol/rest"
	"syra-backend/pkg/ids"
)

func APIKeyAuth(store auth.CredentialStore, usageEvents billing.UsageEventStore) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rawKey, ok := apiKeyFromRequest(r)
			if !ok {
				emitAuthRejectedUsageEvent(r.Context(), usageEvents, store, "", http.StatusUnauthorized)
				writeError(w, http.StatusUnauthorized, "unauthorized", "Missing API key")
				return
			}

			principal, err := auth.AuthenticateAPIKey(r.Context(), store, rawKey)
			if err != nil {
				switch {
				case errors.Is(err, auth.ErrCredentialForbidden):
					emitAuthRejectedUsageEvent(r.Context(), usageEvents, store, rawKey, http.StatusForbidden)
					writeError(w, http.StatusForbidden, "forbidden", "Credential is not allowed")
				default:
					emitAuthRejectedUsageEvent(r.Context(), usageEvents, store, rawKey, http.StatusUnauthorized)
					writeError(w, http.StatusUnauthorized, "unauthorized", "Invalid API key")
				}
				return
			}

			next.ServeHTTP(w, r.WithContext(auth.ContextWithPrincipal(r.Context(), principal)))
		})
	}
}

func emitAuthRejectedUsageEvent(ctx context.Context, usageEvents billing.UsageEventStore, store auth.CredentialStore, rawKey string, status int) {
	if usageEvents == nil {
		return
	}

	now := time.Now().UTC()
	event := billing.UsageEvent{
		EventID:        ids.New(),
		SourceProtocol: restprotocol.Name,
		Status:         billing.StatusRejected,
		HTTPStatus:     status,
		Billable:       false,
		OccurredAt:     now,
	}

	if rawKey != "" && store != nil {
		if prefix, _, err := auth.ParseAPIKey(rawKey); err == nil {
			if credential, err := store.FindByPrefix(context.WithoutCancel(ctx), prefix); err == nil {
				event.TenantID = credential.TenantID
				event.ConsumerID = credential.ConsumerID
			}
		}
	}

	_ = usageEvents.Save(context.WithoutCancel(ctx), event)
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
