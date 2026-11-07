package server

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"runtime/debug"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/apimgr/airports/src/airports"
	cachepkg "github.com/apimgr/airports/src/cache"
	commonhttputil "github.com/apimgr/airports/src/common/httputil"
	"github.com/apimgr/airports/src/common/i18n"
	"github.com/apimgr/airports/src/common/urlvars"
	"github.com/apimgr/airports/src/config"
	dbmetrics "github.com/apimgr/airports/src/db"
	"github.com/apimgr/airports/src/geoip"
	"github.com/apimgr/airports/src/i2p"
	"github.com/apimgr/airports/src/logging"
	"github.com/apimgr/airports/src/mode"
	"github.com/apimgr/airports/src/scheduler"
	"github.com/apimgr/airports/src/tor"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// Asset URL prefixes for the embedded Swagger UI and GraphiQL bundles.
// The UI HTML templates reference these — keep in sync with route mounts.
const (
	swaggerAssetsPrefix = "/server/docs/swagger/assets/"
	graphqlAssetsPrefix = "/server/docs/graphql/assets/"
)

// Build info (set from main)
var (
	Version   = "dev"
	CommitID  = "unknown"
	BuildDate = "unknown"
)

// Server holds application dependencies
type Server struct {
	airports    *airports.Service
	geoip       *geoip.Service
	config      *config.Config
	configDir   string
	configMu    sync.RWMutex // guards config hot-reload
	proxyCIDRs  *trustedProxyCIDRCache
	router      *chi.Mux
	rateLimiter *RateLimiter
	metrics     *appMetrics
	scheduler   *scheduler.Scheduler
	cache       cachepkg.Cache
	blockStore  *BlockStore
	allowlist   *AllowlistLookup
	urlResolver *urlvars.Manager

	// db, dataDir, and tor back /server/healthz component checks (AI.md
	// PART 13). db and tor may be nil (e.g. in unit tests that don't wire
	// them up) — every consumer treats that as a hard "error"/"disabled"
	// state rather than panicking.
	db                       *sql.DB
	dataDir                  string
	tor                      *tor.Manager
	i2p                      *i2p.Manager
	stats                    *statsCollector
	startTime                time.Time
	contactCaptchaMu         sync.Mutex
	contactCaptchaChallenges map[string]contactCaptchaChallenge
	contactCaptchaRequired   map[string]time.Time
	cliReleaseCacheMu        sync.Mutex
	cliReleaseCache          *cliReleaseCache
	cliReleaseAPIBase        string
	cliReleaseHTTPClient     *http.Client
}

// ErrorResponse is the canonical error envelope per AI.md PART 14:
// {"ok": false, "error": "CODE", "message": "...", "details": {}}.
// The HTTP status code carries status — it is intentionally NOT in the body.
// Details is never omitted: callers that have nothing to add must still
// serialize an empty object, never a missing field.
type ErrorResponse struct {
	OK      bool        `json:"ok"`
	Error   string      `json:"error"`
	Message string      `json:"message"`
	Details interface{} `json:"details"`
}

