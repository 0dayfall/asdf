package monitoring

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
)

var (
	// HTTP metrics
	httpRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "endpoint", "status_code"},
	)

	httpRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "Duration of HTTP requests",
			Buckets: []float64{0.001, 0.01, 0.1, 0.5, 1.0, 2.5, 5.0, 10.0},
		},
		[]string{"method", "endpoint", "status_code"},
	)

	// WebFinger specific metrics
	webfingerRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "webfinger_requests_total",
			Help: "Total number of WebFinger requests",
		},
		[]string{"status"},
	)

	webfingerCacheHits = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "webfinger_cache_hits_total",
			Help: "Total number of WebFinger cache hits",
		},
	)

	webfingerCacheMisses = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "webfinger_cache_misses_total",
			Help: "Total number of WebFinger cache misses",
		},
	)

	// Database metrics
	databaseConnectionsActive = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "database_connections_active",
			Help: "Number of active database connections",
		},
	)

	databaseConnectionsIdle = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "database_connections_idle",
			Help: "Number of idle database connections",
		},
	)

	databaseQueryDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "database_query_duration_seconds",
			Help:    "Duration of database queries",
			Buckets: []float64{0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1.0},
		},
		[]string{"operation"},
	)

	// Authentication metrics
	authAttemptsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "auth_attempts_total",
			Help: "Total number of authentication attempts",
		},
		[]string{"result"},
	)

	activeSessionsGauge = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "active_sessions_total",
			Help: "Number of active user sessions",
		},
	)
)

// Metrics holds all the monitoring components
type Metrics struct {
	registry *prometheus.Registry
	logger   *logrus.Logger
}

// NewMetrics creates a new metrics instance
func NewMetrics() *Metrics {
	// Create custom registry
	registry := prometheus.NewRegistry()

	// Register metrics
	registry.MustRegister(
		httpRequestsTotal,
		httpRequestDuration,
		webfingerRequestsTotal,
		webfingerCacheHits,
		webfingerCacheMisses,
		databaseConnectionsActive,
		databaseConnectionsIdle,
		databaseQueryDuration,
		authAttemptsTotal,
		activeSessionsGauge,
	)

	// Setup structured logging
	logger := logrus.New()
	logger.SetFormatter(&logrus.JSONFormatter{
		TimestampFormat: time.RFC3339,
	})

	return &Metrics{
		registry: registry,
		logger:   logger,
	}
}

// HTTPMetricsMiddleware adds Prometheus metrics to HTTP requests
func (m *Metrics) HTTPMetricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Wrap ResponseWriter to capture status code
		ww := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		// Process request
		next.ServeHTTP(ww, r)

		// Record metrics
		duration := time.Since(start).Seconds()
		statusCode := strconv.Itoa(ww.statusCode)

		httpRequestsTotal.WithLabelValues(r.Method, r.URL.Path, statusCode).Inc()
		httpRequestDuration.WithLabelValues(r.Method, r.URL.Path, statusCode).Observe(duration)

		// Log request
		m.logger.WithFields(logrus.Fields{
			"method":     r.Method,
			"path":       r.URL.Path,
			"status":     ww.statusCode,
			"duration":   duration,
			"ip":         getClientIP(r),
			"user_agent": r.Header.Get("User-Agent"),
		}).Info("HTTP request")
	})
}

// RecordWebFingerRequest records a WebFinger request
func (m *Metrics) RecordWebFingerRequest(status string) {
	webfingerRequestsTotal.WithLabelValues(status).Inc()
}

// RecordCacheHit records a cache hit
func (m *Metrics) RecordCacheHit() {
	webfingerCacheHits.Inc()
}

// RecordCacheMiss records a cache miss
func (m *Metrics) RecordCacheMiss() {
	webfingerCacheMisses.Inc()
}

// RecordDatabaseQuery records a database query duration
func (m *Metrics) RecordDatabaseQuery(operation string, duration time.Duration) {
	databaseQueryDuration.WithLabelValues(operation).Observe(duration.Seconds())
}

// UpdateDatabaseConnections updates database connection metrics
func (m *Metrics) UpdateDatabaseConnections(active, idle int) {
	databaseConnectionsActive.Set(float64(active))
	databaseConnectionsIdle.Set(float64(idle))
}

// RecordAuthAttempt records an authentication attempt
func (m *Metrics) RecordAuthAttempt(result string) {
	authAttemptsTotal.WithLabelValues(result).Inc()
}

// UpdateActiveSessions updates the active sessions gauge
func (m *Metrics) UpdateActiveSessions(count int) {
	activeSessionsGauge.Set(float64(count))
}

// Handler returns the Prometheus metrics handler
func (m *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{
		EnableOpenMetrics: true,
	})
}

// Logger returns the structured logger
func (m *Metrics) Logger() *logrus.Logger {
	return m.logger
}

// HealthCheckHandler provides a health check endpoint
func (m *Metrics) HealthCheckHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		// Simple JSON encoding without importing encoding/json again
		response := `{"status":"healthy","timestamp":` + strconv.FormatInt(time.Now().Unix(), 10) + `,"version":"1.0.0"}`
		w.Write([]byte(response))

		m.logger.WithField("component", "healthcheck").Debug("Health check requested")
	}
} // DatabaseMetricsMiddleware wraps database operations with metrics
func (m *Metrics) DatabaseMetricsMiddleware(operation string) func(func() error) error {
	return func(dbFunc func() error) error {
		start := time.Now()
		err := dbFunc()
		duration := time.Since(start)

		m.RecordDatabaseQuery(operation, duration)

		if err != nil {
			m.logger.WithFields(logrus.Fields{
				"operation": operation,
				"duration":  duration,
				"error":     err.Error(),
			}).Error("Database operation failed")
		} else {
			m.logger.WithFields(logrus.Fields{
				"operation": operation,
				"duration":  duration,
			}).Debug("Database operation completed")
		}

		return err
	}
}

// LogLevel sets the logging level
func (m *Metrics) SetLogLevel(level string) error {
	parsedLevel, err := logrus.ParseLevel(level)
	if err != nil {
		return err
	}
	m.logger.SetLevel(parsedLevel)
	return nil
}

// LogWithFields creates a logger with predefined fields
func (m *Metrics) LogWithFields(fields logrus.Fields) *logrus.Entry {
	return m.logger.WithFields(fields)
}

// responseWriter wraps http.ResponseWriter to capture status code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// getClientIP extracts client IP from request
func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		return xff
	}

	// Check X-Real-IP header
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}

	// Fall back to RemoteAddr
	return r.RemoteAddr
}

// StartBackgroundTasks starts background monitoring tasks
func (m *Metrics) StartBackgroundTasks(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				// Perform periodic tasks like updating connection metrics
				m.logger.WithField("component", "monitoring").Debug("Periodic metrics update")
			}
		}
	}()
}
