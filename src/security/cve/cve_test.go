package cve

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

// newTestService builds a Service rooted at a temp dir with sleep/now
// stubbed so tests run instantly and deterministically.
func newTestService(t *testing.T, baseURL string) *Service {
	t.Helper()
	dir := t.TempDir()
	s, err := NewService(dir, "")
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	s.apiBaseURL = baseURL
	s.sleep = func(time.Duration) {}
	s.now = func() time.Time {
		return time.Date(2026, 7, 24, 12, 0, 0, 0, time.UTC)
	}
	return s
}

func sampleCVE(id, lastModified string) json.RawMessage {
	obj := map[string]interface{}{
		"id":               id,
		"sourceIdentifier": "cve@mitre.org",
		"published":        "2026-01-01T00:00:00.000",
		"lastModified":     lastModified,
		"vulnStatus":       "Analyzed",
		"descriptions": []map[string]string{
			{"lang": "en", "value": "Sample description for " + id},
			{"lang": "es", "value": "Descripcion de muestra"},
		},
		"metrics": map[string]interface{}{
			"cvssMetricV31": []map[string]interface{}{
				{
					"cvssData": map[string]interface{}{
						"baseScore":    9.8,
						"baseSeverity": "CRITICAL",
					},
				},
			},
		},
	}
	raw, _ := json.Marshal(obj)
	return raw
}

func TestNewServiceCreatesDir(t *testing.T) {
	dir := t.TempDir()
	s, err := NewService(dir, "key123")
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "cve")); err != nil {
		t.Fatalf("expected cve dir to be created: %v", err)
	}
	if s.requestDelay() != withKeyRequestDelay {
		t.Errorf("expected withKeyRequestDelay when apiKey set, got %v", s.requestDelay())
	}
}

func TestNewServiceRequiresDir(t *testing.T) {
	if _, err := NewService("", ""); err == nil {
		t.Fatal("expected error for empty security dir")
	}
}

func TestRequestDelayWithoutKey(t *testing.T) {
	s, err := NewService(t.TempDir(), "")
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	if s.requestDelay() != noKeyRequestDelay {
		t.Errorf("expected noKeyRequestDelay, got %v", s.requestDelay())
	}
}

func TestDateRangeParamsFirstRun(t *testing.T) {
	s := newTestService(t, "http://unused")
	now := s.now()
	params, end := s.dateRangeParams(watermark{}, now)

	if params.Get("pubStartDate") == "" || params.Get("pubEndDate") == "" {
		t.Fatalf("expected pub date params on first run, got %v", params)
	}
	if params.Get("lastModStartDate") != "" || params.Get("lastModEndDate") != "" {
		t.Fatalf("did not expect lastMod params on first run, got %v", params)
	}
	if !end.Equal(now) {
		t.Errorf("expected watermark end to equal now, got %v want %v", end, now)
	}

	wantStart := now.AddDate(0, 0, -firstRunLookbackDays)
	if params.Get("pubStartDate") != formatNVDDate(wantStart) {
		t.Errorf("pubStartDate = %q, want %q", params.Get("pubStartDate"), formatNVDDate(wantStart))
	}
}

func TestDateRangeParamsIncremental(t *testing.T) {
	s := newTestService(t, "http://unused")
	now := s.now()
	prevEnd := now.Add(-24 * time.Hour)
	wm := watermark{Initialized: true, LastModEnd: prevEnd}

	params, end := s.dateRangeParams(wm, now)

	if params.Get("pubStartDate") != "" || params.Get("pubEndDate") != "" {
		t.Fatalf("did not expect pub date params on incremental run, got %v", params)
	}
	wantStart := prevEnd.Add(-incrementalOverlapMargin)
	if params.Get("lastModStartDate") != formatNVDDate(wantStart) {
		t.Errorf("lastModStartDate = %q, want %q", params.Get("lastModStartDate"), formatNVDDate(wantStart))
	}
	if params.Get("lastModEndDate") != formatNVDDate(now) {
		t.Errorf("lastModEndDate = %q, want %q", params.Get("lastModEndDate"), formatNVDDate(now))
	}
	if !end.Equal(now) {
		t.Errorf("expected watermark end to equal now, got %v", end)
	}
}