// New creates a new server instance. db, dataDir, and torMgr feed the
// /server/healthz component checks (AI.md PART 13); db and torMgr may be
// nil when the caller has no such dependency (e.g. tests).
func New(airportSvc *airports.Service, geoipSvc *geoip.Service, cfg *config.Config, sched *scheduler.Scheduler, db *sql.DB, dataDir, configDir string, torMgr *tor.Manager, recentLogs ...*logging.RecentLogBuffer) *Server {
	dbmetrics.SetQueryLoggingEnabled(cfg.Server.Debug.LogQueries)
	i18n.Configure(i18n.Config{
		Enabled:            cfg.Server.I18n.Enabled,
		DefaultLanguage:    cfg.Server.I18n.DefaultLanguage,
		AvailableLanguages: cfg.Server.I18n.Supported,
		FallbackLanguage:   cfg.Server.I18n.FallbackLanguage,
		CookieName:         cfg.Server.I18n.CookieName,
		CookieMaxAge:       cfg.Server.I18n.CookieMaxAge,
	})

	// Initialize templates
	if err := initTemplates(); err != nil {
		log.Printf("Warning: Failed to load templates: %v", err)
	}

	// Cache backend per AI.md PART 9/12: memory by default, valkey/redis
	// opt-in. Never crash startup on a bad/unreachable cache config — fall
	// back to memory and log a warning (config file is the source of truth,
	// but invalid values must degrade gracefully per PART 5).
	cache, err := cachepkg.NewWithDebug(cfg.Server.Cache, cfg.Server.Debug.LogCache)
	if err != nil {
		log.Printf("Warning: cache backend %q unavailable (%v) — falling back to memory", cfg.Server.Cache.Type, err)
		cache, err = cachepkg.NewWithDebug(config.CacheConfig{Type: "memory", Prefix: cfg.Server.Cache.Prefix, TTL: cfg.Server.Cache.TTL}, cfg.Server.Debug.LogCache)
		if err != nil {
			log.Printf("Warning: memory cache fallback failed (%v) — caching disabled", err)
			cache, _ = cachepkg.NewWithDebug(config.CacheConfig{Type: "none"}, cfg.Server.Debug.LogCache)
		}
	}

	blockStore := NewBlockStore(cfg.Server.Security.BlockedIPs)
	startTime := time.Now()

	s := &Server{
		airports:                 airportSvc,
		geoip:                    geoipSvc,
		config:                   cfg,
		configDir:                configDir,
		rateLimiter:              NewConfiguredRateLimiter(cfg, configDir, blockStore),
		metrics:                  newMetrics(cfg.Server.Metrics, buildInfo{version: Version, commit: CommitID, buildDate: BuildDate}, startTime, dataDir),
		scheduler:                sched,
		cache:                    cache,
		blockStore:               blockStore,
		allowlist:                NewAllowlistLookup(cfg.Server.Security.Allowlist),
		db:                       db,
		dataDir:                  dataDir,
		tor:                      torMgr,
		stats:                    newStatsCollector(),
		startTime:                startTime,
		contactCaptchaChallenges: make(map[string]contactCaptchaChallenge),
		contactCaptchaRequired:   make(map[string]time.Time),
		cliReleaseAPIBase:        "https://api.github.com",
		cliReleaseHTTPClient:     &http.Client{Timeout: 12 * time.Second},
	}
	s.rateLimiter.debugLogDB = s.debugLogDB
	s.configureURLResolver(cfg)
	useExternalRateLimitCache := cfg.Server.Cache.Type == "redis" || cfg.Server.Cache.Type == "valkey"
	if err := s.rateLimiter.SetPersistence(db, cache, useExternalRateLimitCache); err != nil {
		log.Printf("Warning: failed to initialize persistent rate-limit storage (%s); counters will remain process-local", rateLimitStorageErrorKind(err))
	}

	s.rateLimiter.metrics = s.metrics
	s.metrics.setDatabase(db)
	s.metrics.setCache(cache)
	if sched != nil {
		s.metrics.setSchedulerMetrics(sched.PrometheusCollector())
	}
	s.metrics.setTorManager(torMgr)
	if len(recentLogs) > 0 {
		s.metrics.setLokiLogs(recentLogs[0])
	}
	s.setupRouter()
	return s
}

// BlockStore returns the server's shared IP blocklist store, used by the
// scheduler's blocklist_update task so scheduled downloads take effect on
// the already-running server's middleware without a restart.
func (s *Server) BlockStore() *BlockStore {
	return s.blockStore
}

// Allowlist returns the server's shared IP allowlist lookup.
func (s *Server) Allowlist() *AllowlistLookup {
	return s.allowlist
}

// IsTrustedProxyRequest reports whether the request's immediate peer is in
// the same dynamically refreshed trusted-proxy ranges used by the router.
func (s *Server) IsTrustedProxyRequest(r *http.Request) bool {
	if s == nil || r == nil || s.proxyCIDRs == nil {
		return false
	}
	return isTrustedProxyPeer(r.RemoteAddr, s.proxyCIDRs.CIDRs())
}

// Cache returns the configured cache backend (never nil).
func (s *Server) Cache() cachepkg.Cache {
	return s.cache
}

// CloseCache releases the cache backend's resources (connections,
// background goroutines). Safe to call during graceful shutdown.
func (s *Server) CloseCache() error {
	if s.cache == nil {
		return nil
	}
	return s.cache.Close()
}

// Router returns the configured HTTP router
func (s *Server) Router() http.Handler {
	return s.router
}

// ReloadConfig swaps in a new configuration atomically. Called on SIGHUP.
// The router is not rebuilt — settings take effect on the next request.
func (s *Server) ReloadConfig(cfg *config.Config) {
	dbmetrics.SetQueryLoggingEnabled(cfg.Server.Debug.LogQueries)
	s.configMu.Lock()
	s.config = cfg
	s.configMu.Unlock()
	cachepkg.SetDebugLogging(s.cache, cfg.Server.Debug.LogCache)
	s.rateLimiter.setDebugLogCache(cfg.Server.Debug.LogCache)
	if s.i2p != nil {
		if err := s.i2p.UpdateConfig(cfg.Server.I2P); err != nil {
			log.Printf("Warning: failed to reload I2P eepsite: %v", err)
		}
	}
	s.configureURLResolver(cfg)
}

func (s *Server) configureURLResolver(cfg *config.Config) {
	if cfg == nil {
		return
	}
	options := urlvars.Options{
		FQDN: cfg.Server.FQDN, DomainEnv: os.Getenv("DOMAIN"), ListenPort: cfg.Server.Port,
		OnionAddress: cfg.Server.Tor.OnionAddress, ProjectName: "airports",
		Development: strings.EqualFold(cfg.Server.Mode, "development"), Detection: cfg.Server.URLDetection,
		TrustedPeer: s.IsTrustedProxyRequest, BaseURL: RequestBaseURL,
	}
	if s.i2p != nil {
		options.I2PAddress = s.i2p.EepsiteAddress()
	}
	if s.urlResolver == nil {
		s.urlResolver = urlvars.NewManager(options)
	} else {
		s.urlResolver.UpdateOptions(options)
	}
	urlvars.SetDefaultManager(s.urlResolver)
}

