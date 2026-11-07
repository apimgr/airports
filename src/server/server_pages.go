package server

import (
	"net/http"
	"runtime"
	"strings"

	"github.com/apimgr/airports/src/common/i18n"
	"github.com/apimgr/airports/src/config"
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
	privacy := s.config.Server.Privacy
	tracking := s.config.Server.Tracking
	services := configuredThirdPartyServices(privacy, tracking)
	s.renderTemplate(w, r, "server_privacy", map[string]interface{}{
		"Title":              "Privacy",
		"Theme":              s.config.Web.UI.Theme,
		"Privacy":            &privacy,
		"Tracking":           &tracking,
		"TrackingTypeName":   trackingTypeName(tracking.Type),
		"ThirdPartyServices": services,
		"CCPAOptedOut":       ccpaOptOutFromRequest(r),
	})
}

func configuredThirdPartyServices(privacy config.PrivacyConfig, tracking config.TrackingConfig) []config.ThirdPartyService {
	services := append([]config.ThirdPartyService(nil), privacy.ThirdParty.Services...)
	if tracking.Type == "" || tracking.Type == "none" {
		return services
	}
	name := trackingTypeName(tracking.Type)
	for _, service := range services {
		if strings.EqualFold(service.Name, name) {
			return services
		}
	}
	return append(services, config.ThirdPartyService{
		Name: name, Purpose: "Usage analytics", DataSent: "Anonymized: page views, browser type, country",
	})
}

func trackingTypeName(provider string) string {
	switch provider {
	case "google":
		return "Google Analytics"
	case "matomo", "piwik":
		return "Matomo"
	case "owa":
		return "Open Web Analytics"
	case "fathom":
		return "Fathom"
	case "plausible":
		return "Plausible"
	case "umami":
		return "Umami"
	case "simple":
		return "Simple Analytics"
	case "cloudflare":
		return "Cloudflare Web Analytics"
	default:
		return ""
	}
}

// handleServerTerms renders the Terms of Use page.
func (s *Server) handleServerTerms(w http.ResponseWriter, r *http.Request) {
	s.renderTemplate(w, r, "server_terms", map[string]interface{}{
		"Title": "Terms",
		"Theme": s.config.Web.UI.Theme,
	})
}

// handleServerSecurity renders the public security reporting overview.
func (s *Server) handleServerSecurity(w http.ResponseWriter, r *http.Request) {
	contact := s.securityContactEmail(r)
	s.renderTemplate(w, r, "server_security", map[string]interface{}{
		"Title":          "Security",
		"Theme":          s.config.Web.UI.Theme,
		"SecurityEmail":  contact,
		"SecurityTxtURL": strings.TrimSuffix(RequestBaseURL(r), "/") + "/.well-known/security.txt",
	})
}

// handleServerSecurityPolicy renders the coordinated disclosure guidance.
func (s *Server) handleServerSecurityPolicy(w http.ResponseWriter, r *http.Request) {
	s.renderTemplate(w, r, "server_security_policy", map[string]interface{}{
		"Title": "Security Policy",
		"Theme": s.config.Web.UI.Theme,
	})
}

// handleServerSecurityThanks renders the public researcher acknowledgments.
func (s *Server) handleServerSecurityThanks(w http.ResponseWriter, r *http.Request) {
	s.renderTemplate(w, r, "server_security_thanks", map[string]interface{}{
		"Title": "Security Acknowledgments",
		"Theme": s.config.Web.UI.Theme,
	})
}

