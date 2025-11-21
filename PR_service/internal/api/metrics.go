package api

import (
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Metrics struct {
	httpRequestsTotal   *prometheus.CounterVec
	httpRequestDuration *prometheus.HistogramVec
	prCreatedTotal      prometheus.Counter
	prMergedTotal       prometheus.Counter
}

func NewMetrics() *Metrics {
	const namespace = "pr_service"

	m := &Metrics{
		httpRequestsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "http_requests_total",
				Help:      "Total number of HTTP requests",
			},
			[]string{"method", "path", "status"},
		),

		httpRequestDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Name:      "http_request_duration_seconds",
				Help:      "HTTP request duration in seconds",
				Buckets:   prometheus.DefBuckets,
			},
			[]string{"method", "path", "status"},
		),

		prCreatedTotal: prometheus.NewCounter(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "pr_created_total",
				Help:      "Total number of created pull requests",
			},
		),

		prMergedTotal: prometheus.NewCounter(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "pr_merged_total",
				Help:      "Total number of merged pull requests",
			},
		),
	}

	// Регистрируем только основные метрики
	prometheus.MustRegister(m.httpRequestsTotal)
	prometheus.MustRegister(m.httpRequestDuration)
	prometheus.MustRegister(m.prCreatedTotal)
	prometheus.MustRegister(m.prMergedTotal)

	return m
}

func (m *Metrics) MetricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(rw, r)

		duration := time.Since(start).Seconds()
		status := strconv.Itoa(rw.statusCode)

		// Только базовые метрики
		m.httpRequestsTotal.WithLabelValues(r.Method, r.URL.Path, status).Inc()
		m.httpRequestDuration.WithLabelValues(r.Method, r.URL.Path, status).Observe(duration)

		log.Printf("METRIC: %s %s %s - %.3fs", r.Method, r.URL.Path, status, duration)
	})
}

type responseWriter struct {
	http.ResponseWriter
	statusCode int
	size       int
}

func (rw *responseWriter) WriteHeader(statusCode int) {
	rw.statusCode = statusCode
	rw.ResponseWriter.WriteHeader(statusCode)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	size, err := rw.ResponseWriter.Write(b)
	rw.size += size
	return size, err
}

func (m *Metrics) InstrumentedHandler() http.Handler {
	return promhttp.Handler()
}

func (m *Metrics) IncPRCreated() {
	m.prCreatedTotal.Inc()
}

func (m *Metrics) IncPRMerged() {
	m.prMergedTotal.Inc()
}

// Упрощенные методы - убрали сложные метрики
func (m *Metrics) ObserveReviewersAssigned(team string, reviewers int) {
	// Пока ничего не делаем
}

func (m *Metrics) SetTeamMembersCount(teamName string, count int) {
	// Пока ничего не делаем
}

func (m *Metrics) ObserveDBQuery(operation, table string, duration time.Duration) {
	// Пока ничего не делаем
}

func (m *Metrics) SetDBConnections(count int) {
	// Пока ничего не делаем
}
