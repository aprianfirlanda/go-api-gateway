package controlplane

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"syra-backend/internal/auth"
)

const (
	RolePlatformAdmin = "platform_admin"
	RoleTenantAdmin   = "tenant_admin"
)

var ErrUnauthorizedAdmin = errors.New("unauthorized admin")

type AdminIdentity struct {
	ActorID  string
	Role     string
	TenantID string
}

type AdminAPIKey struct {
	ActorID    string
	Role       string
	TenantID   string
	KeyPrefix  string
	SecretHash string
}

type AdminAuthenticator interface {
	Authenticate(r *http.Request) (AdminIdentity, error)
}

type StaticAdminAuthenticator struct {
	BootstrapToken string
	APIKeys        []AdminAPIKey
}

func (a StaticAdminAuthenticator) Authenticate(r *http.Request) (AdminIdentity, error) {
	if token := bearerTokenFromRequest(r); token != "" && token == a.BootstrapToken {
		return AdminIdentity{
			ActorID: "bootstrap_admin",
			Role:    RolePlatformAdmin,
		}, nil
	}

	rawKey, ok := adminAPIKeyFromRequest(r)
	if !ok {
		return AdminIdentity{}, ErrUnauthorizedAdmin
	}
	prefix, secret, err := auth.ParseAPIKey(rawKey)
	if err != nil {
		return AdminIdentity{}, ErrUnauthorizedAdmin
	}
	for _, candidate := range a.APIKeys {
		if candidate.KeyPrefix != prefix {
			continue
		}
		valid, err := auth.VerifySecret(secret, candidate.SecretHash)
		if err != nil || !valid {
			return AdminIdentity{}, ErrUnauthorizedAdmin
		}
		return AdminIdentity{
			ActorID:  candidate.ActorID,
			Role:     candidate.Role,
			TenantID: candidate.TenantID,
		}, nil
	}
	return AdminIdentity{}, ErrUnauthorizedAdmin
}

type adminIdentityContextKey struct{}

func contextWithAdminIdentity(ctx context.Context, identity AdminIdentity) context.Context {
	return context.WithValue(ctx, adminIdentityContextKey{}, identity)
}

func adminIdentityFromContext(ctx context.Context) (AdminIdentity, bool) {
	identity, ok := ctx.Value(adminIdentityContextKey{}).(AdminIdentity)
	return identity, ok
}

func bearerTokenFromRequest(r *http.Request) string {
	const prefix = "Bearer "
	value := r.Header.Get("Authorization")
	if !strings.HasPrefix(value, prefix) {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(value, prefix))
}

func adminAPIKeyFromRequest(r *http.Request) (string, bool) {
	if value := strings.TrimSpace(r.Header.Get("X-Admin-Api-Key")); value != "" {
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
