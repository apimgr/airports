// Package cve implements the download/cache/update subsystem for the
// scheduled "cve_update" task (AI.md PART 18/31). It maintains a local,
// incrementally-refreshed cache of NVD (National Vulnerability Database)
// CVE records under {data_dir}/security/cve/ for operator awareness.
//
// This package is intentionally a data cache only — it does not scan,
// score, or match any local software inventory against the cached CVEs.
package cve

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"
)

// nvdAPIBaseURL is the NVD CVE API 2.0 endpoint (replaces the retired
// yearly nvdcve-1.1-*.json.gz feed files).
const nvdAPIBaseURL = "https://services.nvd.nist.gov/rest/json/cves/2.0"

// resultsPerPage is the page size requested from the NVD API. 2000 is the
// documented maximum; using the maximum minimizes the number of requests
// needed per run given the tight unauthenticated rate limit.
const resultsPerPage = 2000

// firstRunLookbackDays bounds the initial fetch window on a brand-new
// cache (no prior watermark) so the first run stays a bounded, reasonable
// download rather than the full 200k+ CVE historical dataset.
const firstRunLookbackDays = 90

// incrementalOverlapMargin is subtracted from the previous watermark
// before the next incremental run, so a run that partially failed (or a
// clock skew) can never silently skip CVEs modified right at the boundary.
const incrementalOverlapMargin = 2 * time.Hour

// noKeyRequestDelay is the minimum spacing between paginated requests
// when no NVD API key is configured. NVD's published public rate limit
// is 5 requests per rolling 30 second window without a key.
const noKeyRequestDelay = 7 * time.Second

// withKeyRequestDelay is the minimum spacing between paginated requests
// when an NVD API key is configured (published limit: 50 requests / 30s).
const withKeyRequestDelay = 700 * time.Millisecond

// maxRetries caps the number of retry attempts for a single page request
// before the run gives up and reports an error for this scheduled run.
const maxRetries = 5

// retryBaseDelay is the initial backoff delay; it doubles on each
// subsequent retry attempt (capped by retryMaxDelay).
const retryBaseDelay = 2 * time.Second

// retryMaxDelay caps the exponential backoff delay between retries.
const retryMaxDelay = 60 * time.Second

// indexFileName is the local upsert-style CVE cache, keyed by CVE ID.
const indexFileName = "cve_index.json"

// watermarkFileName tracks the last successful lastModEndDate watermark
// so the next scheduled run knows where to resume its incremental fetch.
const watermarkFileName = "last_updated.json"

// CVERecord is a single cached CVE entry. Convenience fields are decoded
// out of the upstream NVD record for easy consumption; Raw preserves the
// full upstream "cve" object so a future consumer is never blocked on
// this package exposing every upstream field.
type CVERecord struct {
	ID               string          `json:"id"`
	SourceIdentifier string          `json:"source_identifier,omitempty"`
	Published        string          `json:"published,omitempty"`
	LastModified     string          `json:"last_modified,omitempty"`
	VulnStatus       string          `json:"vuln_status,omitempty"`
	Description      string          `json:"description,omitempty"`
	CVSSScore        float64         `json:"cvss_score,omitempty"`
	CVSSSeverity     string          `json:"cvss_severity,omitempty"`
	Raw              json.RawMessage `json:"raw,omitempty"`
}

