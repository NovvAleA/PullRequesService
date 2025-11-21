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
	prReviewersAssigned *prometheus.HistogramVec
	teamMembersCount    *prometheus.GaugeVec
	dbQueryDuration     *prometheus.HistogramVec
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

		prReviewersAssigned: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Name:      "pr_reviewers_assigned",
				Help:      "Number of reviewers assigned to PR",
				Buckets:   []float64{0, 1, 2},
			},
			[]string{"team"},
		),

		teamMembersCount: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "team_members_count",
				Help:      "Number of members in teams",
			},
			[]string{"team_name"},
		),

		dbQueryDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Name:      "db_query_duration_seconds",
				Help:      "Database query duration in seconds",
				Buckets:   prometheus.DefBuckets,
			},
			[]string{"operation", "table"},
		),
	}

	// Регистрируем все метрики
	prometheus.MustRegister(
		m.httpRequestsTotal,
		m.httpRequestDuration,
		m.prCreatedTotal,
		m.prMergedTotal,
		m.prReviewersAssigned,
		m.teamMembersCount,
		m.dbQueryDuration,
	)

	return m
}

func (m *Metrics) MetricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(rw, r)

		duration := time.Since(start).Seconds()
		status := strconv.Itoa(rw.statusCode)

		// Безопасно собираем метрики
		if m.httpRequestsTotal != nil {
			m.httpRequestsTotal.WithLabelValues(r.Method, r.URL.Path, status).Inc()
		}
		if m.httpRequestDuration != nil {
			m.httpRequestDuration.WithLabelValues(r.Method, r.URL.Path, status).Observe(duration)
		}

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

// Безопасные методы с проверкой на nil
func (m *Metrics) IncPRCreated() {
	if m.prCreatedTotal != nil {
		m.prCreatedTotal.Inc()
	}
}

func (m *Metrics) IncPRMerged() {
	if m.prMergedTotal != nil {
		m.prMergedTotal.Inc()
	}
}

func (m *Metrics) ObserveReviewersAssigned(team string, reviewers int) {
	if m.prReviewersAssigned != nil {
		m.prReviewersAssigned.WithLabelValues(team).Observe(float64(reviewers))
	}
}

func (m *Metrics) SetTeamMembersCount(teamName string, count int) {
	if m.teamMembersCount != nil {
		m.teamMembersCount.WithLabelValues(teamName).Set(float64(count))
	}
}

func (m *Metrics) ObserveDBQuery(operation, table string, duration time.Duration) {
	if m.dbQueryDuration != nil {
		m.dbQueryDuration.WithLabelValues(operation, table).Observe(duration.Seconds())
	}
}
