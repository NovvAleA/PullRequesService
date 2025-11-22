package api

import (
	"log"
	"net/http"
	"runtime"
	"sort"
	"strconv"
	"sync"
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
	businessErrors      *prometheus.CounterVec
	mu                  sync.RWMutex
}

// Глобальная переменная для времени старта
var appStartTime = time.Now()

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
				Buckets:   []float64{0.01, 0.05, 0.1, 0.2, 0.3, 0.5, 1.0},
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
				Name:      "pr_reviewers_assigned_count",
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

		businessErrors: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "business_errors_total",
				Help:      "Business logic errors by type",
			},
			[]string{"error_type"},
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
		m.businessErrors,
	)

	return m
}

// Thread-safe методы
func (m *Metrics) IncPRCreated() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.prCreatedTotal.Inc()
}

func (m *Metrics) IncPRMerged() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.prMergedTotal.Inc()
}

func (m *Metrics) ObserveReviewersAssigned(team string, reviewers int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.prReviewersAssigned.WithLabelValues(team).Observe(float64(reviewers))
}

func (m *Metrics) SetTeamMembersCount(teamName string, count int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.teamMembersCount.WithLabelValues(teamName).Set(float64(count))
}

func (m *Metrics) ObserveDBQuery(operation, table string, duration time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.dbQueryDuration.WithLabelValues(operation, table).Observe(duration.Seconds())
}

func (m *Metrics) IncBusinessError(errorType string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.businessErrors.WithLabelValues(errorType).Inc()
}

// Метод для middleware - должен быть безопасным
func (m *Metrics) RecordHTTPRequest(method, path, status string, duration time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.httpRequestsTotal.WithLabelValues(method, path, status).Inc()
	m.httpRequestDuration.WithLabelValues(method, path, status).Observe(duration.Seconds())
}

func (m *Metrics) MetricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(rw, r)

		duration := time.Since(start)
		status := strconv.Itoa(rw.statusCode)

		// Используем thread-safe метод
		m.RecordHTTPRequest(r.Method, r.URL.Path, status, duration)

		log.Printf("METRIC: %s %s %s - %.3fs", r.Method, r.URL.Path, status, duration.Seconds())
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

