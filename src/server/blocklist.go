package server

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/apimgr/airports/src/common/i18n"
	"github.com/apimgr/airports/src/config"
	"github.com/apimgr/airports/src/path"
)

// BlockType distinguishes a temporary (rate-limit/abuse, auto-released) IP
// block from a permanent (config-file only, manual release) one, per AI.md
// PART 11 "IP Block Management".
type BlockType string

const (
	BlockTypeTemporary BlockType = "temporary"
	BlockTypePermanent BlockType = "permanent"
)

// IPBlock is a single blocked IP/CIDR entry, matching the data model in
// AI.md PART 11 "IP Block Management".
type IPBlock struct {
	IP           string     `json:"ip"`
	CIDR         string     `json:"cidr,omitempty"`
	Type         BlockType  `json:"type"`
	Reason       string     `json:"reason"`
	BlockedAt    time.Time  `json:"blocked_at"`
	ExpiresAt    *time.Time `json:"expires_at,omitempty"`
	OffenseCount int        `json:"offense_count"`
	AutoBlocked  bool       `json:"auto_blocked"`
}

// defaultTemporaryBlockDuration and defaultTemporaryBlockReason are applied
// by AddTemporary when the caller does not specify them, per AI.md's
// "Temporary | Configurable (default 1h) | ... | Rate limit abuse" table.
const (
	defaultTemporaryBlockDuration = time.Hour
	defaultTemporaryBlockReason   = "rate_limit_abuse"
)

// minAllowedIPv4PrefixBits and minAllowedIPv6PrefixBits enforce AI.md's
// "Reject overly broad ranges" validation rule: IPv4 /0-/7 and IPv6 /0-/15
// are rejected (logged + skipped, never a fatal error).
const (
	minAllowedIPv4PrefixBits = 8
	minAllowedIPv6PrefixBits = 16
)

// parseCIDROrIP parses a config-supplied CIDR or bare IP per AI.md's
// allowlist/blocklist validation rules: single IPs auto-expand to /32
// (IPv4) or /128 (IPv6), and overly broad ranges are rejected. On any
// problem it logs a warning (tagged with the given context label) and
// returns ok=false — callers must skip the entry, never fail startup.
func parseCIDROrIP(raw, context string) (prefix netip.Prefix, ok bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return netip.Prefix{}, false
	}

	if strings.Contains(raw, "/") {
		p, err := netip.ParsePrefix(raw)
		if err != nil {
			log.Printf("%s: invalid CIDR %q: %v (skipped)", context, raw, err)
			return netip.Prefix{}, false
		}
		prefix = p.Masked()
	} else {
		addr, err := netip.ParseAddr(raw)
		if err != nil {
			log.Printf("%s: invalid IP/CIDR %q: %v (skipped)", context, raw, err)
			return netip.Prefix{}, false
		}
		bits := 32
		if addr.Is6() && !addr.Is4In6() {
			bits = 128
		}
		prefix = netip.PrefixFrom(addr, bits)
	}

	minBits := minAllowedIPv4PrefixBits
	if prefix.Addr().Is6() && !prefix.Addr().Is4In6() {
		minBits = minAllowedIPv6PrefixBits
	}
	if prefix.Bits() < minBits {
		log.Printf("%s: %q is broader than the minimum allowed prefix (/%d) — rejected", context, raw, minBits)
		return netip.Prefix{}, false
	}

	return prefix, true
}

// allowlistPrefix pairs a parsed CIDR with its human-readable description.
type allowlistPrefix struct {
	prefix      netip.Prefix
	description string
}

// AllowlistLookup holds the parsed set of trusted IP/CIDR entries from
// server.security.allowlist. It is safe for concurrent use.
type AllowlistLookup struct {
	mu      sync.RWMutex
	entries []allowlistPrefix
}

