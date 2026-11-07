package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	cachepkg "github.com/apimgr/airports/src/cache"
	"github.com/apimgr/airports/src/config"
)

func TestHandleDebugDBReportsPoolStatistics(t *testing.T) {
	s := &Server{db: &sql.DB{}}
	w := httptest.NewRecorder()
	s.handleDebugDB(w, httptest.NewRequest(http.MethodGet, "/debug/db", nil))

	var response struct {
		OK   bool `json:"ok"`
		Data struct {
			Configured         bool `json:"configured"`
			OpenConnections    int  `json:"open_connections"`
			MaxOpenConnections int  `json:"max_open_connections"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if w.Code != http.StatusOK || !response.OK || !response.Data.Configured {
		t.Fatalf("unexpected response: status=%d body=%s", w.Code, w.Body.String())
	}
	if response.Data.OpenConnections != 0 || response.Data.MaxOpenConnections != 0 {
		t.Errorf("unexpected zero-value DB stats: %+v", response.Data)
	}
}

func TestHandleDebugCacheReportsAvailability(t *testing.T) {
	s := &Server{
		config: &config.Config{},
		cache:  &debugProbeCache{},
	}
	w := httptest.NewRecorder()
	s.handleDebugCache(w, httptest.NewRequest(http.MethodGet, "/debug/cache", nil))

	var response struct {
		OK   bool `json:"ok"`
		Data struct {
			Configured          bool   `json:"configured"`
			Status              string `json:"status"`
			StatisticsAvailable bool   `json:"statistics_available"`
			Hits                uint64 `json:"hits"`
			Misses              uint64 `json:"misses"`
			Items               uint64 `json:"items"`
			Bytes               uint64 `json:"bytes"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if w.Code != http.StatusOK || !response.OK || !response.Data.Configured || response.Data.Status != "ok" {
		t.Fatalf("unexpected response: status=%d body=%s", w.Code, w.Body.String())
	}
	if !response.Data.StatisticsAvailable || response.Data.Hits != 3 || response.Data.Misses != 2 || response.Data.Items != 1 || response.Data.Bytes != 8 {
		t.Errorf("unexpected cache stats: %+v", response.Data)
	}
}

type debugProbeCache struct{}

func (*debugProbeCache) Get(context.Context, string) ([]byte, error) {
	return []byte("1"), nil
}

func (*debugProbeCache) Set(context.Context, string, []byte, time.Duration) error {
	return nil
}

func (*debugProbeCache) Delete(context.Context, string) error {
	return nil
}

func (*debugProbeCache) GetOrSet(_ context.Context, _ string, _ time.Duration, loader func() ([]byte, error)) ([]byte, error) {
	return loader()
}

func (*debugProbeCache) Close() error {
	return nil
}

func (*debugProbeCache) MetricsStats() cachepkg.MetricsStats {
	return cachepkg.MetricsStats{Hits: 3, Misses: 2, Items: 1, Bytes: 8, HasSize: true, Enabled: true}
}