// MetricsData возвращает детальные метрики по всем хендлерам
func (h *Handler) MetricsData(w http.ResponseWriter, r *http.Request) {
	type HandlerMetric struct {
		Handler       string  `json:"handler"`
		Method        string  `json:"method"`
		TotalRequests float64 `json:"total_requests"`
		SuccessCount  float64 `json:"success_count"`
		ErrorCount    float64 `json:"error_count"`
		SuccessRate   float64 `json:"success_rate"`
		AvgDurationMs float64 `json:"avg_duration_ms"`
		P95DurationMs float64 `json:"p95_duration_ms"`
		LastMinuteRPS float64 `json:"last_minute_rps"`
	}

	type BusinessMetric struct {
		ErrorType string  `json:"error_type"`
		Count     float64 `json:"count"`
	}

	type MetricsResponse struct {
		Timestamp      time.Time        `json:"timestamp"`
		UptimeSeconds  float64          `json:"uptime_seconds"`
		Goroutines     int              `json:"goroutines"`
		Handlers       []HandlerMetric  `json:"handlers"`
		BusinessErrors []BusinessMetric `json:"business_errors"`
		Totals         struct {
			TotalRequests  float64 `json:"total_requests"`
			TotalPRCreated float64 `json:"total_pr_created"`
			TotalPRMerged  float64 `json:"total_pr_merged"`
		} `json:"totals"`
	}

	// Собираем метрики из Prometheus
	metrics, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	handlerStats := make(map[string]*HandlerMetric)
	businessErrors := make(map[string]float64)
	var totalPRCreated, totalPRMerged float64

	// Сначала собираем все HTTP запросы
	for _, metric := range metrics {
		name := metric.GetName()

		// HTTP requests - счетчики запросов
		if name == "pr_service_http_requests_total" {
			for _, m := range metric.GetMetric() {
				var path, method, status string
				for _, label := range m.GetLabel() {
					switch label.GetName() {
					case "path":
						path = label.GetValue()
					case "method":
						method = label.GetValue()
					case "status":
						status = label.GetValue()
					}
				}

				if path != "" && method != "" {
					key := method + ":" + path
					if handlerStats[key] == nil {
						handlerStats[key] = &HandlerMetric{
							Handler: path,
							Method:  method,
						}
					}

					value := m.GetCounter().GetValue()
					handlerStats[key].TotalRequests += value

					if status == "200" || status == "201" {
						handlerStats[key].SuccessCount += value
					} else {
						handlerStats[key].ErrorCount += value
					}
				}
			}
		}
	}

	// Затем собираем длительности
	for _, metric := range metrics {
		name := metric.GetName()

		// HTTP durations - длительности запросов
		if name == "pr_service_http_request_duration_seconds" {
			for _, m := range metric.GetMetric() {
				var path, method, status string
				for _, label := range m.GetLabel() {
					switch label.GetName() {
					case "path":
						path = label.GetValue()
					case "method":
						method = label.GetValue()
					case "status":
						status = label.GetValue()
					}
				}

				if path != "" && method != "" {
					key := method + ":" + path
					if handlerStats[key] != nil {
						hist := m.GetHistogram()
						if hist != nil {
							sampleCount := hist.GetSampleCount()
							sampleSum := hist.GetSampleSum()

							// Для успешных запросов вычисляем среднее время
							if (status == "200" || status == "201") && sampleCount > 0 {
								// Среднее время в миллисекундах
								avgDuration := (sampleSum / float64(sampleCount)) * 1000
								handlerStats[key].AvgDurationMs = avgDuration

								// P95 время (упрощенный расчет)
								buckets := hist.GetBucket()
								if len(buckets) > 0 {
									var totalCount uint64
									targetCount := uint64(float64(sampleCount) * 0.95)

									for _, bucket := range buckets {
										totalCount += bucket.GetCumulativeCount()
										if totalCount >= targetCount {
											handlerStats[key].P95DurationMs = bucket.GetUpperBound() * 1000
											break
										}
									}
								}

								// Логируем для отладки
								log.Printf("DURATION: %s %s - count: %d, sum: %.6f, avg: %.2fms",
									method, path, sampleCount, sampleSum, avgDuration)
							}
						}
					}
				}
			}
		}

		// Business errors
		if name == "pr_service_business_errors_total" {
			for _, m := range metric.GetMetric() {
				var errorType string
				for _, label := range m.GetLabel() {
					if label.GetName() == "error_type" {
						errorType = label.GetValue()
						break
					}
				}
				if errorType != "" {
					businessErrors[errorType] += m.GetCounter().GetValue()
				}
			}
		}

		// PR created
		if name == "pr_service_pr_created_total" {
			for _, m := range metric.GetMetric() {
				totalPRCreated += m.GetCounter().GetValue()
			}
		}

		// PR merged
		if name == "pr_service_pr_merged_total" {
			for _, m := range metric.GetMetric() {
				totalPRMerged += m.GetCounter().GetValue()
			}
		}
	}

	// Рассчитываем success rate и RPS
	var totalRequests float64
	uptime := time.Since(appStartTime).Minutes()

	for _, stat := range handlerStats {
		if stat.TotalRequests > 0 {
			stat.SuccessRate = (stat.SuccessCount / stat.TotalRequests) * 100
			// RPS за все время работы (requests per second)
			if uptime > 0 {
				stat.LastMinuteRPS = stat.TotalRequests / (uptime * 60)
			}
		}
		totalRequests += stat.TotalRequests
	}

	// Преобразуем в слайсы
	handlers := make([]HandlerMetric, 0, len(handlerStats))
	for _, stat := range handlerStats {
		handlers = append(handlers, *stat)
	}

	businessErrorsSlice := make([]BusinessMetric, 0, len(businessErrors))
	for errorType, count := range businessErrors {
		businessErrorsSlice = append(businessErrorsSlice, BusinessMetric{
			ErrorType: errorType,
			Count:     count,
		})
	}

	// Сортируем
	sort.Slice(handlers, func(i, j int) bool {
		return handlers[i].TotalRequests > handlers[j].TotalRequests
	})

	sort.Slice(businessErrorsSlice, func(i, j int) bool {
		return businessErrorsSlice[i].Count > businessErrorsSlice[j].Count
	})

	// Формируем ответ
	response := MetricsResponse{
		Timestamp:      time.Now().UTC(),
		UptimeSeconds:  time.Since(appStartTime).Seconds(),
		Goroutines:     runtime.NumGoroutine(),
		Handlers:       handlers,
		BusinessErrors: businessErrorsSlice,
	}

	response.Totals.TotalRequests = totalRequests
	response.Totals.TotalPRCreated = totalPRCreated
	response.Totals.TotalPRMerged = totalPRMerged

	WriteJSON(w, http.StatusOK, response)
}