// NewAllowlistLookup builds an AllowlistLookup from the raw config entries.
// Malformed or overly broad entries are logged and skipped, never fatal.
func NewAllowlistLookup(entries []config.AllowlistEntry) *AllowlistLookup {
	al := &AllowlistLookup{}
	for _, e := range entries {
		prefix, ok := parseCIDROrIP(e.CIDR, "server.security.allowlist")
		if !ok {
			continue
		}
		al.entries = append(al.entries, allowlistPrefix{prefix: prefix, description: e.Description})
	}
	return al
}

// Contains reports whether ip matches any entry in the allowlist.
func (al *AllowlistLookup) Contains(ip string) bool {
	if al == nil {
		return false
	}
	addr, err := netip.ParseAddr(ip)
	if err != nil {
		return false
	}

	al.mu.RLock()
	defer al.mu.RUnlock()
	for _, e := range al.entries {
		if e.prefix.Contains(addr) {
			return true
		}
	}
	return false
}

// permanentBlock pairs a parsed CIDR (from server.security.blocked_ips)
// with its IPBlock metadata.
type permanentBlock struct {
	prefix netip.Prefix
	block  *IPBlock
}

// BlockStore holds every source of IP blocks: permanent entries from the
// config file, temporary entries auto-added by rate-limiting/abuse
// detection, and the downloaded FireHOL netset. It is safe for concurrent
// use.
type BlockStore struct {
	mu         sync.RWMutex
	permanent  []permanentBlock
	temporary  map[string]*IPBlock
	downloaded []netip.Prefix
}

// NewBlockStore builds a BlockStore from the config-file permanent block
// list. Malformed entries are logged and skipped, never fatal.
func NewBlockStore(entries []config.BlockedIPEntry) *BlockStore {
	bs := &BlockStore{
		temporary: make(map[string]*IPBlock),
	}
	for _, e := range entries {
		prefix, ok := parseCIDROrIP(e.CIDR, "server.security.blocked_ips")
		if !ok {
			continue
		}
		reason := e.Reason
		if reason == "" {
			reason = "config"
		}
		bs.permanent = append(bs.permanent, permanentBlock{
			prefix: prefix,
			block: &IPBlock{
				IP:     e.CIDR,
				CIDR:   prefix.String(),
				Type:   BlockTypePermanent,
				Reason: reason,
			},
		})
	}
	return bs
}

// IsBlocked reports whether ip is currently blocked by any source
// (temporary, permanent config, or the downloaded blocklist), and returns
// the matching block's metadata for internal logging (never exposed to
// the client — see BlocklistMiddleware).
func (bs *BlockStore) IsBlocked(ip string) (bool, *IPBlock) {
	if bs == nil {
		return false, nil
	}
	addr, err := netip.ParseAddr(ip)
	if err != nil {
		return false, nil
	}

	bs.mu.RLock()
	defer bs.mu.RUnlock()

	if b, ok := bs.temporary[ip]; ok {
		if b.ExpiresAt == nil || time.Now().Before(*b.ExpiresAt) {
			return true, b
		}
		// Expired — treated as not-blocked here; ReleaseExpired performs
		// the actual cleanup on its own schedule.
	}

	for _, pb := range bs.permanent {
		if pb.prefix.Contains(addr) {
			return true, pb.block
		}
	}

	for _, prefix := range bs.downloaded {
		if prefix.Contains(addr) {
			return true, &IPBlock{
				IP:          ip,
				CIDR:        prefix.String(),
				Type:        BlockTypePermanent,
				Reason:      "firehol_level1_blocklist",
				AutoBlocked: true,
			}
		}
	}

	return false, nil
}

// AddTemporary adds (or refreshes) a temporary block for ip. duration<=0
// falls back to the default (1h); reason=="" falls back to
// "rate_limit_abuse", per AI.md's IP Block Management table.
func (bs *BlockStore) AddTemporary(ip, reason string, duration time.Duration) {
	if bs == nil {
		return
	}
	if duration <= 0 {
		duration = defaultTemporaryBlockDuration
	}
	if reason == "" {
		reason = defaultTemporaryBlockReason
	}

	now := time.Now()
	expires := now.Add(duration)

	bs.mu.Lock()
	defer bs.mu.Unlock()

	if existing, ok := bs.temporary[ip]; ok {
		existing.OffenseCount++
		existing.Reason = reason
		existing.ExpiresAt = &expires
		return
	}

	bs.temporary[ip] = &IPBlock{
		IP:           ip,
		Type:         BlockTypeTemporary,
		Reason:       reason,
		BlockedAt:    now,
		ExpiresAt:    &expires,
		OffenseCount: 1,
		AutoBlocked:  true,
	}
}

