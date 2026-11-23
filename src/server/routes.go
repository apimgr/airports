package server

import (
	"io/fs"
	"net/http"
	"strings"

	"github.com/apimgr/airports/src/config"
	graphqlpkg "github.com/apimgr/airports/src/graphql"
	"github.com/apimgr/airports/src/mode"
	swaggerpkg "github.com/apimgr/airports/src/swagger"
	"github.com/go-chi/chi/v5"
)

// setupRouter assembles the global middleware chain and application routes.
func (s *Server) setupRouter() {
	r := chi.NewRouter()
	registerMetrics := func(router chi.Router, path, service string) {
		router.Get(path, metricsAuthMiddleware(
			metricsServiceToken(s.config.Server.Metrics, service),
			s.config.Server.Metrics.Auth.AllowUnauthenticated,
			s.metrics,
			s.metrics.serviceHandler(service),
		).ServeHTTP)
	}
	s.registerMiddleware(r)
	s.registerRoutes(r, registerMetrics)
	s.router = r
}

// registerRoutes mounts the public web, API, metrics, debug, and error routes.
func (s *Server) registerRoutes(r chi.Router, registerMetrics func(chi.Router, string, string)) {
	r.Get("/cli/binaries/{binary}", s.handleCLIBinaryDownload)
	if s.brandingAssets != nil {
		r.Handle("/branding-assets/*", s.brandingAssets.Handler())
	}
	r.Post("/mcp", apiResponseHeaders(s.config.Server.APIVersion, s.handleMCP))
	r.Post("/api/mcp", apiResponseHeaders(s.config.Server.APIVersion, s.handleMCP))

	// Static files — serve with long-lived cache (1 year) per AI.md PART 9 caching rules.
	// Files are embedded at build time; content is immutable per build.
	r.Get("/static/css/accent.css", s.handleAccentCSS)
	staticFiles, _ := fs.Sub(staticFS, "static")
	r.Handle("/static/*", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// AI.md PART 6: static file caching is aggressive in production and
		// disabled in development so edited assets are visible immediately.
		headers := mode.GetCacheHeaders()
		w.Header().Set("Cache-Control", headers.CacheControl)
		if headers.Pragma != "" {
			w.Header().Set("Pragma", headers.Pragma)
		}
		if headers.Expires != "" {
			w.Header().Set("Expires", headers.Expires)
		}
		http.StripPrefix("/static/", http.FileServer(http.FS(staticFiles))).ServeHTTP(w, r)
	}))

	// Translation files — served from the same embedded locale JSON the
	// server and CLI use, per AI.md PART 30 "WebUI JavaScript" row.
	r.Get("/locales/{lang}.json", s.handleLocaleJSON)

	// Special files
	r.Get("/robots.txt", s.handleRobotsTxt)
	r.Get("/llms.txt", s.handleLLMSTxt)
	r.Get("/.well-known/security.txt", s.handleSecurityTxt)
	r.Get("/.well-known/llms.txt", s.handleLLMSTxt)
	r.Get("/.well-known/pgp-key.asc", s.handleSecurityPGPKey)
	r.Get("/manifest.webmanifest", s.handleManifest)
	r.Get("/sw.js", s.handleServiceWorker)

	// Web routes (HTML - All Public, NO AUTH)
	// Canonical paths per AI.md route table: /airports/search, /airports/nearby.
	r.Get("/", s.handleHome)
	r.Get("/airports", s.handleAirportList)
	r.Get("/airports/search", s.handleSearch)
	r.Get("/airports/nearby", s.handleNearby)
	r.Get("/airports/{ident}", s.handleAirportDetail)
	r.Get("/stats", s.handleStats)
	r.Get("/geoip", s.handleGeoIPPage)
	if s.config.Server.Healthz.Root.Enabled {
		r.Get("/healthz", s.handleServerHealthz) // optional root alias for /server/healthz
		r.Get("/healthz.txt", s.handleServerHealthz)
	}

	// /server/* pages (required by IDEA.md and AI.md spec)
	r.Get("/server/about", s.handleServerAbout)
	if s.config.Server.Metrics.Enabled {
		registerMetrics(r, "/server/metrics", "prometheus")
		for _, service := range []string{"prometheus", "grafana", "loki"} {
			registerMetrics(r, "/server/metrics/"+service, service)
		}
	}
	r.Get("/server/help", s.handleServerHelp)
	if s.helpPageEnabled("/server/help/mcp") {
		r.Get("/server/help/mcp", s.handleServerMCPHelp)
	}
	r.Get("/server/healthz", s.handleServerHealthz)
	r.Get("/server/healthz.txt", s.handleServerHealthz)
	r.Get("/server/privacy", s.handleServerPrivacy)
	r.Get("/server/preferences", s.handleServerPreferences)
	r.Post("/server/preferences", s.handleServerPreferencesSave)
	r.Get("/server/preferences/sync", s.handleServerPreferencesSync)
	r.Post("/server/preferences/sync", s.handleServerPreferencesSync)
	r.Get("/server/preferences/sync/import", s.handleServerPreferencesImport)
	r.Get("/server/preferences/tokens", s.handleOwnerTokensPage)
	r.Post("/server/preferences/token/create", s.handleOwnerTokenIssueWeb)
	r.Post("/server/preferences/token/restore", s.handleOwnerTokenRestoreWeb)
	r.Post("/server/preferences/token/forget", s.handleOwnerTokenForgetWeb)
	r.Post("/server/preferences/token/reveal", s.handleOwnerTokenPrimaryRevealWeb)
	r.Post("/server/preferences/token/rotate", s.handleOwnerTokenPrimaryRotateWeb)
	r.Post("/server/preferences/token/revoke", s.handleOwnerTokenPrimaryRevokeWeb)
	r.Post("/server/preferences/tokens", s.handleOwnerTokenCreateLinkedWeb)
	r.Post("/server/preferences/tokens/{id}/reveal", s.handleOwnerTokenLinkedActionWeb)
	r.Post("/server/preferences/tokens/{id}/rotate", s.handleOwnerTokenLinkedActionWeb)
	r.Post("/server/preferences/tokens/{id}/update", s.handleOwnerTokenLinkedActionWeb)
	r.Post("/server/preferences/tokens/{id}/revoke", s.handleOwnerTokenLinkedActionWeb)
	r.Get("/server/dpo", s.handleServerDPO)
	r.Get("/server/terms", s.handleServerTerms)
	r.Get("/server/contact", s.handleServerContact)
	r.Post("/server/contact", s.handleContactSubmission)
	r.Get("/server/security", s.handleServerSecurity)
	r.Get("/server/security/policy", s.handleServerSecurityPolicy)
	r.Get("/server/security/thanks", s.handleServerSecurityThanks)
	r.Get("/server/security/report/{tracking_id}", s.handleServerSecurityReportStatus)
	r.Post("/server/consent", s.handleServerConsent)
	r.Post("/server/ccpa", s.handleServerCCPA)
	// /server/tor/* — INTERNAL CLI-to-running-server control channel per
	// AI.md PART 31 "CLI-to-running-server control channel". Loopback-gated
	// (404 on any non-loopback peer), never versioned, never in Swagger/
	// GraphQL/well-known/FeaturesInfo — same tier as /server/metrics but
	// without a bearer token (there is no legitimate remote caller).
	r.Get("/server/tor/status", torLoopbackOnly(s.handleTorStatus))
	r.Post("/server/tor/validate", torLoopbackOnly(s.handleTorValidate))
	r.Post("/server/tor/restart", torLoopbackOnly(s.handleTorRestart))
	r.Post("/server/tor/regenerate", torLoopbackOnly(s.handleTorRegenerate))
	r.Post("/server/tor/vanity/start", torLoopbackOnly(s.handleTorVanityStart))
	r.Post("/server/tor/vanity/apply", torLoopbackOnly(s.handleTorVanityApply))
	r.Post("/server/tor/import-keys", torLoopbackOnly(s.handleTorImportKeys))
	r.Post("/announcements/dismiss", s.handleAnnouncementDismiss)
	r.Get("/sitemap.xml", s.handleSitemap)
	r.Get("/feed.xml", s.handleAtomFeed)
	r.Head("/feed.xml", s.handleAtomFeed)
	r.Get("/rss.xml", s.handleRSSFeed)
	r.Head("/rss.xml", s.handleRSSFeed)
	// Swagger UI + GraphiQL — handlers live in src/swagger and src/graphql
	// per AI.md PART 14 "Standardized File Locations". UI assets are
	// served from embed.FS in those packages (no CDN, single binary).
	apiBase := s.APIBasePath()
	requestBaseURL := func(r *http.Request) string {
		return strings.TrimSuffix(RequestBaseURL(r), "/")
	}
	swaggerSpec := swaggerpkg.SpecHandlerWithRoutes(Version, s.config.Server.APIVersion, s.openAPIRouteSuperset, requestBaseURL)
	graphqlQuery := graphqlpkg.QueryHandler(s.airports, s.geoip)

	r.Get("/server/docs/swagger", s.handleServerSwaggerUI)
	r.Get("/server/docs/graphql", s.handleServerGraphQLUI)
	r.Get("/server/docs/swagger/theme.css", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
		_, _ = w.Write([]byte(swaggerpkg.ThemeCSS()))
	})
	r.Get("/server/docs/graphql/theme.css", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
		_, _ = w.Write([]byte(graphqlpkg.ThemeCSS()))
	})
	r.Mount(swaggerAssetsPrefix, swaggerpkg.AssetsHandler(swaggerAssetsPrefix))
	r.Mount(graphqlAssetsPrefix, graphqlpkg.AssetsHandler(graphqlAssetsPrefix))

	// Unversioned API aliases (mounted to the same handlers as the current version,
	// per AI.md PART 14 "Unversioned API aliases" — never redirect, never fork behavior).
	r.Get("/api/swagger", swaggerSpec)
	r.Get("/api/healthz", s.handleServerHealthz)
	r.Get("/api/healthz.txt", s.handleServerHealthz)
	r.Post("/api/graphql", graphqlQuery)
	// AI.md note: "Old paths removed: /openapi, /openapi.json, /graphql (GET and POST
	// at root) are no longer served." The deprecated legacy aliases have therefore
	// been deleted from both the root and the versioned tree.

	// /api/autodiscover — unversioned, per AI.md PART 14. Returns server settings,
	// config schema, and options for CLI/agent consumers.
	r.Get("/api/autodiscover", s.handleAutodiscover)
	if s.config.Server.Metrics.Enabled {
		for _, service := range []string{"prometheus", "grafana", "loki"} {
			registerMetrics(r, "/api/metrics/"+service, service)
		}
		registerMetrics(r, "/api/metrics", "prometheus")
	}

	// Versioned API routes - ALL PUBLIC, NO AUTH
	r.Route(apiBase, func(r chi.Router) {
		// Add API version + no-cache headers to all versioned responses per AI.md PART 9.
		r.Use(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("X-API-Version", s.config.Server.APIVersion)
				w.Header().Set("Cache-Control", "private, max-age=0, no-cache")
				next.ServeHTTP(w, r)
			})
		})
		r.Post("/server/mcp", s.handleMCP)

		// API info
		r.Get("/", s.handleAPIInfo)

		// Versioned server API surface (canonical paths per AI.md PART 14).
		r.Get("/server/healthz", s.handleServerHealthz)
		r.Get("/server/healthz.txt", s.handleServerHealthz)
		r.Get("/server/auth/check", s.handleAPITokenCheck)
		if s.config.Server.Metrics.Enabled {
			for _, service := range []string{"prometheus", "grafana", "loki"} {
				registerMetrics(r, "/server/metrics/"+service, service)
			}
			registerMetrics(r, "/server/metrics", "prometheus")
		}
		r.Get("/server/about", s.handleServerAboutAPI)
		r.Get("/server/help", s.handleServerHelpAPI)
		if s.helpPageEnabled("/server/help/mcp") {
			r.Get("/server/help/mcp", s.handleServerMCPHelpAPI)
		}
		r.Get("/server/privacy", s.handleServerPrivacyAPI)
		r.Get("/server/preferences", s.handleServerPreferencesAPI)
		r.Post("/server/preferences", s.handleServerPreferencesSaveAPI)
		r.Get("/server/preferences/sync", s.handleServerPreferencesSyncAPI)
		r.Get("/server/preferences/sync/import", s.handleServerPreferencesImportAPI)
		r.Get("/server/preferences/tokens", s.handleOwnerTokenListAPI)
		r.Post("/server/preferences/tokens/primary", s.handleOwnerTokenIssueAPI)
		r.Post("/server/preferences/tokens/primary/reveal", s.handleOwnerTokenPrimaryRevealAPI)
		r.Post("/server/preferences/tokens/primary/rotate", s.handleOwnerTokenPrimaryRotateAPI)
		r.Delete("/server/preferences/tokens/primary", s.handleOwnerTokenPrimaryRevokeAPI)
		r.Post("/server/preferences/tokens", s.handleOwnerTokenCreateLinkedAPI)
		r.Post("/server/preferences/tokens/{id}/reveal", s.handleOwnerTokenLinkedActionAPI)
		r.Post("/server/preferences/tokens/{id}/rotate", s.handleOwnerTokenLinkedActionAPI)
		r.Patch("/server/preferences/tokens/{id}", s.handleOwnerTokenLinkedActionAPI)
		r.Delete("/server/preferences/tokens/{id}", s.handleOwnerTokenLinkedActionAPI)
		r.Get("/server/dpo", s.handleServerDPOAPI)
		r.Get("/server/terms", s.handleServerTermsAPI)
		r.Get("/server/contact", s.handleServerContactAPI)
		r.Get("/server/security", s.handleServerSecurityAPI)
		r.Get("/server/security/policy", s.handleServerSecurityPolicyAPI)
		r.Get("/server/security/thanks", s.handleServerSecurityThanksAPI)
		r.Post("/server/contact", s.handleContactSubmission)
		r.Get("/server/swagger", swaggerSpec)
		r.Post("/server/graphql", graphqlQuery)

		// Airport endpoints - JSON responses
		r.Get("/airports", s.handleGetAirports)
		r.Get("/airports.json", s.handleGetAirportsJSON)
		r.Get("/airports.csv", s.handleGetAirportsCSV)
		r.Get("/airports.geojson", s.handleGetAirportsGeoJSON)

		// Airport by ident - with .txt extension support
		r.Get("/airports/{ident}", s.handleGetAirportByIdent)
		r.Get("/airports/{ident}.txt", s.handleGetAirportByIdentText)

		// Search / nearby / within — canonical paths per AI.md route table.
		r.Get("/airports/search", s.handleSearchAirports)
		r.Get("/airports/search.txt", s.handleSearchAirportsText)
		r.Get("/airports/nearby", s.handleNearbyAirports)
		r.Get("/airports/nearby.txt", s.handleNearbyAirportsText)
		r.Get("/airports/within", s.handleBBoxAirports)
		r.Get("/airports/autocomplete", s.handleAutocomplete)

		// Countries and states
		r.Get("/countries", s.handleGetCountries)
		r.Get("/countries.txt", s.handleGetCountriesText)
		r.Get("/states/{country}", s.handleGetStates)

		// Statistics
		r.Get("/stats", s.handleAirportStats)
		r.Get("/stats.txt", s.handleAirportStatsText)

		// GeoIP endpoints
		r.Get("/geoip", s.handleGeoIPLookup)
		r.Get("/geoip.txt", s.handleGeoIPLookupText)
		r.Get("/geoip/{ip}", s.handleGeoIPLookupIP)
		r.Get("/geoip/{ip}.txt", s.handleGeoIPLookupIPText)
		r.Get("/geoip/airports/nearby", s.handleGeoIPNearbyAirports)

		// Settings (read-only, no auth needed)
		r.Get("/settings", s.handleGetSettings)
	})

	// Metrics endpoint — only mounted when server.metrics.enabled is true
	// (AI.md PART 20). The /server/metrics alias above serves browsers; this
	// one is for Prometheus scrapers that follow the standard convention.
	// Internal-only regardless of the optional bearer token below — deployments
	// are still expected to firewall this path/port from public access. When
	// disabled, the path is unregistered and falls through to the themed 404.
	if s.config.Server.Metrics.Enabled && s.config.Server.Metrics.Root.Enabled {
		registerMetrics(r, "/metrics", "prometheus")
		for _, service := range []string{"prometheus", "grafana", "loki"} {
			registerMetrics(r, "/metrics/"+service, service)
		}
	}

	// Debug endpoints — only mounted when --debug/DEBUG=true, per AI.md PART 6.
	s.registerDebugRoutes(r)

	// Themed error pages for unmatched routes and methods (AI.md PART 16 —
	// no generic/unstyled browser error pages). Content-negotiated: JSON for
	// API/CLI clients, themed HTML for browsers, plain text otherwise. Chi
	// runs these through the router middleware chain, so the resolved language
	// and request id are available.
	r.NotFound(func(w http.ResponseWriter, req *http.Request) {
		s.renderErrorPage(w, req, http.StatusNotFound)
	})
	r.MethodNotAllowed(func(w http.ResponseWriter, req *http.Request) {
		s.renderErrorPage(w, req, http.StatusMethodNotAllowed)
	})

}

