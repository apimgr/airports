package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
)

// DatasetHealthCheck is the public-safe result for one registered data set.
type DatasetHealthCheck struct {
	Key      string
	Status   string
	Critical bool
}

type healthCheckRegistration struct {
	key      string
	critical bool
	probe    func() string
	status   string
}

var datasetHealthRegistry = struct {
	sync.RWMutex
	checks []*healthCheckRegistration
}{checks: make([]*healthCheckRegistration, 0)}

// RegisterHealthCheck registers one data set once and caches its initial result.
func RegisterHealthCheck(key string, critical bool, probe func() string) error {
	key = strings.TrimSpace(key)
	if !validHealthCheckKey(key) || probe == nil {
		return fmt.Errorf("invalid health check registration")
	}
	registration := &healthCheckRegistration{key: key, critical: critical, probe: probe, status: "error"}
	registration.status = safeHealthCheckStatus(probe())

	datasetHealthRegistry.Lock()
	defer datasetHealthRegistry.Unlock()
	for i, existing := range datasetHealthRegistry.checks {
		if existing.key == key {
			datasetHealthRegistry.checks[i] = registration
			return nil
		}
	}
	datasetHealthRegistry.checks = append(datasetHealthRegistry.checks, registration)
	return nil
}

// RemoveHealthCheck omits a data set whose feature is disabled.
func RemoveHealthCheck(key string) {
	datasetHealthRegistry.Lock()
	defer datasetHealthRegistry.Unlock()
	for i, check := range datasetHealthRegistry.checks {
		if check.key == key {
			datasetHealthRegistry.checks = append(datasetHealthRegistry.checks[:i], datasetHealthRegistry.checks[i+1:]...)
			return
		}
	}
}

// RefreshHealthChecks refreshes cached data set results outside the request path.
func RefreshHealthChecks() {
	datasetHealthRegistry.RLock()
	checks := append([]*healthCheckRegistration(nil), datasetHealthRegistry.checks...)
	datasetHealthRegistry.RUnlock()

	results := make([]string, len(checks))
	for i, check := range checks {
		results[i] = safeHealthCheckStatus(check.probe())
	}

	datasetHealthRegistry.Lock()
	for i, check := range checks {
		for _, registered := range datasetHealthRegistry.checks {
			if registered.key == check.key {
				registered.status = results[i]
				break
			}
		}
	}
	datasetHealthRegistry.Unlock()
}

func currentDatasetHealthChecks() []DatasetHealthCheck {
	datasetHealthRegistry.RLock()
	defer datasetHealthRegistry.RUnlock()
	checks := make([]DatasetHealthCheck, 0, len(datasetHealthRegistry.checks))
	for _, check := range datasetHealthRegistry.checks {
		checks = append(checks, DatasetHealthCheck{Key: check.key, Status: check.status, Critical: check.critical})
	}
	return checks
}

func validHealthCheckKey(key string) bool {
	if key == "" || key[0] < 'a' || key[0] > 'z' {
		return false
	}
	for _, char := range key[1:] {
		if (char < 'a' || char > 'z') && (char < '0' || char > '9') && char != '_' {
			return false
		}
	}
	return true
}

func safeHealthCheckStatus(status string) string {
	switch status {
	case "ok", "degraded", "error":
		return status
	default:
		return "error"
	}
}

func (checks ChecksInfo) MarshalJSON() ([]byte, error) {
	var output bytes.Buffer
	output.WriteByte('{')
	first := true
	writeField := func(key, value string) error {
		if !first {
			output.WriteByte(',')
		}
		first = false
		encodedKey, err := json.Marshal(key)
		if err != nil {
			return err
		}
		encodedValue, err := json.Marshal(value)
		if err != nil {
			return err
		}
		output.Write(encodedKey)
		output.WriteByte(':')
		output.Write(encodedValue)
		return nil
	}
	for _, field := range []struct{ key, value string }{
		{"database", checks.Database}, {"cache", checks.Cache}, {"disk", checks.Disk}, {"scheduler", checks.Scheduler},
	} {
		if err := writeField(field.key, field.value); err != nil {
			return nil, err
		}
	}
	if checks.Tor != "" {
		if err := writeField("tor", checks.Tor); err != nil {
			return nil, err
		}
	}
	if checks.I2P != "" {
		if err := writeField("i2p", checks.I2P); err != nil {
			return nil, err
		}
	}
	for _, check := range checks.Datasets {
		if !validHealthCheckKey(check.Key) {
			continue
		}
		if err := writeField(check.Key, safeHealthCheckStatus(check.Status)); err != nil {
			return nil, err
		}
	}
	output.WriteByte('}')
	return output.Bytes(), nil
}
