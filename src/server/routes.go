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
	registry := &routeRegistry{router: r}
	s.routeRegistry = registry
	registerMetrics := func(_ chi.Router, path, service string) {
		metricsHandler := metricsAuthMiddleware(
			metricsServiceToken(s.config.Server.Metrics, service),
			s.config.Server.Metrics.Auth.AllowUnauthenticated,
			s.metrics,
			s.metrics.serviceHandler(service),
		)
		handler := http.HandlerFunc(metricsHandler.ServeHTTP)
		if strings.HasPrefix(path, s.APIBasePath()+"/") {
			handler = apiResponseHeaders(s.config.Server.APIVersion, handler)
		}
		mountedWhen := "server.metrics.enabled"
		mounted := s.config.Server.Metrics.Enabled
		if path == "/metrics" || strings.HasPrefix(path, "/metrics/") {
			mountedWhen = "server.metrics.enabled && server.metrics.root.enabled"
			mounted = mounted && s.config.Server.Metrics.Root.Enabled
		}
		registry.Add(Route{
			Method: http.MethodGet, Path: path, Summary: http.MethodGet + " " + path,
			Auth: "service", Listed: false, Internal: true, Handler: handler,
			MountedWhen: mountedWhen, Mounted: mounted,
		})
	}
	s.registerMiddleware(r)
	s.registerRoutes(r, registerMetrics)
	s.router = r
}

