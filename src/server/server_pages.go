package server

import (
	"net/http"
	"runtime"

	"github.com/apimgr/airports/src/common/i18n"
)

// Server pages required by IDEA.md and AI.md spec:
//   /server/about    - About page sourced from IDEA.md project description
//   /server/help     - Help / API reference
//   /server/healthz  - Health check (HTML/JSON/text via content negotiation)
//   /server/privacy  - Privacy policy
//   /server/terms    - Terms of use
//   /server/docs/swagger  - Swagger UI (interactive)
//   /server/docs/graphql  - GraphiQL UI (interactive)

// handleServerAbout renders the About page using IDEA.md-derived content.
func (s *Server) handleServerAbout(w http.ResponseWriter, r *http.Request) {
	lang := i18n.FromContext(r.Context())
	s.renderTemplate(w, r, "server_about", map[string]interface{}{
		"Title":       "About",
		"Theme":       s.config.Web.UI.Theme,
		"Tagline":     i18n.T(lang, "about.tagline"),
		"Description": i18n.T(lang, "about.description"),
		"Features": []string{
			i18n.T(lang, "about.feature_rest"),
			i18n.T(lang, "about.feature_graphql"),
			i18n.T(lang, "about.feature_swagger"),
			i18n.T(lang, "about.feature_geoip"),
			i18n.T(lang, "about.feature_geo_queries"),
			i18n.T(lang, "about.feature_webui"),
			i18n.T(lang, "about.feature_cli"),
			i18n.T(lang, "about.feature_binary"),
		},
		"Repo":    "https://github.com/apimgr/airports",
		"License": "MIT",
		"Version": Version,
	})
}

// handleServerHelp renders the Help / API reference page.
func (s *Server) handleServerHelp(w http.ResponseWriter, r *http.Request) {
	s.renderTemplate(w, r, "server_help", map[string]interface{}{
		"Title": "Help",
		"Theme": s.config.Web.UI.Theme,
	})
}

// handleServerPrivacy renders the Privacy page.
func (s *Server) handleServerPrivacy(w http.ResponseWriter, r *http.Request) {
	s.renderTemplate(w, r, "server_privacy", map[string]interface{}{
		"Title": "Privacy",
		"Theme": s.config.Web.UI.Theme,
	})
}

// handleServerTerms renders the Terms of Use page.
func (s *Server) handleServerTerms(w http.ResponseWriter, r *http.Request) {
	s.renderTemplate(w, r, "server_terms", map[string]interface{}{
		"Title": "Terms",
		"Theme": s.config.Web.UI.Theme,
	})
}

// contactAddresses resolves the display-only contact addresses for
// /server/contact, falling back to WebSecurity.Admin (the security.txt
// contact) when server.privacy.contact.general/abuse are unset — this
// project has no SMTP subsystem, so the page is read-only mailto: links
// rather than a submission form (features-rules.md: never queue email
// without a tested SMTP connection).
func (s *Server) contactAddresses() (general, abuse string) {
	general = s.config.Server.Privacy.Contact.General
	abuse = s.config.Server.Privacy.Contact.Abuse
	fallback := s.config.Web.Security.Admin
	if fallback == "" {
		fallback = "security@apimgr.us"
	}
	if general == "" {
		general = fallback
	}
	if abuse == "" {
		abuse = fallback
	}
	return general, abuse
}

// handleServerContact renders the Contact page (display-only mailto links).
func (s *Server) handleServerContact(w http.ResponseWriter, r *http.Request) {
	general, abuse := s.contactAddresses()
	s.renderTemplate(w, r, "server_contact", map[string]interface{}{
		"Title":        "Contact",
		"Theme":        s.config.Web.UI.Theme,
		"GeneralEmail": general,
		"AbuseEmail":   abuse,
	})
}

