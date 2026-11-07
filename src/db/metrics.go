package db

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/apimgr/airports/src/common/diagnostics"
	"github.com/prometheus/client_golang/prometheus"
)

var queryMetrics = newQueryMetrics()

// SetQueryLoggingEnabled updates the configuration gate for SQL diagnostics.
func SetQueryLoggingEnabled(enabled bool) {
	diagnostics.SetQueryLoggingEnabled(enabled)
}

// LogQuery records a bounded SQL diagnostic without emitting bound values.
func LogQuery(query string, argumentCount int, started time.Time, rows int64, err error) {
	diagnostics.LogQuery(query, argumentCount, started, rows, err)
}

func logExecQuery(query string, argumentCount int, started time.Time, result sql.Result, err error) {
	rows := int64(0)
	if result != nil {
		if affected, resultErr := result.RowsAffected(); resultErr == nil {
			rows = affected
		}
	}
	LogQuery(query, argumentCount, started, rows, err)
}

type queryMetricsCollector struct {
	queries  *prometheus.CounterVec
	duration *prometheus.HistogramVec
	errors   *prometheus.CounterVec
}

func (m *queryMetricsCollector) Describe(ch chan<- *prometheus.Desc) {
	m.queries.Describe(ch)
	m.duration.Describe(ch)
	m.errors.Describe(ch)
}

func (m *queryMetricsCollector) Collect(ch chan<- prometheus.Metric) {
	m.queries.Collect(ch)
	m.duration.Collect(ch)
	m.errors.Collect(ch)
}

func newQueryMetrics() *queryMetricsCollector {
	return &queryMetricsCollector{
		queries: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "airports_db_queries_total",
			Help: "Total database queries partitioned by operation and table.",
		}, []string{"operation", "table"}),
		duration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "airports_db_query_duration_seconds",
			Help:    "Database query latency in seconds.",
			Buckets: []float64{0.0001, 0.0005, 0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1},
		}, []string{"operation", "table"}),
		errors: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "airports_db_errors_total",
			Help: "Total database errors partitioned by operation and bounded error type.",
		}, []string{"operation", "error_type"}),
	}
}

// QueryMetricsCollector exposes the shared database query metrics for the
// server registry. Query labels are supplied from fixed call-site constants.
func QueryMetricsCollector() prometheus.Collector {
	return queryMetrics
}

// ObserveQuery records one shared-database query. The strict allowlists keep
// metric cardinality bounded even if a caller passes dynamic input.
func ObserveQuery(operation, table string, started time.Time, err error) {
	if !validOperation(operation) || !validTable(table) {
		return
	}
	queryMetrics.queries.WithLabelValues(operation, table).Inc()
	queryMetrics.duration.WithLabelValues(operation, table).Observe(time.Since(started).Seconds())
	if err != nil {
		queryMetrics.errors.WithLabelValues(operation, classifyQueryError(err)).Inc()
	}
}

func validOperation(operation string) bool {
	switch operation {
	case "select", "insert", "update", "delete":
		return true
	default:
		return false
	}
}

func validTable(table string) bool {
	switch table {
	case "scheduler_tasks", "scheduler_history", "rate_limits":
		return true
	default:
		return false
	}
}

func classifyQueryError(err error) string {
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return "timeout"
	}
	message := strings.ToLower(err.Error())
	if strings.Contains(message, "unique constraint") || strings.Contains(message, "primary key") || strings.Contains(message, "duplicate") {
		return "duplicate"
	}
	if strings.Contains(message, "constraint") || strings.Contains(message, "foreign key") || strings.Contains(message, "not null") {
		return "constraint"
	}
	if strings.Contains(message, "connection") || strings.Contains(message, "database is closed") || strings.Contains(message, "disk i/o") {
		return "connection"
	}
	return "other"
}
