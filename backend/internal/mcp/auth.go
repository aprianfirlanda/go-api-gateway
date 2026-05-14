package mcp

import (
	"context"
	"net/http"
	"strings"
)

type authContextKey struct{}
type identityContextKey struct{}

func withAuthenticatedContext(ctx context.Context) context.Context {
	return context.WithValue(ctx, authContextKey{}, true)
}

func withIdentityContext(ctx context.Context, identity Identity) context.Context {
	return context.WithValue(ctx, identityContextKey{}, identity)
}

func IsAuthenticated(ctx context.Context) bool {
	ok, _ := ctx.Value(authContextKey{}).(bool)
	return ok
}

func IdentityFromContext(ctx context.Context) (Identity, bool) {
	identity, ok := ctx.Value(identityContextKey{}).(Identity)
	return identity, ok
}

func AuthMiddleware(token string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if token == "" {
				writeError(w, http.StatusUnauthorized, "unauthorized", "MCP auth token is not configured")
				return
			}
			if extractToken(r) != token {
				writeError(w, http.StatusUnauthorized, "unauthorized", "Invalid MCP token")
				return
			}
			identity := Identity{
				ActorID:  strings.TrimSpace(r.Header.Get("X-MCP-Actor-ID")),
				Role:     strings.TrimSpace(r.Header.Get("X-MCP-Role")),
				TenantID: strings.TrimSpace(r.Header.Get("X-MCP-Tenant-ID")),
			}
			if identity.ActorID == "" {
				identity.ActorID = "mcp_operator"
			}
			if identity.Role == "" {
				identity.Role = "platform_admin"
			}
			ctx := withAuthenticatedContext(r.Context())
			ctx = withIdentityContext(ctx, identity)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func extractToken(r *http.Request) string {
	if raw := strings.TrimSpace(r.Header.Get("X-MCP-Token")); raw != "" {
		return raw
	}
	const bearerPrefix = "Bearer "
	authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
	if strings.HasPrefix(authHeader, bearerPrefix) {
		return strings.TrimSpace(strings.TrimPrefix(authHeader, bearerPrefix))
	}
	return ""
}