// registerRoutes mounts the public web, API, metrics, debug, and error routes.
func (s *Server) registerRoutes(r chi.Router, registerMetrics func(chi.Router, string, string)) {
	if s.routeRegistry == nil {
		s.routeRegistry = &routeRegistry{router: r}
	}
	registry := s.routeRegistry
	apiBase := s.APIBasePath()
	apiVersion := s.config.Server.APIVersion
	registerVersioned := func(method, path, auth string, handler http.HandlerFunc, aliases ...string) {
		fullPath := apiBase + path
		operationID, summary, requestModel, responseModel := swaggerpkg.CanonicalRouteModels(Version, apiVersion, "en", method, path)
		if operationID == "" {
			operationID = routeOperationID(method, path)
		}
		aliasIDs := make(map[string]string, len(aliases))
		for _, alias := range aliases {
			aliasIDs[alias] = routeOperationID(method, alias)
		}
		binaryType := ""
		switch path {
		case "/airports.json":
			binaryType = "application/json"
		case "/airports.csv":
			binaryType = "text/csv"
		case "/airports.geojson":
			binaryType = "application/geo+json"
		}
		registry.Add(Route{
			Method: method, Path: fullPath, Aliases: aliases, AliasOperationIDs: aliasIDs,
			OperationID: operationID, Summary: summary, Auth: auth,
			Request: requestModel, Response: responseModel, Binary: binaryType,
			Listed: auth != "operator" && auth != "service", Handler: apiResponseHeaders(apiVersion, handler),
		})
	}
	r.Get("/cli/binaries/{binary}", s.handleCLIBinaryDownload)
	if s.brandingAssets != nil {
		r.Handle("/branding-assets/*", s.brandingAssets.Handler())
	}

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
	r.Method(http.MethodHead, "/robots.txt", headOnlyHandler(s.handleRobotsTxt))
	registry.Add(Route{Method: http.MethodGet, Path: "/llms.txt", OperationID: "getLlmsTxt", Summary: "GET /llms.txt", Auth: "none", Listed: true, Handler: s.handleLLMSTxt})
	registry.Add(Route{Method: http.MethodGet, Path: "/.well-known/llms.txt", OperationID: "getWellKnownLlmsTxt", Summary: "GET /.well-known/llms.txt", Auth: "none", Listed: true, Handler: s.handleLLMSTxt})
	registry.Add(Route{Method: http.MethodGet, Path: "/.well-known/security.txt", OperationID: "getSecurityTxt", Summary: "GET /.well-known/security.txt", Auth: "none", Listed: true, Handler: s.handleSecurityTxt})
	registry.Add(Route{
		Method: http.MethodGet, Path: "/.well-known/pgp-key.asc", OperationID: "getPGPKey", Summary: "GET /.well-known/pgp-key.asc", Auth: "none", Listed: true, Handler: s.handleSecurityPGPKey,
		MountedWhen: "security keypair exists", Mounted: true,
	})
	r.Get("/manifest.json", s.handleManifest)
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
	registry.Add(Route{
		Method: http.MethodGet, Path: "/healthz", OperationID: "getRootHealthz", Summary: "GET /healthz", Auth: "none", Listed: true,
		Handler: s.handleServerHealthz, MountedWhen: "server.healthz.root.enabled", Mounted: s.config.Server.Healthz.Root.Enabled,
	})

	// /server/* pages (required by IDEA.md and AI.md spec)
	r.Get("/server/about", s.handleServerAbout)
	r.Get("/server/help", s.handleServerHelp)
	if s.helpPageEnabled("/server/help/mcp") {
		r.Get("/server/help/mcp", s.handleServerMCPHelp)
	}
	r.Get("/server/healthz", s.handleServerHealthz)
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
	registry.Add(Route{Method: http.MethodGet, Path: "/server/tor/status", Auth: "none", Internal: true, Handler: torLoopbackOnly(s.handleTorStatus)})
	registry.Add(Route{Method: http.MethodPost, Path: "/server/tor/validate", Auth: "none", Internal: true, Handler: torLoopbackOnly(s.handleTorValidate)})
	registry.Add(Route{Method: http.MethodPost, Path: "/server/tor/restart", Auth: "none", Internal: true, Handler: torLoopbackOnly(s.handleTorRestart)})
	registry.Add(Route{Method: http.MethodPost, Path: "/server/tor/regenerate", Auth: "none", Internal: true, Handler: torLoopbackOnly(s.handleTorRegenerate)})
	registry.Add(Route{Method: http.MethodPost, Path: "/server/tor/vanity/start", Auth: "none", Internal: true, Handler: torLoopbackOnly(s.handleTorVanityStart)})
	registry.Add(Route{Method: http.MethodPost, Path: "/server/tor/vanity/apply", Auth: "none", Internal: true, Handler: torLoopbackOnly(s.handleTorVanityApply)})
	registry.Add(Route{Method: http.MethodPost, Path: "/server/tor/import-keys", Auth: "none", Internal: true, Handler: torLoopbackOnly(s.handleTorImportKeys)})
	r.Post("/announcements/dismiss", s.handleAnnouncementDismiss)
	r.Get("/sitemap.xml", s.handleSitemap)
	r.Get("/feed.xml", s.handleAtomFeed)
	r.Head("/feed.xml", s.handleAtomFeed)
	r.Get("/rss.xml", s.handleRSSFeed)
	r.Head("/rss.xml", s.handleRSSFeed)
	// Swagger UI + GraphiQL handlers and themes live in src/swagger and
	// src/graphql. Their pinned UI assets use the server's embedded static
	// tree under /static/vendor/ and load without a CDN.
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

	// Unversioned API aliases are mounted from the same route definitions as their canonical endpoints.
	// AI.md note: "Old paths removed: /openapi, /openapi.json, /graphql (GET and POST
	// at root) are no longer served." The deprecated legacy aliases have therefore
	// been deleted from both the root and the versioned tree.

	// /api/autodiscover — unversioned, per AI.md PART 14. Returns server settings,
	// config schema, and options for CLI/agent consumers.
	operationID, summary, requestModel, responseModel := swaggerpkg.CanonicalRouteModels(Version, apiVersion, "en", http.MethodGet, "/api/autodiscover")
	if operationID == "" {
		operationID = "getApiAutodiscover"
	}
	registry.Add(Route{
		Method: http.MethodGet, Path: "/api/autodiscover", OperationID: operationID, Summary: summary, Auth: "none", Listed: true,
		Request: requestModel, Response: responseModel, Handler: s.handleAutodiscover,
	})

	// API aliases and versioned operations are registered from shared Route definitions.
	registerVersioned(http.MethodGet, "/server/swagger", "none", swaggerSpec, "/api/swagger")
	registerVersioned(http.MethodGet, "/server/healthz", "none", s.handleServerHealthz, "/api/healthz")
	registerVersioned(http.MethodGet, "/server/healthz.txt", "none", s.handleServerHealthz)
	registerVersioned(http.MethodGet, "/server/graphql", "none", graphqlQuery, "/api/graphql")
	registerVersioned(http.MethodPost, "/server/graphql", "none", graphqlQuery, "/api/graphql")
	registerVersioned(http.MethodPost, "/server/mcp", "token", s.handleMCP, "/api/mcp", "/mcp")
	registerVersioned(http.MethodGet, "/", "none", s.handleAPIInfo)
	registerVersioned(http.MethodGet, "/server/auth/check", "none", s.handleAPITokenCheck)
	registerVersioned(http.MethodGet, "/server/about", "none", s.handleServerAboutAPI)
	registerVersioned(http.MethodGet, "/server/help", "none", s.handleServerHelpAPI)
	if s.helpPageEnabled("/server/help/mcp") {
		registerVersioned(http.MethodGet, "/server/help/mcp", "none", s.handleServerMCPHelpAPI)
	}
	registerVersioned(http.MethodGet, "/server/privacy", "none", s.handleServerPrivacyAPI)
	registerVersioned(http.MethodGet, "/server/preferences", "none", s.handleServerPreferencesAPI)
	registerVersioned(http.MethodPost, "/server/preferences", "none", s.handleServerPreferencesSaveAPI)
	registerVersioned(http.MethodGet, "/server/preferences/sync", "none", s.handleServerPreferencesSyncAPI)
	registerVersioned(http.MethodGet, "/server/preferences/sync/import", "none", s.handleServerPreferencesImportAPI)
	registerVersioned(http.MethodGet, "/server/preferences/tokens", "owner", s.handleOwnerTokenListAPI)
	registerVersioned(http.MethodPost, "/server/preferences/tokens/primary", "none", s.handleOwnerTokenIssueAPI)
	registerVersioned(http.MethodPost, "/server/preferences/tokens/primary/reveal", "owner", s.handleOwnerTokenPrimaryRevealAPI)
	registerVersioned(http.MethodPost, "/server/preferences/tokens/primary/rotate", "owner", s.handleOwnerTokenPrimaryRotateAPI)
	registerVersioned(http.MethodDelete, "/server/preferences/tokens/primary", "owner", s.handleOwnerTokenPrimaryRevokeAPI)
	registerVersioned(http.MethodPost, "/server/preferences/tokens", "owner", s.handleOwnerTokenCreateLinkedAPI)
	registerVersioned(http.MethodPost, "/server/preferences/tokens/{id}/reveal", "owner", s.handleOwnerTokenLinkedActionAPI)
	registerVersioned(http.MethodPost, "/server/preferences/tokens/{id}/rotate", "owner", s.handleOwnerTokenLinkedActionAPI)
	registerVersioned(http.MethodPatch, "/server/preferences/tokens/{id}", "owner", s.handleOwnerTokenLinkedActionAPI)
	registerVersioned(http.MethodDelete, "/server/preferences/tokens/{id}", "owner", s.handleOwnerTokenLinkedActionAPI)
	registerVersioned(http.MethodGet, "/server/dpo", "none", s.handleServerDPOAPI)
	registerVersioned(http.MethodGet, "/server/terms", "none", s.handleServerTermsAPI)
	registerVersioned(http.MethodGet, "/server/contact", "none", s.handleServerContactAPI)
	registerVersioned(http.MethodGet, "/server/security", "none", s.handleServerSecurityAPI)
	registerVersioned(http.MethodGet, "/server/security/policy", "none", s.handleServerSecurityPolicyAPI)
	registerVersioned(http.MethodGet, "/server/security/thanks", "none", s.handleServerSecurityThanksAPI)
	registerVersioned(http.MethodPost, "/server/contact", "none", s.handleContactSubmission)
	registerVersioned(http.MethodGet, "/airports", "none", s.handleGetAirports)
	registerVersioned(http.MethodGet, "/airports.json", "none", s.handleGetAirportsJSON)
	registerVersioned(http.MethodGet, "/airports.csv", "none", s.handleGetAirportsCSV)
	registerVersioned(http.MethodGet, "/airports.geojson", "none", s.handleGetAirportsGeoJSON)
	registerVersioned(http.MethodGet, "/airports/{ident}", "none", s.handleGetAirportByIdent)
	registerVersioned(http.MethodGet, "/airports/{ident}.txt", "none", s.handleGetAirportByIdent)
	registerVersioned(http.MethodGet, "/airports/search", "none", s.handleSearchAirports)
	registerVersioned(http.MethodGet, "/airports/search.txt", "none", s.handleSearchAirports)
	registerVersioned(http.MethodGet, "/airports/nearby", "none", s.handleNearbyAirports)
	registerVersioned(http.MethodGet, "/airports/nearby.txt", "none", s.handleNearbyAirports)
	registerVersioned(http.MethodGet, "/airports/within", "none", s.handleBBoxAirports)
	registerVersioned(http.MethodGet, "/airports/autocomplete", "none", s.handleAutocomplete)
	registerVersioned(http.MethodGet, "/countries", "none", s.handleGetCountries)
	registerVersioned(http.MethodGet, "/countries.txt", "none", s.handleGetCountries)
	registerVersioned(http.MethodGet, "/states/{country}", "none", s.handleGetStates)
	registerVersioned(http.MethodGet, "/stats", "none", s.handleAirportStats)
	registerVersioned(http.MethodGet, "/stats.txt", "none", s.handleAirportStats)
	registerVersioned(http.MethodGet, "/geoip", "none", s.handleGeoIPLookup)
	registerVersioned(http.MethodGet, "/geoip.txt", "none", s.handleGeoIPLookup)
	registerVersioned(http.MethodGet, "/geoip/{ip}", "none", s.handleGeoIPLookupIP)
	registerVersioned(http.MethodGet, "/geoip/{ip}.txt", "none", s.handleGeoIPLookupIP)
	registerVersioned(http.MethodGet, "/geoip/airports/nearby", "none", s.handleGeoIPNearbyAirports)
	registerVersioned(http.MethodGet, "/settings", "none", s.handleGetSettings)

	for _, metricsPath := range []string{"/server/metrics", "/server/metrics/prometheus", "/server/metrics/grafana", "/server/metrics/loki"} {
		service := strings.TrimPrefix(metricsPath, "/server/metrics")
		service = strings.TrimPrefix(service, "/")
		if service == "" {
			service = "prometheus"
		}
		registerMetrics(r, metricsPath, service)
		registerMetrics(r, apiBase+metricsPath, service)
	}
	for _, service := range []string{"prometheus", "grafana", "loki"} {
		registerMetrics(r, "/api/metrics/"+service, service)
		registerMetrics(r, "/metrics/"+service, service)
	}
	registerMetrics(r, "/api/metrics", "prometheus")
	registerMetrics(r, "/metrics", "prometheus")

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
	if s == nil || s.routeRegistry == nil {
		return nil
	}
	return s.currentRouteRegistry().OpenAPIRoutes(s.APIBasePath())
}

