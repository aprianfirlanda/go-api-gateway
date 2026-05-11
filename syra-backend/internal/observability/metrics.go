package observability

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
)

const namespace = "syra_gateway"

type Metrics struct {
	registry              *prometheus.Registry
	requests              *prometheus.CounterVec
	requestLatency        *prometheus.HistogramVec
	authFailures          *prometheus.CounterVec
	rateLimitRejects      *prometheus.CounterVec
	quotaRejects          *prometheus.CounterVec
	protocolAdapterErrors *prometheus.CounterVec
	iso8583Timeouts       *prometheus.CounterVec
	billingEventFailures  *prometheus.CounterVec
}

func NewMetrics() *Metrics {
	m := &Metrics{
		registry: prometheus.NewRegistry(),
		requests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "requests_total",
			Help:      "Total HTTP requests handled by the gateway.",
		}, []string{"method", "path", "status"}),
		requestLatency: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "request_latency_seconds",
			Help:      "HTTP request latency in seconds.",
			Buckets:   prometheus.DefBuckets,
		}, []string{"method", "path", "status"}),
		authFailures:          counter("auth_failures_total", "Authentication failures.", "reason"),
		rateLimitRejects:      counter("rate_limit_rejects_total", "Rate limit policy rejects.", "tenant_id", "route_id"),
		quotaRejects:          counter("quota_rejects_total", "Quota policy rejects.", "tenant_id", "route_id"),
		protocolAdapterErrors: counter("protocol_adapter_errors_total", "Protocol adapter errors.", "protocol", "stage"),
		iso8583Timeouts:       counter("iso8583_timeouts_total", "ISO8583 timeout errors.", "tenant_id", "route_id"),
		billingEventFailures:  counter("billing_event_failures_total", "Billing usage event write failures.", "tenant_id", "route_id"),
	}
	m.registry.MustRegister(m.requests, m.requestLatency, m.authFailures, m.rateLimitRejects, m.quotaRejects, m.protocolAdapterErrors, m.iso8583Timeouts, m.billingEventFailures)
	return m
}

func counter(name, help string, labels ...string) *prometheus.CounterVec {
	return prometheus.NewCounterVec(prometheus.CounterOpts{Namespace: namespace, Name: name, Help: help}, labels)
}

func (m *Metrics) Handler() http.Handler {
	if m == nil {
		return promhttp.Handler()
	}
	return promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{})
}

func (m *Metrics) Middleware(next http.Handler) http.Handler {
	if m == nil {
		return next
	}
	tracer := otel.Tracer("syra-backend/gateway")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, span := tracer.Start(r.Context(), "http.request")
		defer span.End()

		start := time.Now()
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		next.ServeHTTP(ww, r.WithContext(ctx))

		status := strconv.Itoa(ww.Status())
		m.requests.WithLabelValues(r.Method, r.URL.Path, status).Inc()
		m.requestLatency.WithLabelValues(r.Method, r.URL.Path, status).Observe(time.Since(start).Seconds())
		span.SetAttributes(
			attribute.String("http.method", r.Method),
			attribute.String("url.path", r.URL.Path),
			attribute.Int("http.status_code", ww.Status()),
		)
	})
}

func (m *Metrics) IncAuthFailure(reason string) {
	if m != nil {
		m.authFailures.WithLabelValues(reason).Inc()
	}
}

func (m *Metrics) IncRateLimitReject(tenantID, routeID string) {
	if m != nil {
		m.rateLimitRejects.WithLabelValues(tenantID, routeID).Inc()
	}
}

func (m *Metrics) IncQuotaReject(tenantID, routeID string) {
	if m != nil {
		m.quotaRejects.WithLabelValues(tenantID, routeID).Inc()
	}
}

func (m *Metrics) IncProtocolAdapterError(protocol, stage string) {
	if m != nil {
		m.protocolAdapterErrors.WithLabelValues(protocol, stage).Inc()
	}
}

func (m *Metrics) IncISO8583Timeout(tenantID, routeID string) {
	if m != nil {
		m.iso8583Timeouts.WithLabelValues(tenantID, routeID).Inc()
	}
}

func (m *Metrics) IncBillingEventFailure(tenantID, routeID string) {
	if m != nil {
		m.billingEventFailures.WithLabelValues(tenantID, routeID).Inc()
	}
}
