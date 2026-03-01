package http

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// Latency histogram with finer-grained buckets for ad-tech / sub-100ms SLOs.
	// DefBuckets start at 5ms — too coarse for bid evaluation (target < 10ms).
	httpDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_request_duration_seconds",
		Help:    "Duration of HTTP requests.",
		Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.2, 0.5, 1.0, 2.5, 5.0, 10.0},
	}, []string{"path", "method", "status"})

	httpRequests = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "http_requests_total",
		Help: "Total number of HTTP requests.",
	}, []string{"path", "method", "status"})

	// httpInflight tracks current in-flight requests per route — key metric for latency budgets.
	// Alert when inflight > max_concurrent threshold to detect backpressure events.
	httpInflight = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "http_requests_inflight",
		Help: "Current number of in-flight HTTP requests.",
	}, []string{"path", "method"})

	// concurrencyShed counts requests rejected by ConcurrencyMiddleware (503 responses).
	// Non-zero values indicate the service is running at capacity.
	concurrencyShed = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "http_concurrency_shed_total",
		Help: "Requests dropped by concurrency limiter (503).",
	}, []string{"path"})
)

// MetricsMiddleware records RED metrics + inflight gauge (for latency budget monitoring).
func MetricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

		// Track in-flight using raw path before routing resolves the pattern.
		// This is intentional: inflight gauge is incremented before the handler runs.
		httpInflight.WithLabelValues(r.URL.Path, r.Method).Inc()
		defer httpInflight.WithLabelValues(r.URL.Path, r.Method).Dec()

		next.ServeHTTP(ww, r)

		// After handler: use resolved route pattern for stable cardinality.
		routeCtx := chi.RouteContext(r.Context())
		path := r.URL.Path
		if routeCtx != nil && routeCtx.RoutePattern() != "" {
			path = routeCtx.RoutePattern()
		}

		status := strconv.Itoa(ww.Status())
		httpDuration.WithLabelValues(path, r.Method, status).Observe(time.Since(start).Seconds())
		httpRequests.WithLabelValues(path, r.Method, status).Inc()
	})
}