// currentRouteRegistry refreshes runtime-dependent mount state before deriving
// OpenAPI and MCP surfaces from the shared route inventory.
func (s *Server) currentRouteRegistry() *routeRegistry {
	if s == nil || s.routeRegistry == nil {
		return nil
	}
	routes := s.routeRegistry.Snapshot()
	for i := range routes {
		if routes[i].MountedWhen == "security keypair exists" {
			routes[i].Mounted = s.hasPGPKeypair()
		}
	}
	return &routeRegistry{routes: routes}
}

func apiResponseHeaders(apiVersion string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-API-Version", apiVersion)
		w.Header().Set("Cache-Control", "private, max-age=0, no-cache")
		next(w, r)
	}
}

func (s *Server) openAPIRouteSuperset() []swaggerpkg.OpenAPIRoute {
	return s.openAPIRoutes()
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
	server.registerRoutes(router, func(_ chi.Router, path, _ string) {
		handler := func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNotFound) }
		server.routeRegistry.Add(Route{
			Method: http.MethodGet, Path: path, Summary: http.MethodGet + " " + path,
			Auth: "service", Handler: handler, MountedWhen: "server.metrics.enabled", Mounted: cfg.Server.Metrics.Enabled,
		})
	})
	return server.openAPIRouteSuperset()
}