// handleServerContactAPI returns contact addresses as JSON per AI.md PART 14.
func (s *Server) handleServerContactAPI(w http.ResponseWriter, r *http.Request) {
	general, abuse := s.contactAddresses()
	s.respondItem(w, http.StatusOK, map[string]interface{}{
		"general": general,
		"abuse":   abuse,
		"security": map[string]string{
			"info_url": "/security.txt",
		},
		"repo": "https://github.com/apimgr/airports",
	})
}

// handleServerHelpAPI returns the API help reference as JSON per AI.md PART 14.
func (s *Server) handleServerHelpAPI(w http.ResponseWriter, r *http.Request) {
	lang := i18n.FromContext(r.Context())
	base := s.APIBasePath()
	s.respondItem(w, http.StatusOK, map[string]interface{}{
		"name":    i18n.T(lang, "help.api_name"),
		"version": s.config.Server.APIVersion,
		"endpoints": []map[string]string{
			{"method": "GET", "path": base + "/airports", "description": i18n.T(lang, "help.ep_list_desc")},
			{"method": "GET", "path": base + "/airports/{ident}", "description": i18n.T(lang, "help.ep_single_desc")},
			{"method": "GET", "path": base + "/airports/search?q=", "description": i18n.T(lang, "help.ep_search_desc")},
			{"method": "GET", "path": base + "/airports/nearby?lat=&lon=&radius=", "description": i18n.T(lang, "help.ep_nearby_desc")},
			{"method": "GET", "path": base + "/airports/within?lat_min=&lat_max=&lon_min=&lon_max=", "description": i18n.T(lang, "help.ep_bbox_desc")},
			{"method": "GET", "path": base + "/airports/autocomplete?q=", "description": i18n.T(lang, "help.ep_autocomplete_desc")},
			{"method": "GET", "path": base + "/countries", "description": i18n.T(lang, "help.ep_countries_desc")},
			{"method": "GET", "path": base + "/stats", "description": i18n.T(lang, "help.ep_stats_desc")},
			{"method": "GET", "path": base + "/geoip", "description": i18n.T(lang, "help.ep_geoip_desc")},
			{"method": "GET", "path": base + "/geoip/{ip}", "description": i18n.T(lang, "help.ep_geoip_ip_desc")},
			{"method": "GET", "path": base + "/server/healthz", "description": i18n.T(lang, "help.ep_healthz_desc")},
			{"method": "GET", "path": base + "/server/about", "description": i18n.T(lang, "help.ep_about_desc")},
			{"method": "GET", "path": "/api/autodiscover", "description": i18n.T(lang, "help.ep_autodiscover_desc")},
		},
		"formats": []string{"json", "csv", "geojson", "text"},
		"docs":    "/server/docs/swagger",
	})
}

// handleServerPrivacyAPI returns the privacy policy as JSON per AI.md PART 14.
func (s *Server) handleServerPrivacyAPI(w http.ResponseWriter, r *http.Request) {
	lang := i18n.FromContext(r.Context())
	s.respondItem(w, http.StatusOK, map[string]interface{}{
		"summary": map[string]interface{}{
			"data_stored_on_server": false,
			"data_sold":             false,
			"user_control":          false,
		},
		"cookies": map[string]interface{}{
			"essential": map[string]interface{}{
				"enabled":     true,
				"description": i18n.T(lang, "privacy.api_cookie_essential_desc"),
			},
			"preferences": map[string]interface{}{
				"enabled":     false,
				"description": i18n.T(lang, "privacy.api_cookie_preferences_desc"),
			},
			"analytics": map[string]interface{}{
				"enabled":     false,
				"description": i18n.T(lang, "privacy.api_cookie_analytics_desc"),
			},
		},
		"data": map[string]interface{}{
			"sold":             false,
			"stored_on_server": false,
			"sharing":          []interface{}{},
		},
		"tracking": map[string]interface{}{
			"enabled":   false,
			"type":      "",
			"type_name": "",
		},
		"retention": map[string]interface{}{
			"period":             i18n.T(lang, "privacy.api_retention_period"),
			"export_available":   false,
			"deletion_available": false,
		},
		"third_party": map[string]interface{}{
			"services": []interface{}{},
		},
		"content": map[string]interface{}{
			"consent_message": i18n.T(lang, "privacy.api_consent_message"),
			"data_usage":      i18n.T(lang, "privacy.api_data_usage"),
		},
	})
}