func (s *Server) publicContactEmails(r *http.Request) (general, abuse string) {
	if s.isTorRequest(r) {
		address := strings.TrimSpace(s.config.Server.Tor.ContactEmail)
		return address, address
	}
	contacts := s.config.Server.Contact
	host := strings.TrimSpace(s.config.Server.FQDN)
	resolve := func(value string) string {
		if strings.Contains(value, "{fqdn}") && host == "" {
			return ""
		}
		return strings.ReplaceAll(strings.TrimSpace(value), "{fqdn}", host)
	}
	general = resolve(contacts.General.Email)
	if general == "" {
		general = resolve(s.config.Server.Privacy.Contact.General)
	}
	// Public abuse contact intentionally never falls back to the private admin
	// address; PART 12 permits only abuse.email then general.email here.
	abuse = resolve(contacts.Abuse.Email)
	if abuse == "" {
		abuse = resolve(contacts.General.Email)
	}
	if abuse == "" {
		abuse = resolve(s.config.Server.Privacy.Contact.Abuse)
	}
	if abuse == "" {
		abuse = resolve(s.config.Server.Privacy.Contact.General)
	}
	return general, abuse
}

func (s *Server) securityContactEmail(r *http.Request) string {
	if s.isTorRequest(r) {
		return strings.TrimSpace(s.config.Server.Tor.ContactEmail)
	}
	value := strings.TrimSpace(s.config.Server.Contact.Security.Email)
	if value == "" || value == "security@{fqdn}" {
		value = s.config.Web.Security.Admin
	}
	if value == "" {
		value = "security@{fqdn}"
	}
	host := strings.TrimSpace(s.config.Server.FQDN)
	if strings.Contains(value, "{fqdn}") && host == "" {
		return ""
	}
	return strings.ReplaceAll(strings.TrimSpace(value), "{fqdn}", host)
}

// handleServerContact renders the public contact form and contact information.
func (s *Server) handleServerContact(w http.ResponseWriter, r *http.Request) {
	if !s.config.Server.Pages.Contact.Enabled {
		http.NotFound(w, r)
		return
	}
	general, abuse := s.publicContactEmails(r)
	captchaType := strings.ToLower(strings.TrimSpace(s.config.Server.Pages.Contact.Captcha))
	if captchaType == "" {
		captchaType = "simple"
	}
	if captchaType != "simple" {
		message := "The configured contact CAPTCHA provider is unavailable."
		if s.isTorRequest(r) {
			message = "The configured CAPTCHA provider is not available over Tor; choose the self-hosted simple challenge."
		}
		s.writeErrorJSON(w, r, http.StatusServiceUnavailable, "CAPTCHA_UNAVAILABLE", message)
		return
	}
	question, challengeID := "", ""
	captchaRequired := s.contactCaptchaIsRequired(r)
	if captchaRequired {
		var err error
		question, challengeID, err = s.newContactCaptcha()
		if err != nil {
			s.writeErrorJSON(w, r, http.StatusServiceUnavailable, "CAPTCHA_UNAVAILABLE", "A contact challenge could not be created.")
			return
		}
	}
	s.renderTemplate(w, r, "server_contact", map[string]interface{}{
		"Title":           "Contact",
		"Theme":           s.config.Web.UI.Theme,
		"GeneralEmail":    general,
		"AbuseEmail":      abuse,
		"SecurityURL":     strings.TrimSuffix(RequestBaseURL(r), "/") + "/server/security",
		"APIVersion":      s.config.Server.APIVersion,
		"CaptchaType":     captchaType,
		"CaptchaQuestion": question,
		"CaptchaID":       challengeID,
		"CaptchaRequired": captchaRequired,
	})
}

// handleServerContactAPI returns contact addresses as JSON per AI.md PART 14.
func (s *Server) handleServerContactAPI(w http.ResponseWriter, r *http.Request) {
	general, abuse := s.publicContactEmails(r)
	baseURL := strings.TrimSuffix(RequestBaseURL(r), "/")
	s.respondItem(w, http.StatusOK, map[string]interface{}{
		"general": general,
		"abuse":   abuse,
		"security": map[string]string{
			"info_url": baseURL + "/security.txt",
		},
		"repo": "https://github.com/apimgr/airports",
	})
}

