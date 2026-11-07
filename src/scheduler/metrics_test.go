package scheduler

import (
	"errors"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

func TestPrometheusMetricsRecordTaskLifecycle(t *testing.T) {
	metrics := NewPrometheusMetrics()
	registry := prometheus.NewRegistry()
	if err := registry.Register(metrics); err != nil {
		t.Fatalf("Register(metrics): %v", err)
	}

	metrics.recordTaskStart("geoip_update")
	metrics.recordTaskEnd("geoip_update", 1500*time.Millisecond, nil)
	metrics.recordTaskStart("geoip_update")
	metrics.recordTaskEnd("geoip_update", 2*time.Second, errors.New("download failed"))

	families, err := registry.Gather()
	if err != nil {
		t.Fatalf("Gather(): %v", err)
	}
	got := make(map[string]*dto.MetricFamily, len(families))
	for _, family := range families {
		got[family.GetName()] = family
	}
	if family := got["airports_scheduler_tasks_total"]; family == nil || len(family.Metric) != 2 {
		t.Fatalf("scheduler task counter = %#v, want success and error series", family)
	}
	if family := got["airports_scheduler_task_duration_seconds"]; family == nil || family.Metric[0].GetHistogram().GetSampleCount() != 2 {
		t.Fatalf("scheduler task duration = %#v, want two observations", family)
	}
	if family := got["airports_scheduler_tasks_running"]; family == nil || family.Metric[0].GetGauge().GetValue() != 0 {
		t.Fatalf("scheduler running gauge = %#v, want zero after task end", family)
	}
	if family := got["airports_scheduler_last_run_timestamp"]; family == nil || family.Metric[0].GetGauge().GetValue() <= 0 {
		t.Fatalf("scheduler last-run gauge = %#v, want a Unix timestamp", family)
	}
}