func (s *Server) openAPIRoutes() []swaggerpkg.OpenAPIRoute {
	if s == nil || s.router == nil {
		return nil
	}
	apiBase := s.APIBasePath()
	var routes []swaggerpkg.OpenAPIRoute
	_ = chi.Walk(s.router, func(method, path string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		if path != apiBase && !strings.HasPrefix(path, apiBase+"/") {
			return nil
		}
		path = strings.TrimPrefix(path, apiBase)
		if path == "" {
			path = "/"
		}
		auth := strings.Contains(path, "/metrics") || strings.HasPrefix(path, "/server/mcp") || path == "/server/auth/check"
		if strings.HasPrefix(path, "/server/preferences/tokens") && (method != http.MethodPost || path != "/server/preferences/tokens/primary") {
			auth = true
		}
		route := swaggerpkg.OpenAPIRoute{Method: method, Path: path, Summary: method + " " + path, Auth: auth}
		if strings.Contains(path, "/metrics") {
			route.MountedWhen = "server.metrics.enabled"
			route.Mounted = s.config.Server.Metrics.Enabled
		}
		if strings.HasPrefix(path, "/debug/") {
			route.MountedWhen = "--debug"
			route.Mounted = mode.ShouldShowDebugEndpoints()
			if strings.HasPrefix(path, "/debug/pprof/") {
				route.MountedWhen = "--debug && server.debug.pprof"
				route.Mounted = route.Mounted && s.config.Server.Debug.Pprof
			} else if path == "/debug/vars" {
				route.MountedWhen = "--debug && server.debug.runtime_endpoints"
				route.Mounted = route.Mounted && s.config.Server.Debug.RuntimeEndpoints
			}
			route.Auth = true
		}
		routes = append(routes, route)
		return nil
	})
	return routes
}