// ReleaseExpired releases every temporary block whose duration has expired
// or whose IP now matches the allowlist, per AI.md's "Auto-Release" rules.
// It returns the number of blocks released. Intended to be called every
// minute by the scheduler.
func (bs *BlockStore) ReleaseExpired(allowlist *AllowlistLookup) int {
	if bs == nil {
		return 0
	}
	now := time.Now()

	bs.mu.Lock()
	defer bs.mu.Unlock()

	released := 0
	for ip, b := range bs.temporary {
		expired := b.ExpiresAt != nil && now.After(*b.ExpiresAt)
		nowAllowlisted := allowlist.Contains(ip)
		if expired || nowAllowlisted {
			delete(bs.temporary, ip)
			released++
		}
	}
	return released
}

// LoadDownloadedNetset (re)loads a plain-text, one-CIDR-per-line netset
// file (comments and blank lines ignored) into the store's downloaded
// block list, replacing whatever was previously loaded. Used both by the
// blocklist_update scheduler task and at server startup if a previously
// downloaded file already exists on disk.
func (bs *BlockStore) LoadDownloadedNetset(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	var prefixes []netip.Prefix
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		prefix, ok := parseCIDROrIP(line, "blocklist_update")
		if !ok {
			continue
		}
		prefixes = append(prefixes, prefix)
	}
	if err := scanner.Err(); err != nil {
		return err
	}

	bs.mu.Lock()
	bs.downloaded = prefixes
	bs.mu.Unlock()

	return nil
}

// fireholLevel1URL is the FireHOL level1 netset feed (aggregates Spamhaus
// DROP/EDROP + Dshield, public and free to use, no license restriction).
// Declared as a var (not a const) so tests can point it at an unreachable
// URL to exercise the fail-open download-failure path.
var fireholLevel1URL = "https://iplists.firehol.org/files/firehol_level1.netset"

// downloadNetsetFile downloads url to dest with a 30s timeout and a
// descriptive User-Agent, mirroring the fail-open, atomic-rename pattern
// used by src/geoip's database downloader.
func downloadNetsetFile(dest, url string) error {
	client := &http.Client{Timeout: 30 * time.Second}

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "airports-server/blocklist-updater")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status: %s", resp.Status)
	}

	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

// UpdateBlocklist is the real implementation of the blocklist_update
// scheduled task (AI.md PART 18/PART 27, "Download/update IP/domain
// blocklists"). It downloads the FireHOL level1 netset to a temp file,
// atomically installs it, and reloads it into store. On download failure
// it logs a warning and keeps using the last-downloaded file if one
// exists (fail-open — a stale list is safer than blocking all traffic);
// it only returns an error when there is no previous list to fall back
// on, so the scheduler's retry_on_fail can kick in.
func UpdateBlocklist(store *BlockStore, projectName string) error {
	dir := filepath.Join(paths.GetSecurityDir(projectName), "blocklists")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("blocklist_update: failed to create %s: %w", dir, err)
	}

	finalPath := filepath.Join(dir, "firehol_level1.netset")
	tempPath := finalPath + ".tmp"

	if err := downloadNetsetFile(tempPath, fireholLevel1URL); err != nil {
		os.Remove(tempPath)
		if _, statErr := os.Stat(finalPath); statErr == nil {
			log.Printf("blocklist_update: download failed, keeping last-known list: %v", err)
			return store.LoadDownloadedNetset(finalPath)
		}
		return fmt.Errorf("blocklist_update: download failed and no previous list exists: %w", err)
	}

	if err := os.Rename(tempPath, finalPath); err != nil {
		os.Remove(tempPath)
		return fmt.Errorf("blocklist_update: failed to install downloaded netset: %w", err)
	}

	if err := store.LoadDownloadedNetset(finalPath); err != nil {
		return fmt.Errorf("blocklist_update: failed to load downloaded netset: %w", err)
	}

	log.Println("blocklist_update: FireHOL level1 netset updated successfully")
	return nil
}