// handleServerTermsAPI returns the terms of use as JSON per AI.md PART 14.
func (s *Server) handleServerTermsAPI(w http.ResponseWriter, r *http.Request) {
	s.respondItem(w, http.StatusOK, map[string]interface{}{
		"service": "Airports API",
		"version": Version,
		"license": "MIT",
		"data_source": map[string]string{
			"airports": "OurAirports (public domain)",
			"geoip":    "ip-location-db via jsDelivr CDN (CC0/PDDL)",
		},
		"usage": "Free, open, and authentication-free. No rate-limit keys required.",
		"repo":  "https://github.com/apimgr/airports",
	})
}

// handleServerAboutAPI returns build metadata as JSON per AI.md PART 14.
func (s *Server) handleServerAboutAPI(w http.ResponseWriter, r *http.Request) {
	s.respondItem(w, http.StatusOK, map[string]interface{}{
		"name":        "Airports API",
		"version":     Version,
		"commit":      CommitID,
		"build_date":  BuildDate,
		"repo":        "https://github.com/apimgr/airports",
		"license":     "MIT",
		"go_version":  runtime.Version(),
		"description": "Global airport reference data — free, open, and authentication-free.",
	})
}

// handleServerHealthz is implemented in healthz.go per AI.md PART 13.

// handleAutodiscover returns server settings, configuration schema, and CLI/agent
// options per AI.md PART 14 (/api/autodiscover — unversioned endpoint).
//
// Response includes: server version, API versions, primary URL, cluster URLs,
// cli_versions (per-platform tarball info), and cli_min_version.
func (s *Server) handleAutodiscover(w http.ResponseWriter, r *http.Request) {
	// Determine primary URL from request host (respects X-Forwarded-Proto / TLS).
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	if fwdProto := r.Header.Get("X-Forwarded-Proto"); fwdProto != "" {
		scheme = fwdProto
	}
	primaryURL := scheme + "://" + r.Host
	base := s.APIBasePath()

	resp := map[string]interface{}{
		"server": map[string]interface{}{
			"name":         "Airports API",
			"version":      Version,
			"commit":       CommitID,
			"build_date":   BuildDate,
			"primary_url":  primaryURL,
			"cluster_urls": []string{},
		},
		"api": map[string]interface{}{
			"versions":        []string{s.config.Server.APIVersion},
			"current_version": s.config.Server.APIVersion,
			"base_path":       "/api",
		},
		// cli_versions maps os/arch → {version, url, sha256}.
		// Populated at release time; empty map when not configured.
		"cli_versions": map[string]interface{}{},
		// cli_min_version is the oldest CLI that this server still supports.
		"cli_min_version": "0.0.1",
		"endpoints": map[string]string{
			"healthz":      primaryURL + base + "/server/healthz",
			"about":        primaryURL + base + "/server/about",
			"swagger":      primaryURL + base + "/server/swagger",
			"graphql":      primaryURL + base + "/server/graphql",
			"airports":     primaryURL + base + "/airports",
			"search":       primaryURL + base + "/airports/search",
			"nearby":       primaryURL + base + "/airports/nearby",
			"within":       primaryURL + base + "/airports/within",
			"autocomplete": primaryURL + base + "/airports/autocomplete",
			"geoip":        primaryURL + base + "/geoip",
			"stats":        primaryURL + base + "/stats",
		},
		"features": map[string]bool{
			"geoip":          true,
			"graphql":        true,
			"metrics":        true,
			"tor":            false,
			"authentication": false,
		},
	}

	s.respondItem(w, http.StatusOK, resp)
}
