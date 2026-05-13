package auth

import "context"

type principalContextKey struct{}

type Principal struct {
	TenantID     string
	ConsumerID   string
	CredentialID string
	Scopes       []string
}

func ContextWithPrincipal(ctx context.Context, principal Principal) context.Context {
	return context.WithValue(ctx, principalContextKey{}, principal)
}

func PrincipalFromContext(ctx context.Context) (Principal, bool) {
	principal, ok := ctx.Value(principalContextKey{}).(Principal)
	return principal, ok
}