// watermark records how far the incremental fetch has progressed.
type watermark struct {
	Initialized bool      `json:"initialized"`
	LastModEnd  time.Time `json:"last_mod_end"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// nvdVulnerabilityWrapper mirrors the NVD API 2.0 "vulnerabilities[]"
// array entries; CVE is kept as a RawMessage so it can both be decoded
// into nvdCVE for convenience fields and stored verbatim in CVERecord.Raw.
type nvdVulnerabilityWrapper struct {
	CVE json.RawMessage `json:"cve"`
}

// nvdAPIResponse mirrors the NVD API 2.0 top-level response shape.
type nvdAPIResponse struct {
	ResultsPerPage  int                       `json:"resultsPerPage"`
	StartIndex      int                       `json:"startIndex"`
	TotalResults    int                       `json:"totalResults"`
	Vulnerabilities []nvdVulnerabilityWrapper `json:"vulnerabilities"`
}

// nvdCVE mirrors the fields of a single upstream "cve" object that this
// package extracts into convenience fields on CVERecord.
type nvdCVE struct {
	ID               string `json:"id"`
	SourceIdentifier string `json:"sourceIdentifier"`
	Published        string `json:"published"`
	LastModified     string `json:"lastModified"`
	VulnStatus       string `json:"vulnStatus"`
	Descriptions     []struct {
		Lang  string `json:"lang"`
		Value string `json:"value"`
	} `json:"descriptions"`
	Metrics struct {
		CvssMetricV31 []nvdCVSSMetric `json:"cvssMetricV31"`
		CvssMetricV30 []nvdCVSSMetric `json:"cvssMetricV30"`
		CvssMetricV2  []nvdCVSSMetric `json:"cvssMetricV2"`
	} `json:"metrics"`
}

// nvdCVSSMetric mirrors the common shape shared by cvssMetricV31,
// cvssMetricV30, and cvssMetricV2 entries.
type nvdCVSSMetric struct {
	CvssData struct {
		BaseScore    float64 `json:"baseScore"`
		BaseSeverity string  `json:"baseSeverity"`
	} `json:"cvssData"`
	BaseSeverity string `json:"baseSeverity"`
}

// Service downloads and maintains the local NVD CVE cache.
type Service struct {
	dir        string
	apiKey     string
	apiBaseURL string
	httpClient *http.Client
	now        func() time.Time
	sleep      func(time.Duration)
}

// NewService creates a CVE update service rooted at securityDir/cve.
// The directory is created (0700) eagerly; a failure here is logged and
// a valid, non-nil Service is still returned so callers never fail
// startup over this operator-awareness feature (mirrors the geoip
// package's fail-open pattern per AI.md PART 18/19).
func NewService(securityDir, apiKey string) (*Service, error) {
	if securityDir == "" {
		return nil, fmt.Errorf("security directory is required")
	}

	dir := filepath.Join(securityDir, "cve")
	s := &Service{
		dir:        dir,
		apiKey:     apiKey,
		apiBaseURL: nvdAPIBaseURL,
		httpClient: &http.Client{Timeout: 45 * time.Second},
		now:        time.Now,
		sleep:      time.Sleep,
	}

	if err := os.MkdirAll(dir, 0700); err != nil {
		log.Printf("Warning: failed to create CVE cache directory %s: %v", dir, err)
		log.Println("CVE database updates disabled for this session (operator awareness data only)")
		return s, nil
	}

	return s, nil
}

// requestDelay returns the minimum spacing enforced between paginated
// requests, per NVD's published rate limits.
func (s *Service) requestDelay() time.Duration {
	if s.apiKey != "" {
		return withKeyRequestDelay
	}
	return noKeyRequestDelay
}

// Update performs one incremental NVD CVE fetch-and-cache run. On a
// fresh cache it bounds the fetch to the last firstRunLookbackDays days
// of published CVEs; on subsequent runs it fetches only CVEs modified
// since the last successful watermark (with an overlap margin).
//
// Network/rate-limit failures are retried with capped exponential
// backoff; if retries are exhausted the run gives up and returns an
// error rather than blocking or crashing the caller. Any records
// successfully fetched before a failure are still persisted.
func (s *Service) Update() error {
	if s.dir == "" {
		return fmt.Errorf("cve service not initialized")
	}

	wm, err := s.loadWatermark()
	if err != nil {
		log.Printf("Warning: failed to read CVE watermark, treating as first run: %v", err)
		wm = watermark{}
	}

	now := s.now().UTC()
	params, rangeEnd := s.dateRangeParams(wm, now)

	index, err := s.loadIndex()
	if err != nil {
		log.Printf("Warning: failed to read existing CVE index, starting a new one: %v", err)
		index = map[string]CVERecord{}
	}

	startIndex := 0
	fetched := 0
	for {
		page, err := s.fetchPageWithRetry(params, startIndex)
		if err != nil {
			if saveErr := s.saveIndex(index); saveErr != nil {
				log.Printf("Warning: failed to persist partial CVE index: %v", saveErr)
			}
			return fmt.Errorf("cve update failed after %d records: %w", fetched, err)
		}

		for _, vuln := range page.Vulnerabilities {
			record, decodeErr := decodeRecord(vuln.CVE)
			if decodeErr != nil {
				log.Printf("Warning: skipping malformed CVE record: %v", decodeErr)
				continue
			}
			index[record.ID] = record
			fetched++
		}

		startIndex += len(page.Vulnerabilities)
		if len(page.Vulnerabilities) == 0 || startIndex >= page.TotalResults {
			break
		}

		s.sleep(s.requestDelay())
	}

	if err := s.saveIndex(index); err != nil {
		return fmt.Errorf("failed to save CVE index: %w", err)
	}

	newWatermark := watermark{
		Initialized: true,
		LastModEnd:  rangeEnd,
		UpdatedAt:   now,
	}
	if err := s.saveWatermark(newWatermark); err != nil {
		return fmt.Errorf("failed to save CVE watermark: %w", err)
	}

	log.Printf("CVE database update complete: %d records updated, %d total cached", fetched, len(index))
	return nil
}

// dateRangeParams builds the NVD query parameters for this run and
// returns the "end" boundary that should become the new watermark on
// success.
func (s *Service) dateRangeParams(wm watermark, now time.Time) (url.Values, time.Time) {
	params := url.Values{}

	if !wm.Initialized {
		start := now.AddDate(0, 0, -firstRunLookbackDays)
		params.Set("pubStartDate", formatNVDDate(start))
		params.Set("pubEndDate", formatNVDDate(now))
		return params, now
	}

	start := wm.LastModEnd.Add(-incrementalOverlapMargin)
	if start.After(now) {
		start = now.Add(-incrementalOverlapMargin)
	}
	params.Set("lastModStartDate", formatNVDDate(start))
	params.Set("lastModEndDate", formatNVDDate(now))
	return params, now
}

// formatNVDDate renders a time.Time in the ISO-8601 format the NVD API
// requires for its date range parameters.
func formatNVDDate(t time.Time) string {
	return t.UTC().Format("2006-01-02T15:04:05.000")
}

// fetchPageWithRetry fetches a single page of results, retrying on
// transient/rate-limit failures with capped exponential backoff.
func (s *Service) fetchPageWithRetry(params url.Values, startIndex int) (*nvdAPIResponse, error) {
	delay := retryBaseDelay
	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			s.sleep(delay)
			delay *= 2
			if delay > retryMaxDelay {
				delay = retryMaxDelay
			}
		}

		page, err := s.fetchPage(params, startIndex)
		if err == nil {
			return page, nil
		}
		lastErr = err

		if !isRetryable(err) {
			return nil, err
		}
		log.Printf("Warning: CVE update request failed (attempt %d/%d): %v", attempt+1, maxRetries+1, err)
	}

	return nil, fmt.Errorf("exceeded %d retries: %w", maxRetries, lastErr)
}

// retryableError wraps an error alongside the HTTP status that caused
// it, so isRetryable can distinguish transient failures from permanent
// ones (e.g. malformed request parameters).
type retryableError struct {
	statusCode int
	err        error
}

func (e *retryableError) Error() string { return e.err.Error() }
func (e *retryableError) Unwrap() error { return e.err }

// isRetryable reports whether err represents a transient failure (rate
// limiting or a server-side error) worth retrying.
func isRetryable(err error) bool {
	re, ok := err.(*retryableError)
	if !ok {
		return true
	}
	switch re.statusCode {
	case http.StatusForbidden, http.StatusTooManyRequests,
		http.StatusInternalServerError, http.StatusBadGateway,
		http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return true
	case 0:
		return true
	default:
		return false
	}
}

// fetchPage performs a single HTTP request against the NVD API.
func (s *Service) fetchPage(params url.Values, startIndex int) (*nvdAPIResponse, error) {
	reqParams := url.Values{}
	for k, v := range params {
		reqParams[k] = v
	}
	reqParams.Set("resultsPerPage", fmt.Sprintf("%d", resultsPerPage))
	reqParams.Set("startIndex", fmt.Sprintf("%d", startIndex))

	reqURL := s.apiBaseURL + "?" + reqParams.Encode()

	req, err := http.NewRequest(http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("building CVE request: %w", err)
	}
	req.Header.Set("User-Agent", "airports-server/cve-updater")
	req.Header.Set("Accept", "application/json")
	if s.apiKey != "" {
		req.Header.Set("apiKey", s.apiKey)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, &retryableError{statusCode: 0, err: fmt.Errorf("CVE request failed: %w", err)}
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, &retryableError{statusCode: resp.StatusCode, err: fmt.Errorf("reading CVE response body: %w", err)}
	}

	if resp.StatusCode != http.StatusOK {
		return nil, &retryableError{
			statusCode: resp.StatusCode,
			err:        fmt.Errorf("CVE API returned status %d: %s", resp.StatusCode, truncateForLog(body)),
		}
	}

	var page nvdAPIResponse
	if err := json.Unmarshal(body, &page); err != nil {
		return nil, fmt.Errorf("decoding CVE API response: %w", err)
	}

	return &page, nil
}

// truncateForLog keeps error-log bodies short and free of noise.
func truncateForLog(body []byte) string {
	const maxLen = 200
	if len(body) > maxLen {
		return string(body[:maxLen]) + "..."
	}
	return string(body)
}

// decodeRecord converts a raw upstream "cve" object into a CVERecord,
// preserving the original JSON verbatim in Raw.
func decodeRecord(raw json.RawMessage) (CVERecord, error) {
	var cve nvdCVE
	if err := json.Unmarshal(raw, &cve); err != nil {
		return CVERecord{}, fmt.Errorf("decoding CVE object: %w", err)
	}
	if cve.ID == "" {
		return CVERecord{}, fmt.Errorf("CVE object missing id field")
	}

	record := CVERecord{
		ID:               cve.ID,
		SourceIdentifier: cve.SourceIdentifier,
		Published:        cve.Published,
		LastModified:     cve.LastModified,
		VulnStatus:       cve.VulnStatus,
		Raw:              raw,
	}

	for _, d := range cve.Descriptions {
		if d.Lang == "en" {
			record.Description = d.Value
			break
		}
	}

	if m, ok := bestCVSSMetric(cve); ok {
		record.CVSSScore = m.CvssData.BaseScore
		if m.CvssData.BaseSeverity != "" {
			record.CVSSSeverity = m.CvssData.BaseSeverity
		} else {
			record.CVSSSeverity = m.BaseSeverity
		}
	}

	return record, nil
}

// bestCVSSMetric prefers the newest available CVSS version (3.1, then
// 3.0, then 2.0), matching NVD's own display precedence.
func bestCVSSMetric(cve nvdCVE) (nvdCVSSMetric, bool) {
	if len(cve.Metrics.CvssMetricV31) > 0 {
		return cve.Metrics.CvssMetricV31[0], true
	}
	if len(cve.Metrics.CvssMetricV30) > 0 {
		return cve.Metrics.CvssMetricV30[0], true
	}
	if len(cve.Metrics.CvssMetricV2) > 0 {
		return cve.Metrics.CvssMetricV2[0], true
	}
	return nvdCVSSMetric{}, false
}

// loadIndex reads the local CVE index. A missing file is not an error —
// it just means this is the first run.
func (s *Service) loadIndex() (map[string]CVERecord, error) {
	path := filepath.Join(s.dir, indexFileName)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]CVERecord{}, nil
		}
		return nil, err
	}

	var index map[string]CVERecord
	if err := json.Unmarshal(data, &index); err != nil {
		return nil, fmt.Errorf("parsing existing CVE index: %w", err)
	}
	if index == nil {
		index = map[string]CVERecord{}
	}
	return index, nil
}

// saveIndex atomically writes the CVE index (write-temp-then-rename).
func (s *Service) saveIndex(index map[string]CVERecord) error {
	return atomicWriteJSON(filepath.Join(s.dir, indexFileName), index)
}

// loadWatermark reads the last-successful-update watermark. A missing
// file is not an error — it means this service has never run before.
func (s *Service) loadWatermark() (watermark, error) {
	path := filepath.Join(s.dir, watermarkFileName)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return watermark{}, nil
		}
		return watermark{}, err
	}

	var wm watermark
	if err := json.Unmarshal(data, &wm); err != nil {
		return watermark{}, fmt.Errorf("parsing CVE watermark: %w", err)
	}
	return wm, nil
}

// saveWatermark atomically writes the update watermark.
func (s *Service) saveWatermark(wm watermark) error {
	return atomicWriteJSON(filepath.Join(s.dir, watermarkFileName), wm)
}

// atomicWriteJSON marshals v as indented JSON and writes it to path via
// a temp-file-then-rename so a crash mid-write never corrupts the
// previous, still-valid cache file.
func atomicWriteJSON(path string, v interface{}) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding %s: %w", filepath.Base(path), err)
	}
	data = append(data, '\n')

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0600); err != nil {
		return fmt.Errorf("writing %s: %w", filepath.Base(tmp), err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("renaming %s into place: %w", filepath.Base(tmp), err)
	}
	return nil
}