// SetI2PManager attaches the optional eepsite lifecycle to request handling.
func (s *Server) SetI2PManager(manager *i2p.Manager) {
	s.i2p = manager
	s.configureURLResolver(s.config)
}

// JSON helpers

func encodeJSON(w http.ResponseWriter, data interface{}) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(data)
}

func writeTextError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(status)
	_, _ = fmt.Fprintf(w, "ERROR: %s: %s\n", code, message)
}

// respondItem sends a single resource wrapped in the canonical success
// envelope per AI.md PART 14: {"ok": true, "data": ...}.
func (s *Server) respondItem(w http.ResponseWriter, status int, item interface{}) {
	resp := map[string]interface{}{
		"ok":   true,
		"data": item,
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := encodeJSON(w, resp); err != nil {
		log.Printf("respondItem: encode failed: %v", err)
	}
}

// respondList sends a paginated list wrapped in the canonical success
// envelope per AI.md PART 14: {"ok": true, "data": [...], "pagination": {...}}.
func (s *Server) respondList(w http.ResponseWriter, status int, data interface{}, page, limit, total int) {
	pages := 0
	if limit > 0 {
		pages = (total + limit - 1) / limit
	}
	resp := map[string]interface{}{
		"ok":   true,
		"data": data,
		"pagination": map[string]interface{}{
			"page":  page,
			"limit": limit,
			"total": total,
			"pages": pages,
		},
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := encodeJSON(w, resp); err != nil {
		log.Printf("respondList: encode failed: %v", err)
	}
}

// respondError sends the canonical API error in the negotiated JSON or text
// format. msgKey is an i18n key ("errors.*") looked up in the request's
// resolved language; args are optional Tf-style "{name}" substitutions.
// Never pass raw err.Error() here — log internal errors server-side instead.
func (s *Server) respondError(w http.ResponseWriter, r *http.Request, status int, code, msgKey string, args ...interface{}) {
	lang := i18n.FromContext(r.Context())
	s.writeErrorJSON(w, r, status, code, i18n.Tf(lang, msgKey, args...))
}

// writeErrorJSON sends the canonical error response with an already-translated
// message, using the negotiated API format. The request id (from middleware.GetReqID, the
// same value echoed in the X-Request-ID header) is surfaced in details so a
// failed API call can be correlated with server logs (AI.md PART 9). Details
// is never omitted — an empty object is serialized when there is no request id.
func (s *Server) writeErrorJSON(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	if commonhttputil.GetAPIResponseFormat(r) == "text" {
		writeTextError(w, status, code, message)
		return
	}
	writeJSONError(w, r, status, code, message)
}

func writeJSONError(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	details := map[string]interface{}{}
	if id := middleware.GetReqID(r.Context()); id != "" {
		details["request_id"] = id
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	resp := ErrorResponse{
		OK:      false,
		Error:   code,
		Message: message,
		Details: details,
	}

	if err := encodeJSON(w, resp); err != nil {
		log.Printf("writeErrorJSON: encode failed: %v", err)
	}
}

// errorCodeForStatus maps an HTTP status to the canonical PART 9/14 error code.
func errorCodeForStatus(status int) string {
	switch status {
	case http.StatusBadRequest:
		return "BAD_REQUEST"
	case http.StatusUnauthorized:
		return "UNAUTHORIZED"
	case http.StatusForbidden:
		return "FORBIDDEN"
	case http.StatusNotFound:
		return "NOT_FOUND"
	case http.StatusMethodNotAllowed:
		return "METHOD_NOT_ALLOWED"
	case http.StatusConflict:
		return "CONFLICT"
	case http.StatusTooManyRequests:
		return "RATE_LIMITED"
	case http.StatusServiceUnavailable:
		return "MAINTENANCE"
	default:
		return "SERVER_ERROR"
	}
}

// statusMessageKey maps an HTTP status to an errors.* i18n key, or "" when
// no dedicated key exists (the caller falls back to http.StatusText).
func statusMessageKey(status int) string {
	switch status {
	case http.StatusBadRequest:
		return "errors.bad_request"
	case http.StatusUnauthorized:
		return "errors.unauthorized"
	case http.StatusForbidden:
		return "errors.forbidden"
	case http.StatusNotFound:
		return "errors.not_found"
	case http.StatusTooManyRequests:
		return "errors.rate_limited"
	case http.StatusServiceUnavailable:
		return "errors.maintenance"
	case http.StatusInternalServerError:
		return "errors.internal"
	default:
		return ""
	}
}

// renderErrorPage sends a themed, content-negotiated error response per AI.md
// PART 14/16: the canonical JSON envelope for API/CLI clients, the themed
// error.html page (extending base.html) for browsers, and plain text
// otherwise. All three carry the correct HTTP status code; JSON additionally
// carries the request id (PART 9). It is safe to call only before any response
// body has been written.
func (s *Server) renderErrorPage(w http.ResponseWriter, r *http.Request, status int) {
	lang := i18n.FromContext(r.Context())
	code := errorCodeForStatus(status)

	message := http.StatusText(status)
	if key := statusMessageKey(status); key != "" {
		message = i18n.T(lang, key)
	}

	accept := r.Header.Get("Accept")
	switch {
	case strings.Contains(accept, "application/json"):
		s.writeErrorJSON(w, r, status, code, message)
	case strings.Contains(accept, "text/html"):
		statusText := http.StatusText(status)
		data := map[string]interface{}{
			"Title":      statusText,
			"StatusCode": status,
			"StatusText": statusText,
			"Message":    message,
		}
		// On a pre-write template failure, fall back to a bare (unthemed)
		// response — never recurse into themed rendering.
		if err := s.executeTemplateStatus(w, r, "error", status, data); err != nil {
			log.Printf("renderErrorPage: render error template failed: %v", err)
			http.Error(w, message, status)
		}
	default:
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(status)
		_, _ = fmt.Fprintf(w, "ERROR: %s: %s\n", code, message)
	}
}

func (s *Server) respondText(w http.ResponseWriter, status int, text string) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(status)
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.TrimRight(text, "\n") + "\n"
	_, _ = w.Write([]byte(text))
}

// handleAPIInfo returns API information
func (s *Server) handleAPIInfo(w http.ResponseWriter, r *http.Request) {
	baseURL := strings.TrimSuffix(RequestBaseURL(r), "/")
	base := baseURL + s.APIBasePath()
	info := map[string]interface{}{
		"name":        "Airports API",
		"version":     s.config.Server.APIVersion,
		"description": "Global airport location information API with GeoIP integration",
		"endpoints": map[string]string{
			"airports":     base + "/airports",
			"search":       base + "/airports/search?q=query",
			"nearby":       base + "/airports/nearby?lat=40.64&lon=-73.78&radius=50",
			"within":       base + "/airports/within?lat_min=&lat_max=&lon_min=&lon_max=",
			"autocomplete": base + "/airports/autocomplete?q=",
			"airport":      base + "/airports/{ident}",
			"geoip":        base + "/geoip",
			"stats":        base + "/stats",
			"countries":    base + "/countries",
			"swagger":      base + "/server/swagger",
			"graphql":      base + "/server/graphql",
			"healthz":      base + "/server/healthz",
			"autodiscover": baseURL + "/api/autodiscover",
		},
		"documentation": baseURL + "/server/docs/swagger",
	}

	s.respondItem(w, http.StatusOK, info)
}

// handleRobotsTxt returns robots.txt based on config, per AI.md PART 11
// "robots.txt". Denied AI crawlers get their own stanza ahead of the wildcard
// block; bots resolving to allow are covered by "User-agent: *" and get none.
func (s *Server) handleRobotsTxt(w http.ResponseWriter, r *http.Request) {
	var sb strings.Builder

	for _, bot := range s.deniedAIBots() {
		_, _ = fmt.Fprintf(&sb, "User-agent: %s\nDisallow: /\n\n", bot)
	}

	sb.WriteString("User-agent: *\n")
	for _, path := range s.config.Web.Robots.Allow {
		_, _ = fmt.Fprintf(&sb, "Allow: %s\n", path)
	}
	for _, path := range s.config.Web.Robots.Deny {
		_, _ = fmt.Fprintf(&sb, "Disallow: %s\n", path)
	}
	_, _ = fmt.Fprintf(&sb, "Sitemap: %s/sitemap.xml\n", s.publicBaseURL(r))

	s.respondText(w, http.StatusOK, sb.String())
}

// deniedAIBots returns the sorted list of configured AI crawlers that resolve
// to deny — either an explicit "deny" value, or no explicit value while
// web.robots.ai_bots.default is "deny". Explicit entries always win over the
// default, per AI.md PART 11 "AI Crawler Rules".
func (s *Server) deniedAIBots() []string {
	aiBots := s.config.Web.Robots.AIBots
	denied := make([]string, 0, len(aiBots.Bots))
	for bot, access := range aiBots.Bots {
		switch access {
		case "deny":
			denied = append(denied, bot)
		case "allow":
			// explicitly allowed — covered by the wildcard block
		default:
			if aiBots.Default == "deny" {
				denied = append(denied, bot)
			}
		}
	}
	sort.Strings(denied)
	return denied
}

// handleLocaleJSON serves the embedded translation file for {lang} so
// WebUI JavaScript can fetch /locales/{lang}.json, per AI.md PART 30
// "WebUI JavaScript" row. Unsupported languages fall back to English
// rather than erroring (PART 30 "Unsupported language fallback").
func (s *Server) handleLocaleJSON(w http.ResponseWriter, r *http.Request) {
	lang := strings.TrimSuffix(chi.URLParam(r, "lang"), ".json")

	raw, err := i18n.RawLocale(lang)
	if err != nil {
		s.renderErrorPage(w, r, http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(raw)
	_, _ = w.Write([]byte("\n"))
}

// publicBaseURL resolves scheme://host for the current request. The scheme is
// derived from the actual TLS state; X-Forwarded-Proto is honored ONLY when
// the request arrived through a recognized trusted proxy (AI.md PART 12).
// The trusted-proxy signal reused here is the same one getClientIP/realIP
// rely on: middleware.ClientIPFromXFF resolves a client IP into the context
// only after walking past trusted proxy hops, so a non-empty value means the
// peer was trusted. Untrusted peers can spoof the header, so their forwarded
// proto is ignored. The default port for the resolved scheme (:80 for http,
// :443 for https) is stripped from the host per AI.md PART 13/15.
func (s *Server) publicBaseURL(r *http.Request) string {
	if s.isI2PRequest(r) {
		return "http://" + strings.TrimSuffix(strings.TrimSpace(s.i2p.EepsiteAddress()), "/") + strings.TrimSuffix(RequestBaseURL(r), "/")
	}
	if s.isTorRequest(r) {
		return "http://" + strings.TrimSuffix(strings.TrimSpace(s.config.Server.Tor.OnionAddress), "/") + strings.TrimSuffix(RequestBaseURL(r), "/")
	}
	if s.urlResolver != nil {
		return s.urlResolver.BuildURL(r, "")
	}
	baseURL := RequestBaseURL(r)
	if baseURL == "/" {
		baseURL = ""
	}
	if s.isTorRequest(r) {
		return "http://" + strings.TrimSuffix(strings.TrimSpace(s.config.Server.Tor.OnionAddress), "/") + baseURL
	}
	scheme := urlvars.ResolveProto(r, middleware.GetClientIP(r.Context()) != "")
	return scheme + "://" + stripDefaultPort(r.Host, scheme) + baseURL
}

func (s *Server) isI2PRequest(r *http.Request) bool {
	if r == nil || s.i2p == nil || s.i2p.EepsiteAddress() == "" {
		return false
	}
	requestHost := urlvars.OriginalRequestHost(r)
	if host, _, err := net.SplitHostPort(requestHost); err == nil {
		requestHost = host
	}
	return strings.EqualFold(strings.TrimSuffix(requestHost, "."), strings.TrimSuffix(s.i2p.EepsiteAddress(), "."))
}

func (s *Server) isOverlayRequest(r *http.Request) bool {
	return s.isTorRequest(r) || s.isI2PRequest(r)
}

// isTorRequest recognizes only a request accepted by the dedicated Tor
// backend listener with validated PROXY-protocol circuit metadata. Host is
// client-controlled and must never establish Tor identity.
func (s *Server) isTorRequest(r *http.Request) bool {
	return TorCircuitID(r) != ""
}

// stripDefaultPort removes the scheme's default port (:80 for http, :443 for
// https) from host, leaving other ports intact. IPv6 literals keep their
// bracket notation. Hosts without a port are returned unchanged.
func stripDefaultPort(host, scheme string) string {
	h, port, err := net.SplitHostPort(host)
	if err != nil {
		return host
	}
	if (scheme == "http" && port == "80") || (scheme == "https" && port == "443") {
		if strings.Contains(h, ":") {
			return "[" + h + "]"
		}
		return h
	}
	return host
}

// sitemapEntry describes one <url> block in sitemap.xml per AI.md PART 16
// "Sitemap Generation Rules" (priority/changefreq by page type).
type sitemapEntry struct {
	Path       string
	Priority   string
	ChangeFreq string
}

// publicSitemapEntries is the single list of routes considered public content:
// it drives both sitemap.xml and the per-route "index,follow" robots meta tag
// (AI.md PART 16 "Robots Directive" — every other route fails closed to
// "noindex,nofollow"). Never add API, health, debug, or internal routes here.
var publicSitemapEntries = []sitemapEntry{
	{Path: "/", Priority: "1.0", ChangeFreq: "daily"},
	{Path: "/airports/search", Priority: "0.8", ChangeFreq: "weekly"},
	{Path: "/airports/nearby", Priority: "0.8", ChangeFreq: "weekly"},
	{Path: "/stats", Priority: "0.8", ChangeFreq: "weekly"},
	{Path: "/geoip", Priority: "0.8", ChangeFreq: "weekly"},
	{Path: "/server/about", Priority: "0.8", ChangeFreq: "weekly"},
	{Path: "/server/help", Priority: "0.8", ChangeFreq: "weekly"},
	{Path: "/server/privacy", Priority: "0.8", ChangeFreq: "weekly"},
	{Path: "/server/terms", Priority: "0.8", ChangeFreq: "weekly"},
	{Path: "/server/contact", Priority: "0.8", ChangeFreq: "weekly"},
	{Path: "/server/docs/swagger", Priority: "0.7", ChangeFreq: "weekly"},
	{Path: "/server/docs/graphql", Priority: "0.7", ChangeFreq: "weekly"},
}

// handleSitemap returns a dynamically generated sitemap.xml listing every
// public frontend route. API endpoints and any authenticated/server-management
// pages are NEVER included per AI.md PART 16.
func (s *Server) handleSitemap(w http.ResponseWriter, r *http.Request) {
	base := s.publicBaseURL(r)
	lastmod := time.Now().UTC().Format("2006-01-02")

	entries := publicSitemapEntries

	var sb strings.Builder
	sb.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	sb.WriteString(`<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">` + "\n")
	for _, e := range entries {
		sb.WriteString("  <url>\n")
		sb.WriteString("    <loc>" + base + e.Path + "</loc>\n")
		sb.WriteString("    <lastmod>" + lastmod + "</lastmod>\n")
		sb.WriteString("    <changefreq>" + e.ChangeFreq + "</changefreq>\n")
		sb.WriteString("    <priority>" + e.Priority + "</priority>\n")
		sb.WriteString("  </url>\n")
	}
	sb.WriteString("</urlset>\n")

	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(sb.String()))
}

// handleSecurityTxt returns security.txt based on config
func (s *Server) handleSecurityTxt(w http.ResponseWriter, r *http.Request) {
	contact := s.securityContactEmail(r)

	content := fmt.Sprintf("Expires: %s\n", time.Now().AddDate(1, 0, 0).Format("2006-01-02T15:04:05Z"))
	if contact != "" {
		content = "Contact: mailto:" + contact + "\n" + content
	}
	if !s.isTorRequest(r) {
		content += "Preferred-Languages: en\n"
	}

	s.respondText(w, http.StatusOK, content)
}

func (s *Server) trackingCSPSources(r *http.Request) (string, string) {
	if !canLoadAnalytics(r, s.config) || s.isTorRequest(r) {
		return "", ""
	}
	tracking := s.config.Server.Tracking
	if config.ValidateTracking(&tracking) != nil {
		return "", ""
	}
	scripts := make([]string, 0, 2)
	connects := make([]string, 0, 3)
	addOrigin := func(target *[]string, origin string) {
		if origin != "" && !slices.Contains(*target, origin) {
			*target = append(*target, origin)
		}
	}
	customOrigin := func() string {
		parsed, err := url.Parse(tracking.URL)
		if err != nil || parsed.Host == "" || parsed.User != nil || (parsed.Scheme != "https" && parsed.Scheme != "http") {
			return ""
		}
		return parsed.Scheme + "://" + parsed.Host
	}
	switch tracking.Type {
	case "google":
		addOrigin(&scripts, "https://www.googletagmanager.com")
		addOrigin(&scripts, "https://www.google-analytics.com")
		addOrigin(&connects, "https://www.google-analytics.com")
		addOrigin(&connects, "https://analytics.google.com")
	case "matomo", "piwik", "owa", "umami":
		origin := customOrigin()
		addOrigin(&scripts, origin)
		addOrigin(&connects, origin)
	case "fathom":
		origin := customOrigin()
		if origin == "" {
			origin = "https://cdn.usefathom.com"
			addOrigin(&connects, "https://api.usefathom.com")
		}
		addOrigin(&scripts, origin)
		addOrigin(&connects, origin)
	case "plausible":
		origin := customOrigin()
		if origin == "" {
			origin = "https://plausible.io"
		}
		addOrigin(&scripts, origin)
		addOrigin(&connects, origin)
	case "simple":
		addOrigin(&scripts, "https://scripts.simpleanalyticscdn.com")
		addOrigin(&connects, "https://queue.simpleanalyticscdn.com")
	case "cloudflare":
		addOrigin(&scripts, "https://static.cloudflareinsights.com")
		addOrigin(&connects, "https://cloudflareinsights.com")
	}
	return strings.Join(scripts, " "), strings.Join(connects, " ")
}

// handleManifest returns the PWA web app manifest.
func (s *Server) handleManifest(w http.ResponseWriter, r *http.Request) {
	baseURL := strings.TrimSuffix(RequestBaseURL(r), "/")
	icon := func(name string) string { return baseURL + "/static/icons/" + name }
	manifest := map[string]interface{}{
		"name":             "Airports API",
		"short_name":       "Airports",
		"description":      "Global airport location information API",
		"start_url":        baseURL + "/",
		"scope":            baseURL + "/",
		"display":          "standalone",
		"orientation":      "any",
		"background_color": "#282a36",
		"theme_color":      "#bd93f9",
		"categories":       []string{"utilities", "travel"},
		"icons": []map[string]string{
			{"src": icon("icon-72.png"), "sizes": "72x72", "type": "image/png"},
			{"src": icon("icon-96.png"), "sizes": "96x96", "type": "image/png"},
			{"src": icon("icon-128.png"), "sizes": "128x128", "type": "image/png"},
			{"src": icon("icon-144.png"), "sizes": "144x144", "type": "image/png"},
			{"src": icon("icon-152.png"), "sizes": "152x152", "type": "image/png"},
			{"src": icon("icon-192.png"), "sizes": "192x192", "type": "image/png"},
			{"src": icon("icon-384.png"), "sizes": "384x384", "type": "image/png"},
			{"src": icon("icon-512.png"), "sizes": "512x512", "type": "image/png"},
			{"src": icon("icon-maskable-192.png"), "sizes": "192x192", "type": "image/png", "purpose": "maskable"},
			{"src": icon("icon-maskable-512.png"), "sizes": "512x512", "type": "image/png", "purpose": "maskable"},
		},
	}

	w.Header().Set("Content-Type", "application/manifest+json")
	if err := encodeJSON(w, manifest); err != nil {
		log.Printf("handleManifest: encode failed: %v", err)
	}
}

// handleServiceWorker returns the PWA service worker
func (s *Server) handleServiceWorker(w http.ResponseWriter, r *http.Request) {
	baseURL, _ := json.Marshal(strings.TrimSuffix(RequestBaseURL(r), "/"))
	sw := `// Service Worker for Airports API
const BASE_URL = __BASE_URL__;
const CACHE_NAME = 'airports-v1';
const urlsToCache = [
  BASE_URL + '/',
  BASE_URL + '/static/css/common.css',
  BASE_URL + '/static/css/components.css',
  BASE_URL + '/static/css/public.css',
  BASE_URL + '/static/js/app.js',
  BASE_URL + '/static/offline.html'
];

self.addEventListener('install', event => {
  event.waitUntil(
    caches.open(CACHE_NAME)
      .then(cache => cache.addAll(urlsToCache))
  );
});

self.addEventListener('fetch', event => {
  event.respondWith(
    caches.match(event.request)
      .then(response => response || fetch(event.request)
        .catch(() => {
          if (event.request.mode === 'navigate') {
            return caches.match(BASE_URL + '/static/offline.html');
          }
          return Response.error();
        }))
  );
});

`
	sw = strings.Replace(sw, "__BASE_URL__", string(baseURL), 1)
	w.Header().Set("Content-Type", "application/javascript")
	_, _ = w.Write([]byte(sw))
}

// handleGetSettings returns current configuration (read-only)
func (s *Server) handleGetSettings(w http.ResponseWriter, r *http.Request) {
	// Return safe subset of settings (no sensitive data)
	settings := map[string]interface{}{
		"theme":   s.config.Web.UI.Theme,
		"cors":    s.config.Server.CORS.AllowedOrigins,
		"metrics": s.config.Server.Metrics.Enabled,
	}

	s.respondItem(w, http.StatusOK, settings)
}

// panicRecoverer recovers from handler panics per AI.md PART 6's
// mode-dependent panic recovery: "graceful" in production (log the error,
// return a generic 500) and "verbose" in development (also write the
// recovered value and stack trace into the response so a local developer
// sees the failure immediately). Replaces chi's generic middleware.Recoverer
// so the response body can vary by mode.
func (s *Server) panicRecoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				log.Printf("panic recovered: %v\n%s", rec, debug.Stack())
				w.Header().Set("Content-Type", "text/plain; charset=utf-8")
				w.WriteHeader(http.StatusInternalServerError)
				if mode.GetPanicRecoveryMode() == "verbose" {
					_, _ = fmt.Fprintf(w, "panic: %v\n\n%s", rec, debug.Stack())
					return
				}
				_, _ = fmt.Fprint(w, "An internal error occurred. Please contact support if the problem persists.")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// securityHeaders applies the baseline HTTP security headers required by
// AI.md PART 11 ("Security Headers" / "Cross-Origin Isolation Headers" /
// "Reporting API" / "Content Security Policy"). HSTS is set conditionally
// on TLS-terminated requests.
func (s *Server) securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		// Server: and X-App-Version expose the app name and version per AI.md PART 14.
		h.Set("Server", "airports/"+Version)
		h.Set("X-App-Version", Version)
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "SAMEORIGIN")
		h.Set("X-XSS-Protection", "1; mode=block")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		h.Set("X-Permitted-Cross-Domain-Policies", "none")
		h.Set("Origin-Agent-Cluster", "?1")
		// Cross-Origin Isolation headers (AI.md PART 11 §"Cross-Origin
		// Isolation Headers"). COOP/COEP stay at their spec defaults
		// (unsafe-none) since this app neither embeds SharedArrayBuffer
		// nor handles payment/banking flows; CORP stays cross-origin so
		// the public API/static assets remain embeddable.
		h.Set("Cross-Origin-Opener-Policy", "unsafe-none")
		h.Set("Cross-Origin-Embedder-Policy", "unsafe-none")
		h.Set("Cross-Origin-Resource-Policy", "cross-origin")
		// Permissions-Policy: spec default table (AI.md PART 11 §"Permissions-Policy
		// Configuration"), with geolocation scoped to self since the app's
		// "use my location" / nearby-airports feature actively uses it.
		h.Set("Permissions-Policy", strings.Join([]string{
			"accelerometer=()",
			"ambient-light-sensor=()",
			"attribution-reporting=()",
			"autoplay=(self)",
			"battery=()",
			"browsing-topics=()",
			"camera=()",
			"display-capture=()",
			"encrypted-media=(self)",
			"fullscreen=(self)",
			"geolocation=(self)",
			"gyroscope=()",
			"hid=()",
			"idle-detection=()",
			"interest-cohort=()",
			"magnetometer=()",
			"microphone=()",
			"midi=()",
			"payment=(self)",
			"picture-in-picture=(self)",
			"publickey-credentials-get=(self)",
			"screen-wake-lock=()",
			"serial=()",
			"storage-access=(self)",
			"usb=()",
			"web-share=(self)",
			"xr-spatial-tracking=()",
		}, ", "))

		// Reporting API (AI.md PART 11 §"Reporting API (Modern + Legacy)"):
		// modern Reporting-Endpoints plus the legacy Report-To/NEL headers,
		// both pointing at the same default report group.
		reportURL := s.publicBaseURL(r) + s.APIBasePath() + "/server/reports/default"
		h.Set("Reporting-Endpoints", `default="`+reportURL+`"`)
		h.Set("Report-To", `{"group":"default","max_age":10886400,"endpoints":[{"url":"`+reportURL+`"}]}`)
		h.Set("NEL", `{"report_to":"default","max_age":2592000,"include_subdomains":true}`)

		// Content-Security-Policy (AI.md PART 11 §"Content Security Policy",
		// default per-directive policy). script-src is 'self' only — all JS
		// lives in static/js/app.js, no inline scripts or inline handlers
		// remain anywhere in the frontend (AI.md PART 16). style-src keeps
		// 'unsafe-inline' per the documented spec default (inline style=""
		// attributes are unavoidable).
		cspReportURI := s.publicBaseURL(r) + s.APIBasePath() + "/server/reports/csp"
		csp :=
			"default-src 'self'; " +
				"script-src 'self'; " +
				"style-src 'self' 'unsafe-inline'; " +
				"img-src 'self' data: blob: https:; " +
				"font-src 'self' https:; " +
				"connect-src 'self'; " +
				"media-src 'self' blob:; " +
				"worker-src 'self' blob:; " +
				"manifest-src 'self'; " +
				"frame-src 'self'; " +
				"frame-ancestors 'self'; " +
				"base-uri 'self'; " +
				"form-action 'self'; " +
				"object-src 'none'; " +
				"report-to default; " +
				"report-uri " + cspReportURI
		if scripts, connects := s.trackingCSPSources(r); scripts != "" || connects != "" {
			if scripts != "" {
				csp = strings.Replace(csp, "script-src 'self';", "script-src 'self' "+scripts+";", 1)
			}
			if connects != "" {
				csp = strings.Replace(csp, "connect-src 'self';", "connect-src 'self' "+connects+";", 1)
			}
		}
		if !s.isOverlayRequest(r) {
			csp += " upgrade-insecure-requests;"
		}
		h.Set("Content-Security-Policy", csp)
		// HSTS only when the request actually arrived over TLS (either
		// direct or via a trusted reverse-proxy that set X-Forwarded-Proto).
		secureRequest := urlvars.ResolveProto(r, false) == "https"
		if s.urlResolver != nil {
			proto, _, _ := s.urlResolver.GetURLVars(r)
			secureRequest = proto == "https"
		} else if middleware.GetClientIP(r.Context()) != "" {
			secureRequest = urlvars.ResolveProto(r, true) == "https"
		}
		if !s.isOverlayRequest(r) && secureRequest {
			h.Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains; preload")
		}
		next.ServeHTTP(w, r)
	})
}

