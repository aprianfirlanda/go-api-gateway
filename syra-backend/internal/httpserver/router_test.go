package httpserver

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthzReturnsHealthy(t *testing.T) {
	router := NewRouter(Dependencies{
		Logger:        slog.New(slog.NewTextHandler(discardWriter{}, nil)),
		HealthService: stubHealthService{},
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusOK)
	}
	if got := rec.Body.String(); got != "{\"status\":\"healthy\"}\n" {
		t.Fatalf("body = %q, want healthy json", got)
	}
	if rec.Header().Get("X-Request-Id") == "" {
		t.Fatal("expected request id header")
	}
}

func TestReadyzReturnsUnavailableWhenDependencyFails(t *testing.T) {
	router := NewRouter(Dependencies{
		Logger:        slog.New(slog.NewTextHandler(discardWriter{}, nil)),
		HealthService: stubHealthService{readyErr: errors.New("db down")},
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
	if got := rec.Body.String(); got != "{\"status\":\"not_ready\"}\n" {
		t.Fatalf("body = %q, want not_ready json", got)
	}
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
