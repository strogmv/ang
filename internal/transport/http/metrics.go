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
	httpDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_request_duration_seconds",
		Help:    "Duration of HTTP requests.",
		Buckets: prometheus.DefBuckets,
	}, []string{"path", "method", "status"})

	httpRequests = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "http_requests_total",
		Help: "Total number of HTTP requests.",
	}, []string{"path", "method", "status"})
)

// MetricsMiddleware records RED metrics
func MetricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

		next.ServeHTTP(ww, r)

		// Try to get route pattern (e.g. /users/{id}) instead of raw path
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