// requestIDHeader echoes the chi-generated request ID back to the client
// in the X-Request-ID response header so it can be quoted in bug reports.
func (s *Server) requestIDHeader(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if id := middleware.GetReqID(r.Context()); id != "" {
			w.Header().Set("X-Request-ID", id)
		}
		next.ServeHTTP(w, r)
	})
}

// handleGeoIPPage renders the GeoIP lookup page. Reads "ip" from the query
// string and performs the lookup server-side so the page works as a plain
// GET form with JavaScript disabled (AI.md PART 16 "No JavaScript-Disabled
// Broken State") — the JS-enhanced form posts back the same "ip" param and
// preventDefault()s to render the result via AJAX instead.
func (s *Server) handleGeoIPPage(w http.ResponseWriter, r *http.Request) {
	data := map[string]interface{}{
		"Title": "GeoIP Lookup",
		"Theme": s.config.Web.UI.Theme,
	}

	ipStr := strings.TrimSpace(r.URL.Query().Get("ip"))
	if ipStr != "" {
		data["IP"] = ipStr
		if loc, err := s.geoip.LookupString(ipStr); err != nil {
			data["LookupError"] = mode.GetErrorDetail(err)
		} else {
			data["Location"] = loc
			if loc.Latitude != 0 || loc.Longitude != 0 {
				data["Nearby"] = s.airports.GetNearbyWithDistance(loc.Latitude, loc.Longitude, 100, 10, airports.UnitImperial)
			}
		}
	}

	s.renderTemplate(w, r, "geoip", data)
}