// ctxKeyAllowlisted is the context key AllowlistMiddleware sets when the
// client IP matches server.security.allowlist. Downstream middleware
// (blocklist, rate limit, geoip, auto-block) checks IsAllowlisted and
// skips enforcement; auth middleware deliberately ignores it.
type ctxKeyAllowlistedType struct{}

var ctxKeyAllowlisted = ctxKeyAllowlistedType{}

// IsAllowlisted reports whether AllowlistMiddleware marked this request's
// client IP as allowlisted.
func IsAllowlisted(ctx context.Context) bool {
	v, _ := ctx.Value(ctxKeyAllowlisted).(bool)
	return v
}

// AllowlistMiddleware sets a context flag if the client IP is allowlisted,
// per AI.md PART 11 "IP Block Management" — Middleware.
func AllowlistMiddleware(allowlist *AllowlistLookup) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := getClientIP(r)
			if allowlist.Contains(ip) {
				r = r.WithContext(context.WithValue(r.Context(), ctxKeyAllowlisted, true))
			}
			next.ServeHTTP(w, r)
		})
	}
}

// blockedPageHTML is a minimal, self-contained themed error page for
// browser requests to a blocked IP. It never reveals the block reason or
// expiry (Tier 1 — internal-only per AI.md PART 9 backend rules); those
// details are logged server-side only. Follows the project's dark-default
// theme convention using CSS custom properties, no external assets.
const blockedPageHTML = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Access Denied</title>
<style>
  :root { --bg: #0f1115; --fg: #e6e6e6; --accent: #e0555f; }
  @media (prefers-color-scheme: light) {
    :root { --bg: #f7f7f8; --fg: #16181d; --accent: #c22b35; }
  }
  html, body {
    margin: 0; height: 100%;
    background: var(--bg); color: var(--fg);
    font-family: system-ui, sans-serif;
    display: flex; align-items: center; justify-content: center;
  }
  main { max-width: 32rem; text-align: center; padding: 2rem; }
  h1 { color: var(--accent); font-size: 1.5rem; margin-bottom: 0.5rem; }
  p { line-height: 1.5; }
</style>
</head>
<body>
<main>
<h1>403 &mdash; Access Denied</h1>
<p>Your request could not be completed.</p>
</main>
</body>
</html>
`

// respondBlocked sends the 403 response for a blocked IP, following the
// same Accept-header content negotiation used elsewhere in this package
// (e.g. handleServerHealthz): text/html for browsers, JSON otherwise. The
// block reason/expiry are never included in the response body.
func respondBlocked(w http.ResponseWriter, r *http.Request) {
	accept := r.Header.Get("Accept")
	if strings.Contains(accept, "text/html") {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(blockedPageHTML))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusForbidden)
	lang := i18n.FromContext(r.Context())
	resp := ErrorResponse{
		OK:      false,
		Error:   "IP_BLOCKED",
		Message: i18n.T(lang, "errors.forbidden"),
		Details: map[string]interface{}{},
	}
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		log.Printf("respondBlocked: encode failed: %v", err)
	}
}

// BlocklistMiddleware rejects requests from blocked IPs with 403, unless
// the request was already marked allowlisted upstream by
// AllowlistMiddleware. The block reason/expiry are logged internally but
// never exposed to the client, per AI.md PART 9 backend Tier 1 rules.
func BlocklistMiddleware(store *BlockStore) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if IsAllowlisted(r.Context()) {
				next.ServeHTTP(w, r)
				return
			}

			ip := getClientIP(r)
			if blocked, block := store.IsBlocked(ip); blocked {
				log.Printf("security.ip_blocked: rejected request ip=%s reason=%s type=%s path=%s",
					ip, block.Reason, block.Type, r.URL.Path)
				respondBlocked(w, r)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