func TestDateRangeParamsIncrementalClockSkew(t *testing.T) {
	s := newTestService(t, "http://unused")
	now := s.now()
	// Watermark far in the future relative to "now" (clock skew case).
	wm := watermark{Initialized: true, LastModEnd: now.Add(48 * time.Hour)}

	params, _ := s.dateRangeParams(wm, now)
	wantStart := now.Add(-incrementalOverlapMargin)
	if params.Get("lastModStartDate") != formatNVDDate(wantStart) {
		t.Errorf("expected clock-skew fallback start %q, got %q", formatNVDDate(wantStart), params.Get("lastModStartDate"))
	}
}

func TestUpdateFirstRunSinglePage(t *testing.T) {
	var requests int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requests, 1)
		if got := r.Header.Get("User-Agent"); got != "airports-server/cve-updater" {
			t.Errorf("unexpected User-Agent: %q", got)
		}
		resp := nvdAPIResponse{
			ResultsPerPage: 2,
			StartIndex:     0,
			TotalResults:   2,
			Vulnerabilities: []nvdVulnerabilityWrapper{
				{CVE: sampleCVE("CVE-2026-0001", "2026-07-20T00:00:00.000")},
				{CVE: sampleCVE("CVE-2026-0002", "2026-07-21T00:00:00.000")},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	s := newTestService(t, server.URL)
	if err := s.Update(); err != nil {
		t.Fatalf("Update: %v", err)
	}
	if atomic.LoadInt32(&requests) != 1 {
		t.Errorf("expected exactly 1 request, got %d", requests)
	}

	index, err := s.loadIndex()
	if err != nil {
		t.Fatalf("loadIndex: %v", err)
	}
	if len(index) != 2 {
		t.Fatalf("expected 2 cached records, got %d", len(index))
	}
	rec, ok := index["CVE-2026-0001"]
	if !ok {
		t.Fatal("expected CVE-2026-0001 in index")
	}
	if rec.Description != "Sample description for CVE-2026-0001" {
		t.Errorf("unexpected description: %q", rec.Description)
	}
	if rec.CVSSScore != 9.8 || rec.CVSSSeverity != "CRITICAL" {
		t.Errorf("unexpected CVSS fields: %+v", rec)
	}

	wm, err := s.loadWatermark()
	if err != nil {
		t.Fatalf("loadWatermark: %v", err)
	}
	if !wm.Initialized {
		t.Error("expected watermark to be initialized after first run")
	}
	if !wm.LastModEnd.Equal(s.now()) {
		t.Errorf("expected watermark LastModEnd = now, got %v", wm.LastModEnd)
	}
}

func TestUpdatePagination(t *testing.T) {
	pageSize := 1
	total := 3
	var seenStartIndexes []string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		startIdx := r.URL.Query().Get("startIndex")
		seenStartIndexes = append(seenStartIndexes, startIdx)

		idx := 0
		fmt.Sscanf(startIdx, "%d", &idx)
		var vulns []nvdVulnerabilityWrapper
		if idx < total {
			vulns = []nvdVulnerabilityWrapper{
				{CVE: sampleCVE(fmt.Sprintf("CVE-2026-%04d", idx), "2026-07-20T00:00:00.000")},
			}
		}
		resp := nvdAPIResponse{
			ResultsPerPage:  pageSize,
			StartIndex:      idx,
			TotalResults:    total,
			Vulnerabilities: vulns,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	s := newTestService(t, server.URL)
	if err := s.Update(); err != nil {
		t.Fatalf("Update: %v", err)
	}

	if len(seenStartIndexes) != total {
		t.Fatalf("expected %d paginated requests, got %d: %v", total, len(seenStartIndexes), seenStartIndexes)
	}

	index, err := s.loadIndex()
	if err != nil {
		t.Fatalf("loadIndex: %v", err)
	}
	if len(index) != total {
		t.Fatalf("expected %d records, got %d", total, len(index))
	}
}

func TestUpdateUpsertsIntoExistingIndex(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := nvdAPIResponse{
			ResultsPerPage: 1,
			StartIndex:     0,
			TotalResults:   1,
			Vulnerabilities: []nvdVulnerabilityWrapper{
				{CVE: sampleCVE("CVE-2026-0001", "2026-07-24T06:00:00.000")},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	s := newTestService(t, server.URL)

	preexisting := map[string]CVERecord{
		"CVE-2020-9999": {ID: "CVE-2020-9999", Description: "Old unrelated CVE"},
		"CVE-2026-0001": {ID: "CVE-2026-0001", Description: "Stale description", LastModified: "2026-07-01T00:00:00.000"},
	}
	if err := s.saveIndex(preexisting); err != nil {
		t.Fatalf("saveIndex: %v", err)
	}
	if err := s.saveWatermark(watermark{Initialized: true, LastModEnd: s.now().Add(-time.Hour)}); err != nil {
		t.Fatalf("saveWatermark: %v", err)
	}

	if err := s.Update(); err != nil {
		t.Fatalf("Update: %v", err)
	}

	index, err := s.loadIndex()
	if err != nil {
		t.Fatalf("loadIndex: %v", err)
	}
	if len(index) != 2 {
		t.Fatalf("expected upsert to preserve unrelated record, got %d entries", len(index))
	}
	if index["CVE-2020-9999"].Description != "Old unrelated CVE" {
		t.Errorf("expected unrelated record to be preserved untouched")
	}
	if index["CVE-2026-0001"].LastModified != "2026-07-24T06:00:00.000" {
		t.Errorf("expected upsert to refresh LastModified, got %q", index["CVE-2026-0001"].LastModified)
	}
}

func TestUpdateRetriesOnRateLimitThenSucceeds(t *testing.T) {
	var attempts int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&attempts, 1)
		if n <= 2 {
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte(`{"message":"rate limited"}`))
			return
		}
		resp := nvdAPIResponse{
			ResultsPerPage:  1,
			StartIndex:      0,
			TotalResults:    1,
			Vulnerabilities: []nvdVulnerabilityWrapper{{CVE: sampleCVE("CVE-2026-0099", "2026-07-24T00:00:00.000")}},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	s := newTestService(t, server.URL)
	if err := s.Update(); err != nil {
		t.Fatalf("Update: %v", err)
	}
	if atomic.LoadInt32(&attempts) != 3 {
		t.Fatalf("expected 3 attempts (2 failures + 1 success), got %d", attempts)
	}

	index, err := s.loadIndex()
	if err != nil {
		t.Fatalf("loadIndex: %v", err)
	}
	if _, ok := index["CVE-2026-0099"]; !ok {
		t.Fatal("expected record fetched after retry to be indexed")
	}
}

func TestUpdateGivesUpAfterMaxRetries(t *testing.T) {
	var attempts int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&attempts, 1)
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte(`{"message":"unavailable"}`))
	}))
	defer server.Close()

	s := newTestService(t, server.URL)
	err := s.Update()
	if err == nil {
		t.Fatal("expected error after exhausting retries")
	}
	if got := atomic.LoadInt32(&attempts); got != maxRetries+1 {
		t.Errorf("expected %d attempts, got %d", maxRetries+1, got)
	}
}

