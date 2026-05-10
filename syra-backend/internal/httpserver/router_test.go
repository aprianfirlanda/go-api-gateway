package httpserver

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHealthzReturnsHealthy(t *testing.T) {
	router := NewRouter(Dependencies{
		Logger:        slog.New(slog.NewTextHandler(discardWriter{}, nil)),
		HealthService: stubHealthService{},
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.JSONEq(t, `{"status":"healthy"}`, rec.Body.String())
	require.NotEmpty(t, rec.Header().Get("X-Request-Id"))
}

func TestReadyzReturnsReady(t *testing.T) {
	router := NewRouter(Dependencies{
		Logger:        slog.New(slog.NewTextHandler(discardWriter{}, nil)),
		HealthService: stubHealthService{},
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.JSONEq(t, `{"status":"ready"}`, rec.Body.String())
}

func TestReadyzReturnsUnavailableWhenDependencyFails(t *testing.T) {
	router := NewRouter(Dependencies{
		Logger:        slog.New(slog.NewTextHandler(discardWriter{}, nil)),
		HealthService: stubHealthService{readyErr: errors.New("db down")},
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusServiceUnavailable, rec.Code)
	require.JSONEq(t, `{"status":"not_ready"}`, rec.Body.String())
}

func TestRequestIDMiddlewarePreservesInboundRequestID(t *testing.T) {
	router := NewRouter(Dependencies{
		Logger:        slog.New(slog.NewTextHandler(discardWriter{}, nil)),
		HealthService: stubHealthService{},
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	req.Header.Set("X-Request-Id", "external-request-id")

	router.ServeHTTP(rec, req)

	require.Equal(t, "external-request-id", rec.Header().Get("X-Request-Id"))
}

type stubHealthService struct {
	liveErr  error
	readyErr error
}

func (s stubHealthService) Liveness(context.Context) error {
	return s.liveErr
}

func (s stubHealthService) Readiness(context.Context) error {
	return s.readyErr
}

type discardWriter struct{}

func (discardWriter) Write(p []byte) (int, error) {
	return len(p), nil
}
