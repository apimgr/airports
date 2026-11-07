package db

import (
	"context"
	"errors"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

func TestObserveQueryUsesBoundedLabels(t *testing.T) {
	collector := newQueryMetrics()
	collector.queries.WithLabelValues("select", "scheduler_tasks").Inc()
	collector.duration.WithLabelValues("select", "scheduler_tasks").Observe(0.002)
	collector.errors.WithLabelValues("select", "timeout").Inc()

	registry := prometheus.NewRegistry()
	if err := registry.Register(collector); err != nil {
		t.Fatalf("register metrics: %v", err)
	}
	families, err := registry.Gather()
	if err != nil {
		t.Fatalf("gather metrics: %v", err)
	}
	if len(families) != 3 {
		t.Fatalf("got %d metric families, want 3", len(families))
	}
}

func TestObserveQueryRejectsUnboundedLabels(t *testing.T) {
	for _, tc := range []struct {
		operation string
		table     string
		valid     bool
	}{
		{"select", "scheduler_tasks", true},
		{"create", "scheduler_tasks", false},
		{"select", "scheduler_tasks;secret", false},
	} {
		if got := validOperation(tc.operation) && validTable(tc.table); got != tc.valid {
			t.Errorf("validOperation/table(%q, %q) = %v, want %v", tc.operation, tc.table, got, tc.valid)
		}
	}
	if got := classifyQueryError(context.DeadlineExceeded); got != "timeout" {
		t.Errorf("deadline error type = %q, want timeout", got)
	}
	if got := classifyQueryError(errors.New("UNIQUE constraint failed")); got != "duplicate" {
		t.Errorf("unique error type = %q, want duplicate", got)
	}
}
