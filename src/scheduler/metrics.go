package scheduler

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// PrometheusMetrics contains the scheduler's bounded-cardinality Prometheus collectors.
type PrometheusMetrics struct {
	tasksTotal   *prometheus.CounterVec
	taskDuration *prometheus.HistogramVec
	tasksRunning *prometheus.GaugeVec
	lastRun      *prometheus.GaugeVec
}

// NewPrometheusMetrics creates metrics for the four scheduler metric families.
func NewPrometheusMetrics() *PrometheusMetrics {
	return &PrometheusMetrics{
		tasksTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "airports_scheduler_tasks_total",
			Help: "Total number of scheduled tasks executed, partitioned by task and status.",
		}, []string{"task", "status"}),
		taskDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "airports_scheduler_task_duration_seconds",
			Help:    "Scheduled task duration in seconds.",
			Buckets: []float64{0.1, 0.5, 1, 5, 10, 30, 60, 300, 600},
		}, []string{"task"}),
		tasksRunning: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "airports_scheduler_tasks_running",
			Help: "Number of currently running scheduled tasks.",
		}, []string{"task"}),
		lastRun: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "airports_scheduler_last_run_timestamp",
			Help: "Unix timestamp of the last scheduled task run.",
		}, []string{"task"}),
	}
}

func (m *PrometheusMetrics) Describe(ch chan<- *prometheus.Desc) {
	m.tasksTotal.Describe(ch)
	m.taskDuration.Describe(ch)
	m.tasksRunning.Describe(ch)
	m.lastRun.Describe(ch)
}

func (m *PrometheusMetrics) Collect(ch chan<- prometheus.Metric) {
	m.tasksTotal.Collect(ch)
	m.taskDuration.Collect(ch)
	m.tasksRunning.Collect(ch)
	m.lastRun.Collect(ch)
}

func (m *PrometheusMetrics) recordTaskStart(task string) {
	if m != nil {
		m.tasksRunning.WithLabelValues(task).Inc()
	}
}

func (m *PrometheusMetrics) recordTaskEnd(task string, duration time.Duration, err error) {
	if m == nil {
		return
	}
	m.tasksRunning.WithLabelValues(task).Dec()
	m.taskDuration.WithLabelValues(task).Observe(duration.Seconds())
	m.lastRun.WithLabelValues(task).SetToCurrentTime()
	status := "success"
	if err != nil {
		status = "error"
	}
	m.tasksTotal.WithLabelValues(task, status).Inc()
}