func TestUpdateNonRetryableFailsImmediately(t *testing.T) {
	var attempts int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&attempts, 1)
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"message":"bad request"}`))
	}))
	defer server.Close()

	s := newTestService(t, server.URL)
	err := s.Update()
	if err == nil {
		t.Fatal("expected error for non-retryable failure")
	}
	if got := atomic.LoadInt32(&attempts); got != 1 {
		t.Errorf("expected exactly 1 attempt for non-retryable failure, got %d", got)
	}
}

func TestUpdateMalformedJSONResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{not valid json`))
	}))
	defer server.Close()

	s := newTestService(t, server.URL)
	if err := s.Update(); err == nil {
		t.Fatal("expected error for malformed JSON response")
	}
}

func TestUpdateSkipsMalformedRecordButKeepsOthers(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := nvdAPIResponse{
			ResultsPerPage: 2,
			StartIndex:     0,
			TotalResults:   2,
			Vulnerabilities: []nvdVulnerabilityWrapper{
				{CVE: json.RawMessage(`{"sourceIdentifier":"x"}`)},
				{CVE: sampleCVE("CVE-2026-0055", "2026-07-24T00:00:00.000")},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	s := newTestService(t, server.URL)
	if err := s.Update(); err != nil {
		t.Fatalf("Update: %v", err)
	}

	index, err := s.loadIndex()
	if err != nil {
		t.Fatalf("loadIndex: %v", err)
	}
	if len(index) != 1 {
		t.Fatalf("expected malformed record to be skipped, got %d entries", len(index))
	}
	if _, ok := index["CVE-2026-0055"]; !ok {
		t.Fatal("expected valid record to still be indexed")
	}
}

func TestDecodeRecordPrefersNewestCVSS(t *testing.T) {
	raw := sampleCVE("CVE-2026-0001", "2026-07-24T00:00:00.000")
	rec, err := decodeRecord(raw)
	if err != nil {
		t.Fatalf("decodeRecord: %v", err)
	}
	if rec.CVSSScore != 9.8 {
		t.Errorf("expected CVSS v3.1 score to be preferred, got %v", rec.CVSSScore)
	}
}

func TestDecodeRecordMissingID(t *testing.T) {
	if _, err := decodeRecord(json.RawMessage(`{"sourceIdentifier":"x"}`)); err == nil {
		t.Fatal("expected error for CVE object missing id")
	}
}

func TestBestCVSSMetricFallback(t *testing.T) {
	var cve nvdCVE
	cve.Metrics.CvssMetricV2 = []nvdCVSSMetric{{BaseSeverity: "HIGH"}}
	cve.Metrics.CvssMetricV2[0].CvssData.BaseScore = 7.5

	m, ok := bestCVSSMetric(cve)
	if !ok {
		t.Fatal("expected fallback to CVSS v2 metric")
	}
	if m.CvssData.BaseScore != 7.5 {
		t.Errorf("unexpected fallback score: %v", m.CvssData.BaseScore)
	}
}

func TestBestCVSSMetricNone(t *testing.T) {
	if _, ok := bestCVSSMetric(nvdCVE{}); ok {
		t.Fatal("expected no CVSS metric to be found")
	}
}

func TestAtomicWriteAndLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sample.json")

	type payload struct {
		Value string `json:"value"`
	}
	if err := atomicWriteJSON(path, payload{Value: "hello"}); err != nil {
		t.Fatalf("atomicWriteJSON: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	var got payload
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if got.Value != "hello" {
		t.Errorf("round-trip mismatch: %+v", got)
	}
	if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		t.Errorf("expected temp file to be renamed away, stat err = %v", err)
	}
}

func TestLoadIndexMissingFileIsNotError(t *testing.T) {
	s := newTestService(t, "http://unused")
	index, err := s.loadIndex()
	if err != nil {
		t.Fatalf("expected no error for missing index file, got %v", err)
	}
	if len(index) != 0 {
		t.Errorf("expected empty index, got %d entries", len(index))
	}
}

func TestLoadWatermarkMissingFileIsNotError(t *testing.T) {
	s := newTestService(t, "http://unused")
	wm, err := s.loadWatermark()
	if err != nil {
		t.Fatalf("expected no error for missing watermark file, got %v", err)
	}
	if wm.Initialized {
		t.Errorf("expected zero-value watermark, got %+v", wm)
	}
}

func TestLoadIndexCorruptFile(t *testing.T) {
	s := newTestService(t, "http://unused")
	if err := os.WriteFile(filepath.Join(s.dir, indexFileName), []byte("not json"), 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if _, err := s.loadIndex(); err == nil {
		t.Fatal("expected error for corrupt index file")
	}
}

func TestLoadWatermarkCorruptFile(t *testing.T) {
	s := newTestService(t, "http://unused")
	if err := os.WriteFile(filepath.Join(s.dir, watermarkFileName), []byte("not json"), 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if _, err := s.loadWatermark(); err == nil {
		t.Fatal("expected error for corrupt watermark file")
	}
}

func TestIsRetryableNonHTTPError(t *testing.T) {
	if !isRetryable(fmt.Errorf("generic error")) {
		t.Error("expected non-retryableError types to be treated as retryable")
	}
}