// handleServerHelpAPI returns the API help reference as JSON per AI.md PART 14.
func (s *Server) handleServerHelpAPI(w http.ResponseWriter, r *http.Request) {
	lang := i18n.FromContext(r.Context())
	baseURL := strings.TrimSuffix(RequestBaseURL(r), "/")
	base := baseURL + s.APIBasePath()
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
			{"method": "GET", "path": baseURL + "/api/autodiscover", "description": i18n.T(lang, "help.ep_autodiscover_desc")},
		},
		"formats": []string{"json", "csv", "geojson", "text"},
		"docs":    baseURL + "/server/docs/swagger",
	})
}

// handleServerPrivacyAPI returns the privacy policy as JSON per AI.md PART 14.
func (s *Server) handleServerPrivacyAPI(w http.ResponseWriter, r *http.Request) {
	privacy := s.config.Server.Privacy
	tracking := s.config.Server.Tracking
	services := configuredThirdPartyServices(privacy, tracking)
	sharing := make([]map[string]string, 0, len(privacy.Data.Sharing))
	for _, condition := range privacy.Data.Sharing {
		sharing = append(sharing, map[string]string{
			"condition": condition.Condition,
			"when":      condition.When,
			"data":      condition.Data,
		})
	}
	serviceData := make([]map[string]string, 0, len(services))
	for _, service := range services {
		serviceData = append(serviceData, map[string]string{
			"name": service.Name, "purpose": service.Purpose,
			"data_sent": service.DataSent, "policy_url": service.PolicyURL,
		})
	}
	s.respondItem(w, http.StatusOK, map[string]interface{}{
		"summary": map[string]interface{}{
			"data_stored_on_server": privacy.Data.StoredOnServer,
			"data_sold":             privacy.Data.Sold,
			"user_control":          true,
		},
		"cookies": map[string]interface{}{
			"essential": map[string]interface{}{
				"enabled":     true,
				"description": privacy.Cookies.Essential.Description,
			},
			"preferences": map[string]interface{}{
				"enabled":     privacy.Cookies.Preferences.Enabled,
				"description": privacy.Cookies.Preferences.Description,
			},
			"analytics": map[string]interface{}{
				"enabled":     privacy.Cookies.Analytics.Enabled,
				"description": privacy.GetAnalyticsDescription(),
			},
		},
		"data": map[string]interface{}{
			"sold":             privacy.Data.Sold,
			"stored_on_server": privacy.Data.StoredOnServer,
			"sharing":          sharing,
		},
		"tracking": map[string]interface{}{
			"enabled":   CanLoadAnalytics(r),
			"type":      tracking.Type,
			"type_name": trackingTypeName(tracking.Type),
		},
		"retention": map[string]interface{}{
			"period":             privacy.Retention.Period,
			"export_available":   privacy.Retention.ExportAvailable,
			"deletion_available": privacy.Retention.DeletionAvailable,
		},
		"third_party": map[string]interface{}{
			"services": serviceData,
		},
		"ccpa": map[string]interface{}{
			"applicable":     privacy.IsCCPAApplicable(),
			"opt_out_url":    strings.TrimSuffix(RequestBaseURL(r), "/") + "/server/privacy#ccpa-opt-out",
			"user_opted_out": ccpaOptOutFromRequest(r),
		},
		"content": map[string]interface{}{
			"consent_message": privacy.GetConsentMessage(),
			"data_usage":      privacy.GetDataUsageContent(),
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
	primaryURL := s.publicBaseURL(r)
	baseURL := strings.TrimSuffix(RequestBaseURL(r), "/")
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
			"base_path":       baseURL + "/api",
		},
		// cli_versions maps os/arch → {version, url, sha256} from the matching GitHub release.
		"cli_versions": s.cliReleaseVersions(r),
		// cli_min_version is the oldest CLI that this server still supports.
		"cli_min_version": s.config.Server.Update.CLIMinVersion,
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
