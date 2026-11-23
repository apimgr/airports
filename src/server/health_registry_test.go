package server

import (
	"encoding/json"
	"strings"
	"testing"
)

func isolateHealthRegistry(t *testing.T) {
	t.Helper()
	datasetHealthRegistry.Lock()
	previous := append([]*healthCheckRegistration(nil), datasetHealthRegistry.checks...)
	datasetHealthRegistry.checks = nil
	datasetHealthRegistry.Unlock()
	t.Cleanup(func() {
		datasetHealthRegistry.Lock()
		datasetHealthRegistry.checks = previous
		datasetHealthRegistry.Unlock()
	})
}

func TestRegisterHealthCheckCachesSafeOrderedResults(t *testing.T) {
	isolateHealthRegistry(t)
	if err := RegisterHealthCheck("data_airports", true, func() string { return "ok" }); err != nil {
		t.Fatal(err)
	}
	if err := RegisterHealthCheck("geoip_country", false, func() string { return "untrusted detail" }); err != nil {
		t.Fatal(err)
	}
	checks := currentDatasetHealthChecks()
	if len(checks) != 2 || checks[0].Key != "data_airports" || checks[1].Key != "geoip_country" {
		t.Fatalf("registered checks = %+v, want registration order", checks)
	}
	if checks[0].Status != "ok" || checks[1].Status != "error" {
		t.Fatalf("cached statuses = %+v, want ok and safe error", checks)
	}
	if !checks[0].Critical || checks[1].Critical {
		t.Fatalf("critical flags = %+v, want true then false", checks)
	}
}

func TestChecksInfoJSONPreservesDatasetOrderAndPublicValues(t *testing.T) {
	checks := ChecksInfo{
		Database: "ok", Cache: "ok", Disk: "ok", Scheduler: "ok", Tor: "degraded", I2P: "degraded",
		Datasets: []DatasetHealthCheck{{Key: "data_airports", Status: "ok"}, {Key: "geoip_country", Status: "degraded"}},
	}
	encoded, err := json.Marshal(checks)
	if err != nil {
		t.Fatal(err)
	}
	body := string(encoded)
	orderedKeys := []string{`"database"`, `"cache"`, `"disk"`, `"scheduler"`, `"tor"`, `"i2p"`, `"data_airports"`, `"geoip_country"`}
	previous := -1
	for _, key := range orderedKeys {
		position := strings.Index(body, key)
		if position <= previous {
			t.Fatalf("JSON checks order is wrong at %s: %s", key, body)
		}
		previous = position
	}
	if strings.Contains(body, "critical") || strings.Contains(body, "path") || strings.Contains(body, "error text") {
		t.Fatalf("internal health metadata leaked in JSON: %s", body)
	}
}

func TestRemoveHealthCheckOmitsDisabledDataset(t *testing.T) {
	isolateHealthRegistry(t)
	if err := RegisterHealthCheck("geoip_country", false, func() string { return "ok" }); err != nil {
		t.Fatal(err)
	}
	RemoveHealthCheck("geoip_country")
	if checks := currentDatasetHealthChecks(); len(checks) != 0 {
		t.Fatalf("checks after removal = %+v, want none", checks)
	}
}
