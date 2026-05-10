package auth

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHashAndVerifySecret(t *testing.T) {
	hash, err := HashSecretWithParams("secret", testHashParams())
	require.NoError(t, err)

	ok, err := VerifySecret("secret", hash)
	require.NoError(t, err)
	require.True(t, ok)

	ok, err = VerifySecret("wrong", hash)
	require.NoError(t, err)
	require.False(t, ok)
}

func TestAuthenticateAPIKeyResolvesPrincipal(t *testing.T) {
	hash, err := HashSecretWithParams("secret", testHashParams())
	require.NoError(t, err)

	store := NewInMemoryCredentialStore(APIKeyCredential{
		ID:         "cred_1",
		TenantID:   "tenant_1",
		ConsumerID: "consumer_1",
		KeyPrefix:  "gw_live_abc",
		SecretHash: hash,
		Status:     StatusActive,
	})

	principal, err := AuthenticateAPIKey(context.Background(), store, "gw_live_abc.secret")

	require.NoError(t, err)
	require.Equal(t, "tenant_1", principal.TenantID)
	require.Equal(t, "consumer_1", principal.ConsumerID)
	require.Equal(t, "cred_1", principal.CredentialID)
}

func TestAuthenticateAPIKeyRejectsSuspendedCredential(t *testing.T) {
	hash, err := HashSecretWithParams("secret", testHashParams())
	require.NoError(t, err)

	store := NewInMemoryCredentialStore(APIKeyCredential{
		ID:         "cred_1",
		TenantID:   "tenant_1",
		ConsumerID: "consumer_1",
		KeyPrefix:  "gw_live_abc",
		SecretHash: hash,
		Status:     StatusSuspended,
	})

	_, err = AuthenticateAPIKey(context.Background(), store, "gw_live_abc.secret")

	require.ErrorIs(t, err, ErrCredentialForbidden)
}

func testHashParams() HashParams {
	return HashParams{
		Memory:      32,
		Iterations:  1,
		Parallelism: 1,
		SaltLength:  8,
		KeyLength:   16,
	}
}
