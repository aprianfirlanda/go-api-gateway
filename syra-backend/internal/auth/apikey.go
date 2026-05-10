package auth

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"golang.org/x/crypto/argon2"
)

const (
	StatusActive    = "active"
	StatusSuspended = "suspended"
	StatusRevoked   = "revoked"
)

var (
	ErrInvalidAPIKey       = errors.New("invalid api key")
	ErrCredentialNotFound  = errors.New("api key credential not found")
	ErrCredentialForbidden = errors.New("api key credential forbidden")
)

type APIKeyCredential struct {
	ID         string
	TenantID   string
	ConsumerID string
	KeyPrefix  string
	SecretHash string
	Status     string
}

type CredentialStore interface {
	FindByPrefix(ctx context.Context, prefix string) (APIKeyCredential, error)
}

type InMemoryCredentialStore struct {
	mu          sync.RWMutex
	credentials map[string]APIKeyCredential
}

func NewInMemoryCredentialStore(credentials ...APIKeyCredential) *InMemoryCredentialStore {
	store := &InMemoryCredentialStore{credentials: map[string]APIKeyCredential{}}
	for _, credential := range credentials {
		store.credentials[credential.KeyPrefix] = credential
	}
	return store
}

func (s *InMemoryCredentialStore) FindByPrefix(ctx context.Context, prefix string) (APIKeyCredential, error) {
	if err := ctx.Err(); err != nil {
		return APIKeyCredential{}, err
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	credential, ok := s.credentials[prefix]
	if !ok {
		return APIKeyCredential{}, ErrCredentialNotFound
	}
	return credential, nil
}

type HashParams struct {
	Memory      uint32
	Iterations  uint32
	Parallelism uint8
	SaltLength  uint32
	KeyLength   uint32
}

func DefaultHashParams() HashParams {
	return HashParams{
		Memory:      64 * 1024,
		Iterations:  3,
		Parallelism: 1,
		SaltLength:  16,
		KeyLength:   32,
	}
}

func HashSecret(secret string) (string, error) {
	return HashSecretWithParams(secret, DefaultHashParams())
}

func HashSecretWithParams(secret string, params HashParams) (string, error) {
	if secret == "" {
		return "", ErrInvalidAPIKey
	}

	salt := make([]byte, params.SaltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate salt: %w", err)
	}

	hash := argon2.IDKey([]byte(secret), salt, params.Iterations, params.Memory, params.Parallelism, params.KeyLength)
	return encodeHash(params, salt, hash), nil
}

func VerifySecret(secret, encodedHash string) (bool, error) {
	params, salt, expectedHash, err := decodeHash(encodedHash)
	if err != nil {
		return false, err
	}

	actualHash := argon2.IDKey([]byte(secret), salt, params.Iterations, params.Memory, params.Parallelism, params.KeyLength)
	if subtle.ConstantTimeCompare(actualHash, expectedHash) != 1 {
		return false, nil
	}
	return true, nil
}

func ParseAPIKey(value string) (prefix string, secret string, err error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", "", ErrInvalidAPIKey
	}

	prefix, secret, ok := strings.Cut(value, ".")
	if !ok || prefix == "" || secret == "" {
		return "", "", ErrInvalidAPIKey
	}

	return prefix, secret, nil
}

func AuthenticateAPIKey(ctx context.Context, store CredentialStore, rawKey string) (Principal, error) {
	prefix, secret, err := ParseAPIKey(rawKey)
	if err != nil {
		return Principal{}, err
	}

	credential, err := store.FindByPrefix(ctx, prefix)
	if err != nil {
		return Principal{}, err
	}

	switch credential.Status {
	case StatusActive:
	case StatusSuspended, StatusRevoked:
		return Principal{}, ErrCredentialForbidden
	default:
		return Principal{}, ErrCredentialForbidden
	}

	ok, err := VerifySecret(secret, credential.SecretHash)
	if err != nil {
		return Principal{}, err
	}
	if !ok {
		return Principal{}, ErrInvalidAPIKey
	}

	return Principal{
		TenantID:     credential.TenantID,
		ConsumerID:   credential.ConsumerID,
		CredentialID: credential.ID,
	}, nil
}

func encodeHash(params HashParams, salt, hash []byte) string {
	b64Salt := base64.RawStdEncoding.EncodeToString(salt)
	b64Hash := base64.RawStdEncoding.EncodeToString(hash)
	return fmt.Sprintf(
		"$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s",
		params.Memory,
		params.Iterations,
		params.Parallelism,
		b64Salt,
		b64Hash,
	)
}

func decodeHash(encodedHash string) (HashParams, []byte, []byte, error) {
	parts := strings.Split(encodedHash, "$")
	if len(parts) != 6 || parts[1] != "argon2id" || parts[2] != "v=19" {
		return HashParams{}, nil, nil, fmt.Errorf("invalid argon2 hash")
	}

	params, err := decodeParams(parts[3])
	if err != nil {
		return HashParams{}, nil, nil, err
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return HashParams{}, nil, nil, fmt.Errorf("decode salt: %w", err)
	}

	hash, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return HashParams{}, nil, nil, fmt.Errorf("decode hash: %w", err)
	}

	params.SaltLength = uint32(len(salt))
	params.KeyLength = uint32(len(hash))

	return params, salt, hash, nil
}

func decodeParams(value string) (HashParams, error) {
	var params HashParams
	for _, part := range strings.Split(value, ",") {
		key, rawValue, ok := strings.Cut(part, "=")
		if !ok {
			return HashParams{}, fmt.Errorf("invalid argon2 params")
		}

		parsed, err := strconv.ParseUint(rawValue, 10, 32)
		if err != nil {
			return HashParams{}, fmt.Errorf("parse argon2 param %s: %w", key, err)
		}

		switch key {
		case "m":
			params.Memory = uint32(parsed)
		case "t":
			params.Iterations = uint32(parsed)
		case "p":
			params.Parallelism = uint8(parsed)
		default:
			return HashParams{}, fmt.Errorf("unknown argon2 param %s", key)
		}
	}

	if params.Memory == 0 || params.Iterations == 0 || params.Parallelism == 0 {
		return HashParams{}, fmt.Errorf("missing argon2 params")
	}

	return params, nil
}