func apiResponseHeaders(apiVersion string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-API-Version", apiVersion)
		w.Header().Set("Cache-Control", "private, max-age=0, no-cache")
		next(w, r)
	}
}

func (s *Server) openAPIRouteSuperset() []swaggerpkg.OpenAPIRoute {
	routes := s.openAPIRoutes()
	// Root aliases and well-known resources are part of the embedded superset
	// even though they are outside the versioned API namespace.
	optionalRoot := []struct {
		path, condition string
		mounted         bool
	}{
		{"/healthz", "server.healthz.root.enabled", s.config.Server.Healthz.Root.Enabled},
		{"/metrics", "server.metrics.root.enabled", s.config.Server.Metrics.Enabled && s.config.Server.Metrics.Root.Enabled},
		{"/metrics/prometheus", "server.metrics.root.enabled && server.metrics.enabled", s.config.Server.Metrics.Enabled && s.config.Server.Metrics.Root.Enabled},
		{"/metrics/grafana", "server.metrics.root.enabled && server.metrics.enabled", s.config.Server.Metrics.Enabled && s.config.Server.Metrics.Root.Enabled},
		{"/metrics/loki", "server.metrics.root.enabled && server.metrics.enabled", s.config.Server.Metrics.Enabled && s.config.Server.Metrics.Root.Enabled},
		{"/llms.txt", "always", true},
		{"/.well-known/security.txt", "always", true},
		{"/.well-known/llms.txt", "always", true},
		{"/.well-known/pgp-key.asc", "security.publish_pgp_key", s.config.Web.Security.PublishPGPKey},
	}
	for _, endpoint := range optionalRoot {
		mountedWhen := endpoint.condition
		if mountedWhen == "always" {
			mountedWhen = ""
		}
		routes = append(routes, swaggerpkg.OpenAPIRoute{
			Method: http.MethodGet, Path: endpoint.path, Summary: http.MethodGet + " " + endpoint.path,
			MountedWhen: mountedWhen, Mounted: endpoint.mounted,
		})
	}
	aliases := []swaggerpkg.OpenAPIRoute{
		{Method: http.MethodGet, Path: "/api/swagger", Summary: "GET /api/swagger"},
		{Method: http.MethodGet, Path: "/api/healthz", Summary: "GET /api/healthz"},
		{Method: http.MethodGet, Path: "/api/healthz.txt", Summary: "GET /api/healthz.txt"},
		{Method: http.MethodPost, Path: "/api/graphql", Summary: "POST /api/graphql"},
		{Method: http.MethodPost, Path: "/api/mcp", Summary: "POST /api/mcp", Auth: true},
		{Method: http.MethodPost, Path: "/mcp", Summary: "POST /mcp", Auth: true},
		{Method: http.MethodGet, Path: "/api/autodiscover", Summary: "GET /api/autodiscover"},
	}
	routes = append(routes, aliases...)
	for _, service := range []string{"", "/prometheus", "/grafana", "/loki"} {
		path := "/api/metrics" + service
		routes = append(routes, swaggerpkg.OpenAPIRoute{
			Method: http.MethodGet, Path: path, Summary: http.MethodGet + " " + path,
			Auth: true, MountedWhen: "server.metrics.enabled", Mounted: s.config.Server.Metrics.Enabled,
		})
	}
	metricsMounted := s.config.Server.Metrics.Enabled
	for _, path := range []string{"/server/metrics", "/server/metrics/prometheus", "/server/metrics/grafana", "/server/metrics/loki"} {
		found := false
		for _, route := range routes {
			if route.Method == http.MethodGet && route.Path == path {
				found = true
				break
			}
		}
		if !found {
			routes = append(routes, swaggerpkg.OpenAPIRoute{
				Method: http.MethodGet, Path: path, Summary: http.MethodGet + " " + path,
				Auth: true, MountedWhen: "server.metrics.enabled", Mounted: metricsMounted,
			})
		}
	}
	debugMounted := mode.ShouldShowDebugEndpoints()
	debugPaths := []string{
		"/debug/config", "/debug/routes", "/debug/cache", "/debug/db",
		"/debug/scheduler", "/debug/memory", "/debug/goroutines",
		"/debug/pprof/", "/debug/pprof/cmdline", "/debug/pprof/profile", "/debug/pprof/symbol",
		"/debug/pprof/trace", "/debug/pprof/heap", "/debug/pprof/goroutine", "/debug/pprof/allocs",
		"/debug/pprof/block", "/debug/pprof/mutex", "/debug/pprof/threadcreate", "/debug/vars",
	}
	for _, path := range debugPaths {
		apiPath := "/api" + path
		condition := "--debug"
		mounted := debugMounted
		if strings.HasPrefix(path, "/debug/pprof/") {
			condition = "--debug && server.debug.pprof"
			mounted = mounted && s.config.Server.Debug.Pprof
		} else if path == "/debug/vars" {
			condition = "--debug && server.debug.runtime_endpoints"
			mounted = mounted && s.config.Server.Debug.RuntimeEndpoints
		}
		routes = append(routes, swaggerpkg.OpenAPIRoute{
			Method: http.MethodGet, Path: apiPath, Summary: http.MethodGet + " " + apiPath,
			Auth: true, MountedWhen: condition, Mounted: mounted,
		})
	}
	for _, path := range debugPaths {
		found := false
		for _, route := range routes {
			if route.Method == http.MethodGet && route.Path == path {
				found = true
				break
			}
		}
		if !found {
			condition := "--debug"
			mounted := debugMounted
			if strings.HasPrefix(path, "/debug/pprof/") {
				condition = "--debug && server.debug.pprof"
				mounted = mounted && s.config.Server.Debug.Pprof
			} else if path == "/debug/vars" {
				condition = "--debug && server.debug.runtime_endpoints"
				mounted = mounted && s.config.Server.Debug.RuntimeEndpoints
			}
			routes = append(routes, swaggerpkg.OpenAPIRoute{
				Method: http.MethodGet, Path: path, Summary: http.MethodGet + " " + path,
				Auth: true, MountedWhen: condition, Mounted: mounted,
			})
		}
	}
	return routes
}

func OpenAPIRouteSupersetForBuild() []swaggerpkg.OpenAPIRoute {
	wasDebugEnabled := mode.IsDebugEnabled()
	mode.SetDebugEnabled(true)
	defer mode.SetDebugEnabled(wasDebugEnabled)
	cfg := config.DefaultApplicationConfig()
	cfg.Server.Metrics.Enabled = true
	cfg.Server.Debug.Pprof = true
	cfg.Server.Debug.RuntimeEndpoints = true
	router := chi.NewRouter()
	server := &Server{config: cfg, router: router}
	server.registerRoutes(router, func(routeRouter chi.Router, path, _ string) {
		routeRouter.Get(path, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	})
	return server.openAPIRouteSuperset()
}
