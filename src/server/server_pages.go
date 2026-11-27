package server

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/apimgr/airports/src/common/i18n"
	"github.com/apimgr/airports/src/common/urlvars"
	"github.com/apimgr/airports/src/config"
	"github.com/go-chi/chi/v5"
)

const resourceLabelPlural = "airports"

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
		"Tagline":     i18n.TranslateText(lang, "about.tagline"),
		"Description": i18n.TranslateText(lang, "about.description"),
		"Features": []string{
			i18n.TranslateText(lang, "about.feature_rest"),
			i18n.TranslateText(lang, "about.feature_graphql"),
			i18n.TranslateText(lang, "about.feature_swagger"),
			i18n.TranslateText(lang, "about.feature_geoip"),
			i18n.TranslateText(lang, "about.feature_geo_queries"),
			i18n.TranslateText(lang, "about.feature_webui"),
			i18n.TranslateText(lang, "about.feature_cli"),
			i18n.TranslateText(lang, "about.feature_binary"),
		},
		"Repo":    "https://github.com/apimgr/airports",
		"License": "MIT",
		"Version": Version,
	})
}

func (s *Server) handleServerSwaggerUI(w http.ResponseWriter, r *http.Request) {
	s.renderTemplate(w, r, "server_docs_swagger", map[string]interface{}{
		"Title": "API Documentation", "SwaggerUI": true,
		"SwaggerURL": strings.TrimSuffix(RequestBaseURL(r), "/") + s.APIBasePath() + "/server/swagger",
	})
}

func (s *Server) handleServerGraphQLUI(w http.ResponseWriter, r *http.Request) {
	s.renderTemplate(w, r, "server_docs_graphql", map[string]interface{}{
		"Title": "GraphQL Playground", "GraphiQLUI": true,
		"GraphQLEndpoint": strings.TrimSuffix(RequestBaseURL(r), "/") + s.APIBasePath() + "/server/graphql",
	})
}

// handleServerHelp renders the Help / API reference page.
func (s *Server) handleServerHelp(w http.ResponseWriter, r *http.Request) {
	lang := i18n.FromContext(r.Context())
	apiBase := strings.TrimSuffix(RequestBaseURL(r), "/") + s.APIBasePath()
	torEnabled, onionAddress := false, ""
	if s.tor != nil {
		torEnabled = s.tor.IsTorEnabled()
		if s.config.Server.Tor.Expose {
			onionAddress = strings.TrimSpace(s.tor.OnionAddress())
		}
	}
	i2pEnabled, i2pRunning, i2pAddress := false, false, ""
	if s.i2p != nil {
		i2pEnabled = s.i2p.IsI2PEnabled()
		i2pRunning = s.i2p.IsI2PRunning()
		if i2pRunning && s.config.Server.I2P.Expose {
			i2pAddress = strings.TrimSpace(s.i2p.EepsiteAddress())
		}
	}
	s.renderTemplate(w, r, "server_help", map[string]interface{}{
		"Title":       i18n.TranslateText(lang, "help.title"),
		"Theme":       s.config.Web.UI.Theme,
		"HelpPages":   s.registeredHelpPages(lang),
		"HelpContent": strings.TrimSpace(s.config.Server.Pages.Help.Content),
		"APIBase":     apiBase, "TorEnabled": torEnabled, "TorRunning": onionAddress != "", "OnionAddress": onionAddress,
		"I2PEnabled": i2pEnabled, "I2PRunning": i2pRunning, "I2PAddress": i2pAddress,
	})
}

type helpPage struct {
	Path        string `json:"path"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Order       int    `json:"order"`
	Priority    string `json:"-"`
	ChangeFreq  string `json:"-"`
	titleKey    string
	descKey     string
	enabled     func(*Server) bool
}

// helpPageRegistry is the source for help hub links and the help API page list.
var helpPageRegistry = []helpPage{{
	Path: "/server/help/mcp", Order: 10, Priority: "0.7", ChangeFreq: "weekly",
	titleKey: "help.mcp_title", descKey: "help.mcp_desc",
	enabled: func(*Server) bool { return true },
}}

var preferencesPageRegistry = []helpPage{
	{Path: "/server/preferences/tokens", Order: 10, titleKey: "owner_tokens.title", descKey: "owner_tokens.new_hint", enabled: func(*Server) bool { return true }},
	{Path: "/server/preferences/sync", Order: 20, titleKey: "preferences.sync_title", descKey: "preferences.sync_description", enabled: func(*Server) bool { return true }},
}

func (s *Server) registeredPreferencePages(lang string) []helpPage {
	pages := make([]helpPage, 0, len(preferencesPageRegistry))
	for _, page := range preferencesPageRegistry {
		if page.enabled != nil && !page.enabled(s) {
			continue
		}
		page.Title = i18n.TranslateText(lang, page.titleKey)
		page.Description = i18n.TranslateText(lang, page.descKey)
		pages = append(pages, page)
	}
	return pages
}

func (s *Server) registeredHelpPages(lang string) []helpPage {
	pages := make([]helpPage, 0, len(helpPageRegistry))
	for _, page := range helpPageRegistry {
		if page.enabled != nil && !page.enabled(s) {
			continue
		}
		page.Title = i18n.TranslateText(lang, page.titleKey)
		page.Description = i18n.TranslateText(lang, page.descKey)
		pages = append(pages, page)
	}
	return pages
}

func (s *Server) helpPageEnabled(path string) bool {
	for _, page := range helpPageRegistry {
		if page.Path == path {
			return page.enabled == nil || page.enabled(s)
		}
	}
	return false
}

func (s *Server) handleServerMCPHelp(w http.ResponseWriter, r *http.Request) {
	lang := i18n.FromContext(r.Context())
	baseURL := strings.TrimSuffix(s.publicBaseURL(r), "/")
	tools := s.mcpTools()
	endpoint := baseURL + s.APIBasePath() + "/server/mcp"
	toolsListJSON, _ := json.Marshal(map[string]interface{}{
		"jsonrpc": "2.0", "id": 1, "method": "tools/list",
		"params": map[string]interface{}{"_meta": map[string]string{"io.modelcontextprotocol/protocolVersion": currentMCPVersion}},
	})
	toolsListCommand := "curl --request POST --url " + shellSingleQuote(endpoint) +
		" --header 'Content-Type: application/json' --header 'Authorization: Bearer '\"$MCP_TOKEN\"" +
		" --header 'MCP-Protocol-Version: " + currentMCPVersion + "' --header 'Mcp-Method: tools/list' --data " + shellSingleQuote(string(toolsListJSON))
	configJSON, _ := json.MarshalIndent(map[string]interface{}{"mcpServers": map[string]interface{}{"airports": map[string]interface{}{
		"type": "streamable-http", "url": endpoint,
		"headers": map[string]string{"Authorization": "Bearer $MCP_TOKEN"},
	}}}, "", "  ")
	claudeCommand := "claude mcp add --transport http airports " + shellSingleQuote(endpoint) + " --header \"Authorization: Bearer $MCP_TOKEN\""
	s.renderTemplate(w, r, "server_help_mcp", map[string]interface{}{
		"Title":        i18n.TranslateText(lang, "help.mcp_title"),
		"Theme":        s.config.Web.UI.Theme,
		"MCPURL":       endpoint,
		"MCPConfig":    string(configJSON),
		"MCPClaude":    claudeCommand,
		"MCPToolsList": toolsListCommand,
		"Tools":        tools,
		"Versions":     []string{currentMCPVersion},
		"Project":      strings.TrimSpace(s.config.Server.Pages.Help.MCP),
	})
}

// shellSingleQuote quotes one value for a POSIX shell command.
func shellSingleQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func (s *Server) handleServerMCPHelpAPI(w http.ResponseWriter, r *http.Request) {
	baseURL := strings.TrimSuffix(s.publicBaseURL(r), "/")
	tools := s.mcpTools()
	publicTools := make([]map[string]interface{}, 0, len(tools))
	for _, tool := range tools {
		publicTools = append(publicTools, map[string]interface{}{
			"name": tool.Name, "description": tool.Description,
		})
	}
	s.respondItem(w, http.StatusOK, map[string]interface{}{
		"enabled": true,
		"url":     baseURL + s.APIBasePath() + "/server/mcp",
		"aliases": []string{baseURL + "/api/mcp", baseURL + "/mcp"},
		"auth":    map[string]string{"scheme": "Bearer"},
		"tools":   publicTools,
		"project": strings.TrimSpace(s.config.Server.Pages.Help.MCP),
	})
}

// handleServerPrivacy renders the Privacy page.
func (s *Server) handleServerPrivacy(w http.ResponseWriter, r *http.Request) {
	privacy := s.config.Server.Privacy
	privacyText := localizedPrivacyDisplayText(privacy, i18n.FromContext(r.Context()))
	tracking := s.config.Server.Tracking
	lang := i18n.FromContext(r.Context())
	services := configuredThirdPartyServices(privacy, tracking, lang)
	dpoEmail := s.dpoEmail(r)
	s.renderTemplate(w, r, "server_privacy", map[string]interface{}{
		"Title":              "Privacy",
		"Theme":              s.config.Web.UI.Theme,
		"Privacy":            &privacy,
		"PrivacyText":        privacyText,
		"Tracking":           &tracking,
		"TrackingTypeName":   trackingTypeName(tracking.Type),
		"ThirdPartyServices": services,
		"CCPAOptedOut":       ccpaOptOutFromRequest(r),
		"PrivacyDPOEmail":    dpoEmail,
		"BuildDate":          BuildDate,
	})
}

func (s *Server) handleServerPreferences(w http.ResponseWriter, r *http.Request) {
	selected := "auto"
	if queryLanguage := strings.TrimSpace(r.URL.Query().Get("lang")); queryLanguage != "" && i18n.IsSupported(queryLanguage) {
		selected = queryLanguage
	} else if cookie, err := r.Cookie(i18n.LanguageCookieName); err == nil && i18n.IsSupported(cookie.Value) {
		selected = cookie.Value
	}
	selectedTheme := s.config.Web.UI.Theme
	if cookie, err := r.Cookie(themeCookieName); err == nil && validThemes[cookie.Value] {
		selectedTheme = cookie.Value
	}
	detected := i18n.FromContext(r.Context())
	detectedName := detected
	for _, language := range i18n.AvailableLanguages() {
		if language.Code == detected {
			detectedName = language.NativeName
			break
		}
	}
	s.renderTemplate(w, r, "server_preferences", map[string]interface{}{
		"Title": "Preferences", "Theme": s.config.Web.UI.Theme,
		"Languages": i18n.AvailableLanguages(), "SelectedLanguage": selected,
		"DetectedLanguage": detected, "DetectedLanguageName": detectedName,
		"SelectedTheme": selectedTheme, "Privacy": &s.config.Server.Privacy,
		"PreferencePages": s.registeredPreferencePages(i18n.FromContext(r.Context())),
		"CCPAOptedOut":    ccpaOptOutFromRequest(r),
	})
}

func (s *Server) handleOwnerTokensPage(w http.ResponseWriter, r *http.Request) {
	s.renderOwnerTokensPage(w, r)
}

func (s *Server) renderOwnerTokensPage(w http.ResponseWriter, r *http.Request) {
	revealID := ""
	revealToken := ""
	notice := ""
	if noticeCookie, err := r.Cookie(ownerTokenNoticeCookieName); err == nil {
		s.setOwnerTokenNoticeCookie(w, "", true)
		switch noticeCookie.Value {
		case "created", "revealed", "rotated", "updated", "revoked":
			notice = noticeCookie.Value
		}
	}
	if viewCookie, err := r.Cookie(ownerTokenViewCookieName); err == nil {
		s.setOwnerTokenViewCookie(w, "", true)
		revealToken = ""
		revealID = ""
		if identity, _, ok := s.ownerTokenFromCookie(r); ok && identity.Kind == "primary" {
			var id string
			var expectedKind string
			switch viewCookie.Value {
			case "primary":
				id, expectedKind = identity.ID, "primary"
			default:
				if validOwnerTokenID(viewCookie.Value) {
					id, expectedKind = viewCookie.Value, "linked"
				}
			}
			if id != "" {
				if token, revealErr := s.revealOwnerToken(identity.Owner, id, expectedKind); revealErr == nil {
					revealID, revealToken = id, token
				}
			}
		}
	}
	identity, cookieToken, hasOwner := s.ownerTokenFromCookie(r)
	if hasOwner && identity.Kind != "primary" {
		hasOwner = false
		identity = nil
	}
	if hasOwner {
		s.setOwnerCookie(w, r, cookieToken)
	} else if _, err := s.ownerTokenCookie(r); err == nil {
		s.clearOwnerCookie(w, r)
	}
	items := []ownerTokenRecord{}
	linkedCount := 0
	primaryID := ""
	primaryLast4 := ""
	resourceCount := int64(0)
	if hasOwner && identity != nil {
		records, err := s.ownerTokenRecords(identity.Owner)
		if err != nil {
			s.renderErrorPage(w, r, http.StatusInternalServerError)
			return
		}
		items = records
		for _, item := range records {
			switch item.Kind {
			case "primary":
				primaryID = item.ID
			case "linked":
				linkedCount++
			}
		}
		_, _, resourceCount, primaryLast4, err = ownerTokenRevocationDetails(s.db, identity.Owner)
		if err != nil {
			s.renderErrorPage(w, r, http.StatusInternalServerError)
			return
		}
	}
	w.Header().Set("Cache-Control", "no-store")
	s.renderTemplate(w, r, "server_owner_tokens", map[string]interface{}{
		"Title": "Owner tokens", "Theme": s.config.Web.UI.Theme,
		"OwnerActive": hasOwner, "Tokens": items, "PrimaryTokenID": primaryID, "PrimaryLast4": primaryLast4,
		"LinkedCount": linkedCount, "ResourceCount": resourceCount,
		"RevealID": revealID, "RevealToken": revealToken, "TokenNotice": notice,
		"ResourceLabelPlural": resourceLabelPlural,
	})
}

func configuredThirdPartyServices(privacy config.PrivacyConfig, tracking config.TrackingConfig, lang string) []config.ThirdPartyService {
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
		Name:     name,
		Purpose:  i18n.TranslateText(lang, "privacy.analytics_service_purpose"),
		DataSent: i18n.TranslateText(lang, "privacy.analytics_service_data"),
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

func (s *Server) handleServerDPO(w http.ResponseWriter, r *http.Request) {
	contacts := s.config.Server.Contact
	s.renderTemplate(w, r, "server_dpo", map[string]interface{}{
		"Title": "Data Protection Officer",
		"Theme": s.config.Web.UI.Theme,
		"Name":  strings.TrimSpace(contacts.DPO.Name),
		"Email": s.dpoEmail(r),
	})
}

func (s *Server) handleServerDPOAPI(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimSpace(s.config.Server.Contact.DPO.Name)
	if name == "" {
		name = ""
	}
	base := strings.TrimSuffix(RequestBaseURL(r), "/")
	s.respondItem(w, http.StatusOK, map[string]string{
		"name":        name,
		"email":       s.dpoEmail(r),
		"privacy_url": base + "/server/privacy",
	})
}

func (s *Server) dpoEmail(r *http.Request) string {
	value := s.config.Server.Contact.ResolveEmail("dpo")
	host := s.resolvedFQDN(r)
	if strings.Contains(value, "{fqdn}") && host == "localhost" {
		return ""
	}
	return resolveFQDNPlaceholder(value, host)
}

// handleServerSecurity renders the public security reporting overview.
func (s *Server) handleServerSecurity(w http.ResponseWriter, r *http.Request) {
	contact := s.securityContactEmail(r)
	securityID, _ := s.securityIDAt(time.Now())
	s.renderTemplate(w, r, "server_security", map[string]interface{}{
		"Title":          "Security",
		"Theme":          s.config.Web.UI.Theme,
		"SecurityEmail":  contact,
		"SecurityTxtURL": strings.TrimSuffix(RequestBaseURL(r), "/") + "/.well-known/security.txt",
		"BaseURL":        strings.TrimSuffix(RequestBaseURL(r), "/"),
		"ReportURL":      s.securityReportURL(r),
		"SecurityID":     securityID,
		"ReportContact":  strings.TrimSpace(s.config.Web.Security.ReportURL),
		"SecurityKeyAvailable": func() bool {
			key, err := s.pgpPublicKey()
			return err == nil && len(key) > 0
		}(),
	})
}

// handleServerSecurityPolicy renders the coordinated disclosure guidance.
func (s *Server) handleServerSecurityPolicy(w http.ResponseWriter, r *http.Request) {
	policy := strings.TrimSpace(s.config.Web.Security.Policy)
	if policy == "" {
		policy = s.defaultSecurityPolicy(r)
	}
	s.renderTemplate(w, r, "server_security_policy", map[string]interface{}{
		"Title":  "Security Policy",
		"Theme":  s.config.Web.UI.Theme,
		"Policy": policy,
	})
}

func (s *Server) defaultSecurityPolicy(r *http.Request) string {
	cfg := s.config
	host := s.resolvedFQDN(r)
	i2pManager := s.i2p
	domains := make([]string, 0, 3)
	if host != "" && !strings.EqualFold(host, "localhost") {
		domains = append(domains, host)
	}
	for _, domain := range strings.Split(os.Getenv("DOMAIN"), ",") {
		domain = strings.TrimSpace(domain)
		if domain != "" && domain != host && urlvars.IsValidHost(domain, strings.EqualFold(cfg.Server.Mode, "development"), "airports") {
			domains = append(domains, domain)
		}
	}
	scope := strings.Join(domains, ", ")
	if cfg.Server.Tor.Expose && strings.TrimSpace(cfg.Server.Tor.OnionAddress) != "" {
		scope += ", " + strings.TrimSpace(cfg.Server.Tor.OnionAddress)
	}
	if cfg.Server.I2P.Expose && i2pManager != nil && strings.TrimSpace(i2pManager.EepsiteAddress()) != "" {
		scope += ", " + strings.TrimSpace(i2pManager.EepsiteAddress())
	}
	lang := "en"
	if r != nil {
		lang = i18n.FromContext(r.Context())
	}
	securityLink := "[" + i18n.TranslateText(lang, "security.policy_page_link") + "](/server/security)"
	return strings.Join([]string{
		"## " + i18n.TranslateText(lang, "security.policy_scope_heading"),
		i18n.TranslateFormattedText(lang, "security.policy_scope_body", "scope", scope),
		"## " + i18n.TranslateText(lang, "security.policy_out_of_scope_heading"),
		i18n.TranslateText(lang, "security.policy_out_of_scope_body"),
		"## " + i18n.TranslateText(lang, "security.policy_coordinated_heading"),
		i18n.TranslateFormattedText(lang, "security.policy_coordinated_body", "security_link", securityLink),
		"## " + i18n.TranslateText(lang, "security.policy_safe_harbor_heading"),
		i18n.TranslateText(lang, "security.policy_safe_harbor_body"),
		"## " + i18n.TranslateText(lang, "security.policy_reporting_heading"),
		i18n.TranslateFormattedText(lang, "security.policy_reporting_body", "security_link", securityLink),
	}, "\n\n")
}

func (s *Server) handleServerSecurityAPI(w http.ResponseWriter, r *http.Request) {
	base := strings.TrimSuffix(RequestBaseURL(r), "/")
	var pgpURL, pgpFingerprint interface{}
	if publicKey, err := s.pgpPublicKey(); err == nil && len(publicKey) > 0 {
		if fingerprint, err := openPGPV4Fingerprint(publicKey); err == nil {
			pgpURL = base + "/.well-known/pgp-key.asc"
			pgpFingerprint = fingerprint
		}
	}
	pgp := map[string]interface{}{"available": pgpFingerprint != nil, "url": pgpURL, "fingerprint": pgpFingerprint}
	contacts := make([]map[string]string, 0, 3)
	if reportURL := strings.TrimSpace(s.config.Web.Security.ReportURL); reportURL != "" && !s.isTorRequest(r) {
		contacts = append(contacts, map[string]string{
			"type":  "github",
			"value": strings.NewReplacer("{project_org}", "apimgr", "{project_name}", "airports").Replace(reportURL),
		})
	}
	if s.config.Server.Pages.Contact.Enabled {
		if reportURL := s.securityReportURL(r); reportURL != "" {
			contacts = append(contacts, map[string]string{"type": "contact_form", "value": reportURL})
		}
	}
	contact := strings.TrimSpace(s.config.Web.Security.Contact)
	if s.isTorRequest(r) || contact == "" {
		contact = s.securityContactEmail(r)
	} else {
		contact = resolveFQDNPlaceholder(contact, s.resolvedFQDN(r))
	}
	if contact != "" {
		contacts = append(contacts, map[string]string{"type": "email", "value": strings.TrimPrefix(contact, "mailto:")})
	}
	data := map[string]interface{}{
		"contacts":   contacts,
		"expires":    securityExpiry(s.config.Web.Security.Expires, time.Now()),
		"pgp":        pgp,
		"policy_url": base + "/server/security/policy",
		"thanks_url": base + "/server/security/thanks",
	}
	if s.config.Server.Pages.Contact.Enabled {
		data["report_url"] = base + "/server/contact"
	}
	s.respondItem(w, http.StatusOK, data)
}

func (s *Server) handleServerSecurityPolicyAPI(w http.ResponseWriter, r *http.Request) {
	policy := strings.TrimSpace(s.config.Web.Security.Policy)
	if policy == "" {
		policy = s.defaultSecurityPolicy(r)
	}
	s.respondItem(w, http.StatusOK, map[string]string{"policy": policy})
}

func (s *Server) handleServerSecurityThanksAPI(w http.ResponseWriter, r *http.Request) {
	entries, err := s.securityThanksEntries()
	if err != nil {
		s.writeErrorJSON(w, r, http.StatusInternalServerError, "SECURITY_ACKNOWLEDGMENTS_UNAVAILABLE", "Security acknowledgments are unavailable.")
		return
	}
	s.respondItem(w, http.StatusOK, map[string]interface{}{"entries": entries})
}

func (s *Server) securityThanksEntries() (entries []map[string]string, resultErr error) {
	if s.db == nil {
		return []map[string]string{}, nil
	}
	if err := s.ensureDisclosureSchema(); err != nil {
		return nil, err
	}
	rows, err := s.db.Query(`SELECT credit, disclosed_on, severity FROM security_thanks ORDER BY disclosed_on DESC, tracking_id DESC`)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := rows.Close(); err != nil {
			resultErr = errors.Join(resultErr, fmt.Errorf("close security acknowledgments: %w", err))
		}
	}()
	entries = make([]map[string]string, 0)
	for rows.Next() {
		var credit, disclosedOn, severity string
		if err := rows.Scan(&credit, &disclosedOn, &severity); err != nil {
			return nil, err
		}
		entries = append(entries, map[string]string{"credit": credit, "date": disclosedOn, "severity": severity})
	}
	return entries, rows.Err()
}

func (s *Server) handleServerSecurityReportStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("Cache-Control", "no-store")
	if s.db == nil {
		s.renderErrorPage(w, r, http.StatusNotFound)
		return
	}
	token := strings.TrimSpace(securityStatusToken(r))
	if token == "" {
		token = strings.TrimSpace(r.URL.Query().Get("token"))
	}
	digest := sha256.Sum256([]byte(token))
	var tokenHash, status, created string
	var usedAt, closedAt, disclosureDate sql.NullString
	var researcherComment string
	err := s.db.QueryRow(`SELECT token_hash, status, created_at, token_last_used, closed_at, researcher_comment, disclosure_date FROM security_reports WHERE tracking_id = ?`, chi.URLParam(r, "tracking_id")).Scan(&tokenHash, &status, &created, &usedAt, &closedAt, &researcherComment, &disclosureDate)
	if err != nil || len(token) != 64 || !constantHexEqual(tokenHash, hex.EncodeToString(digest[:])) {
		s.renderErrorPage(w, r, http.StatusNotFound)
		return
	}
	now := time.Now().UTC()
	if closedAt.Valid && closedAt.String != "" {
		closed, parseErr := time.Parse(time.RFC3339, closedAt.String)
		if parseErr != nil {
			closed, parseErr = time.Parse("2006-01-02 15:04:05", closedAt.String)
		}
		if parseErr != nil || now.After(closed.Add(30*24*time.Hour)) {
			s.renderErrorPage(w, r, http.StatusNotFound)
			return
		}
	}
	if usedAt.Valid && usedAt.String != "" {
		used, parseErr := time.Parse("2006-01-02", usedAt.String)
		if parseErr == nil && used.Format("2006-01-02") == now.Format("2006-01-02") {
			s.renderErrorPage(w, r, http.StatusNotFound)
			return
		}
	}
	today := now.Format("2006-01-02")
	result, err := s.db.Exec(`UPDATE security_reports SET token_last_used = ? WHERE tracking_id = ? AND (token_last_used IS NULL OR token_last_used <> ?)`, today, chi.URLParam(r, "tracking_id"), today)
	if err != nil {
		s.renderErrorPage(w, r, http.StatusInternalServerError)
		return
	}
	if affected, err := result.RowsAffected(); err != nil || affected != 1 {
		s.renderErrorPage(w, r, http.StatusNotFound)
		return
	}
	disclosure := ""
	if disclosureDate.Valid {
		disclosure = disclosureDate.String
	}
	s.renderTemplate(w, r, "server_security_report", map[string]interface{}{
		"Title": "Security Report Status", "Theme": s.config.Web.UI.Theme,
		"Status": status, "CreatedAt": created, "ResearcherComment": researcherComment,
		"DisclosureDate": disclosure,
	})
}

// handleServerSecurityThanks renders the public researcher acknowledgments.
func (s *Server) handleServerSecurityThanks(w http.ResponseWriter, r *http.Request) {
	entries, err := s.securityThanksEntries()
	if err != nil {
		s.renderErrorPage(w, r, http.StatusInternalServerError)
		return
	}
	s.renderTemplate(w, r, "server_security_thanks", map[string]interface{}{
		"Title":   "Security Acknowledgments",
		"Theme":   s.config.Web.UI.Theme,
		"Entries": entries,
	})
}

func (s *Server) publicContactEmails(r *http.Request) (general, abuse string) {
	if s.isTorRequest(r) {
		address := strings.TrimSpace(s.config.Server.Tor.ContactEmail)
		return address, address
	}
	contacts := s.config.Server.Contact
	host := s.resolvedFQDN(r)
	resolve := func(value string) string {
		return resolveFQDNPlaceholder(value, host)
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
	host := s.resolvedFQDN(r)
	return resolveFQDNPlaceholder(value, host)
}

// handleServerContact renders the public contact form and contact information.
func (s *Server) handleServerContact(w http.ResponseWriter, r *http.Request) {
	if !s.config.Server.Pages.Contact.Enabled {
		http.NotFound(w, r)
		return
	}
	securityID := strings.TrimSpace(r.URL.Query().Get("security_id"))
	securityMode := securityID != "" && s.validSecurityID(securityID, time.Now())
	if securityID != "" && !securityMode {
		s.logInvalidSecurityID(r, securityID)
	}
	kind := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("type")))
	allowed := contactAllowedTypes(securityMode)
	validType := false
	for _, candidate := range allowed {
		if candidate == kind {
			validType = true
			break
		}
	}
	showChooser := !validType
	data, err := s.contactPageData(r, kind, securityMode && validType, securityID, showChooser)
	if err != nil {
		s.writeErrorJSON(w, r, http.StatusServiceUnavailable, "CONTACT_UNAVAILABLE", "The contact form is unavailable.")
		return
	}
	w.Header().Set("X-Robots-Tag", "noindex, nofollow")
	s.renderTemplate(w, r, "server_contact", data)
}

func (s *Server) contactPageData(r *http.Request, kind string, securityMode bool, securityID string, showChooser bool) (map[string]interface{}, error) {
	general, abuse := s.publicContactEmails(r)
	fqdn := s.resolvedFQDN(r)
	formFields := contactFormFields(kind)
	if kind == "abuse" || kind == "dmca" {
		for _, field := range formFields {
			if field["name"] == "reported_url" {
				field["required"] = true
			}
		}
	}
	for _, field := range formFields {
		if field["max_length"] == nil {
			field["max_length"] = 0
		}
		field["value"] = ""
	}
	data := map[string]interface{}{
		"Title": "Contact", "Theme": s.config.Web.UI.Theme,
		"GeneralEmail": general, "AbuseEmail": abuse,
		"SecurityURL": strings.TrimSuffix(RequestBaseURL(r), "/") + "/server/security",
		"APIVersion":  s.config.Server.APIVersion, "FormAction": "/server/contact",
		"FQDN":         fqdn,
		"SecurityMode": securityMode, "SecurityID": securityID,
		"FormType": kind, "FormTitle": map[string]string{"abuse": "contact.type_abuse", "dmca": "contact.type_dmca", "privacy": "contact.type_privacy", "security": "contact.type_security"}[kind],
		"FormFields": formFields, "ShowChooser": showChooser,
		"Types": []map[string]string{
			{"type": "abuse", "url": "?type=abuse", "label": "contact.type_abuse"},
			{"type": "dmca", "url": "?type=dmca", "label": "contact.type_dmca"},
			{"type": "privacy", "url": "?type=privacy", "label": "contact.type_privacy"},
		},
	}
	if securityMode {
		data["Types"] = []map[string]string{{"type": "security", "url": "?security_id=" + url.QueryEscape(securityID) + "&type=security", "label": "contact.type_security"}}
	}
	if showChooser {
		return data, nil
	}
	captchaType := s.contactCaptchaProvider(r)
	if captchaType == "" {
		captchaType = "simple"
	}
	question, challengeID, err := s.newContactCaptchaForLanguage(i18n.FromContext(r.Context()))
	if err != nil {
		return nil, err
	}
	formToken, err := contactFormToken(s, time.Now())
	if err != nil {
		return nil, err
	}
	data["CaptchaType"] = captchaType
	data["CaptchaSiteKey"] = s.config.Server.Pages.Contact.CaptchaSiteKey
	data["CaptchaQuestion"] = question
	data["CaptchaID"] = challengeID
	data["CaptchaRequired"] = true
	data["FormToken"] = formToken
	return data, nil
}

// handleServerContactAPI returns the configured contact form definition.
func (s *Server) handleServerContactAPI(w http.ResponseWriter, r *http.Request) {
	if !s.config.Server.Pages.Contact.Enabled {
		http.NotFound(w, r)
		return
	}
	securityID := strings.TrimSpace(r.URL.Query().Get("security_id"))
	securityMode := securityID != "" && s.validSecurityID(securityID, time.Now())
	if securityID != "" && !securityMode {
		s.logInvalidSecurityID(r, securityID)
	}
	kind := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("type")))
	allowed := contactAllowedTypes(securityMode)
	validType := false
	for _, candidate := range allowed {
		if candidate == kind {
			validType = true
			break
		}
	}
	if !validType {
		kind = ""
	}
	baseURL := strings.TrimSuffix(RequestBaseURL(r), "/")
	question, challengeID, err := s.newContactCaptchaForLanguage(i18n.FromContext(r.Context()))
	if err != nil {
		s.writeErrorJSON(w, r, http.StatusServiceUnavailable, "CAPTCHA_UNAVAILABLE", "A contact challenge could not be created.")
		return
	}
	formToken, err := contactFormToken(s, time.Now())
	if err != nil {
		s.writeErrorJSON(w, r, http.StatusServiceUnavailable, "CONTACT_UNAVAILABLE", "The contact form is unavailable.")
		return
	}
	captcha := map[string]interface{}{"provider": "simple", "captcha_id": challengeID, "question": question}
	provider := s.contactCaptchaProvider(r)
	if provider != "" && provider != "simple" {
		captcha = map[string]interface{}{"provider": provider, "site_key": s.config.Server.Pages.Contact.CaptchaSiteKey, "fallback": map[string]string{"provider": "simple", "captcha_id": challengeID, "question": question}}
	}
	_, abuse := s.publicContactEmails(r)
	types := contactAllowedTypes(securityMode)
	fields := contactFormFields(kind)
	typeField := map[string]interface{}{"name": "type", "type": "enum", "required": true, "max_length": nil, "required_when": nil, "max_length_when": nil, "options": types}
	fields = append([]map[string]interface{}{typeField}, fields...)
	lang := i18n.FromContext(r.Context())
	for _, field := range fields {
		if label, ok := field["label"].(string); ok {
			field["label"] = i18n.TranslateText(lang, label)
		}
	}
	data := map[string]interface{}{
		"fields": fields, "captcha": captcha,
		"form_token": formToken, "types": types,
		"security_url": baseURL + "/server/security", "abuse_email": abuse,
	}
	s.respondItem(w, http.StatusOK, data)
}

// handleServerHelpAPI returns the API help reference as JSON per AI.md PART 14.
func (s *Server) handleServerHelpAPI(w http.ResponseWriter, r *http.Request) {
	lang := i18n.FromContext(r.Context())
	pages := make([]map[string]string, 0)
	for _, page := range s.registeredHelpPages(lang) {
		pages = append(pages, map[string]string{"title": page.Title, "path": page.Path})
	}
	s.respondItem(w, http.StatusOK, map[string]interface{}{
		"title":   i18n.TranslateText(lang, "help.title"),
		"content": strings.TrimSpace(s.config.Server.Pages.Help.Content),
		"format":  "markdown",
		"pages":   pages,
	})
}

// handleServerPrivacyAPI returns the privacy policy as JSON per AI.md PART 14.
func (s *Server) handleServerPrivacyAPI(w http.ResponseWriter, r *http.Request) {
	privacy := s.config.Server.Privacy
	privacyText := localizedPrivacyDisplayText(privacy, i18n.FromContext(r.Context()))
	tracking := s.config.Server.Tracking
	services := configuredThirdPartyServices(privacy, tracking, i18n.FromContext(r.Context()))
	sharing := make([]map[string]string, 0, len(privacy.Data.Sharing))
	for _, condition := range privacyText.Sharing {
		item := map[string]string{
			"condition": condition.Condition,
			"when":      condition.When,
			"data":      condition.Data,
		}
		item["description"] = condition.Description
		sharing = append(sharing, item)
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
				"description": privacyText.EssentialCookieDescription,
			},
			"preferences": map[string]interface{}{
				"enabled":     privacy.Cookies.Preferences.Enabled,
				"description": privacyText.PreferencesCookieDescription,
			},
			"analytics": map[string]interface{}{
				"enabled":     privacy.Cookies.Analytics.Enabled,
				"description": privacyText.AnalyticsCookieDescription,
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
			"period":             privacyText.RetentionPeriod,
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
			"consent_message": privacyText.ConsentMessage,
			"data_usage":      privacyText.DataUsage,
			"data_security":   privacyText.DataSecurity,
		},
	})
}

// handleServerTermsAPI returns the terms of use as JSON per AI.md PART 14.
func (s *Server) handleServerTermsAPI(w http.ResponseWriter, r *http.Request) {
	lang := i18n.FromContext(r.Context())
	format, content, ok := s.operatorContentSource(r, "terms")
	if !ok {
		format = "markdown"
		content = localizedTermsMarkdown(lang)
	}
	s.respondItem(w, http.StatusOK, map[string]interface{}{
		"title":   i18n.TranslateText(lang, "terms.title"),
		"content": content,
		"format":  format,
	})
}

func localizedTermsMarkdown(lang string) string {
	sections := []string{
		"# " + i18n.TranslateText(lang, "terms.title"),
		"## " + i18n.TranslateText(lang, "terms.acceptance_heading"),
		i18n.TranslateText(lang, "terms.acceptance_desc"),
		"## " + i18n.TranslateText(lang, "terms.service_heading"),
		i18n.TranslateText(lang, "terms.service_desc"),
		"## " + i18n.TranslateText(lang, "terms.acceptable_heading"),
		"- " + i18n.TranslateFormattedText(lang, "terms.rate_limit", "formats", ".json, .csv, .geojson"),
		"- " + i18n.TranslateText(lang, "terms.no_attack"),
		"- " + i18n.TranslateText(lang, "terms.attribution"),
		"## " + i18n.TranslateText(lang, "terms.warranty_heading"),
		i18n.TranslateText(lang, "terms.warranty"),
		"## " + i18n.TranslateText(lang, "terms.liability_heading"),
		i18n.TranslateText(lang, "terms.liability_desc"),
		"## " + i18n.TranslateText(lang, "terms.license_heading"),
		i18n.TranslateText(lang, "terms.license"),
		"## " + i18n.TranslateText(lang, "terms.changes_heading"),
		i18n.TranslateText(lang, "terms.changes_desc"),
		"## " + i18n.TranslateText(lang, "terms.governing_law_heading"),
		i18n.TranslateText(lang, "terms.governing_law_desc"),
	}
	return strings.Join(sections, "\n\n")
}

// handleServerAboutAPI returns the project description as JSON per AI.md PART 14.
func (s *Server) handleServerAboutAPI(w http.ResponseWriter, r *http.Request) {
	format, content, ok := s.operatorContentSource(r, "about")
	if !ok {
		format = "markdown"
		content = defaultAboutMarkdown(i18n.FromContext(r.Context()))
	}
	s.respondItem(w, http.StatusOK, map[string]interface{}{
		"title":   i18n.TranslateText(i18n.FromContext(r.Context()), "about.title"),
		"content": content,
		"format":  format,
	})
}

func defaultAboutMarkdown(lang string) string {
	features := []string{
		i18n.TranslateText(lang, "about.feature_rest"),
		i18n.TranslateText(lang, "about.feature_graphql"),
		i18n.TranslateText(lang, "about.feature_swagger"),
		i18n.TranslateText(lang, "about.feature_geoip"),
		i18n.TranslateText(lang, "about.feature_geo_queries"),
		i18n.TranslateText(lang, "about.feature_webui"),
		i18n.TranslateText(lang, "about.feature_cli"),
		i18n.TranslateText(lang, "about.feature_binary"),
	}
	var list strings.Builder
	for _, feature := range features {
		fmt.Fprintf(&list, "- %s\n", feature)
	}
	return fmt.Sprintf("# %s\n\n%s\n\n%s\n\n## %s\n\n%s\n\n## %s\n\n- [GitHub](https://github.com/apimgr/airports)\n- [Help and API documentation](/server/help)\n- [Swagger UI](/server/docs/swagger)\n- [GraphQL playground](/server/docs/graphql)\n",
		i18n.TranslateText(lang, "about.title"), i18n.TranslateText(lang, "about.tagline"),
		i18n.TranslateText(lang, "about.description"), i18n.TranslateText(lang, "about.features_heading"),
		strings.TrimSpace(list.String()), i18n.TranslateText(lang, "about.links_heading"))
}

// handleServerHealthz is implemented in healthz.go per AI.md PART 13.

// handleAutodiscover returns the public discovery contract from AI.md PART 14.
func (s *Server) handleAutodiscover(w http.ResponseWriter, r *http.Request) {
	resp := map[string]interface{}{
		"name":    "airports",
		"version": Version,
		"api": map[string]interface{}{
			"current_version": s.config.Server.APIVersion,
		},
		// cli_versions maps os/arch → {version, url, sha256} from the matching GitHub release.
		"cli_versions": s.cliReleaseVersions(r),
		// cli_min_version is the oldest CLI that this server still supports.
		"cli_min_version": s.config.Server.Update.CLIMinVersion,
		"urls":            s.autodiscoverURLs(r),
	}
	if s.config.Server.Tor.Expose && s.tor != nil && s.tor.IsTorEnabled() {
		if address := strings.TrimSpace(s.tor.OnionAddress()); address != "" {
			resp["onion"] = address
		}
	}
	if s.config.Server.I2P.Expose && s.i2p != nil && s.config.Server.I2P.Enabled {
		if address := strings.TrimSpace(s.i2p.EepsiteAddress()); address != "" {
			resp["i2p"] = address
		}
	}

	s.respondItem(w, http.StatusOK, resp)
}

func (s *Server) autodiscoverURLs(r *http.Request) []string {
	if s.isTorRequest(r) {
		if s.config.Server.Tor.OnionAddress == "" {
			return []string{}
		}
		return []string{"http://" + strings.TrimSuffix(strings.TrimSpace(s.config.Server.Tor.OnionAddress), "/")}
	}
	proto, host, port := urlvars.GetRequestURLVariables(r)
	if s.urlResolver != nil {
		proto, host, port = s.urlResolver.GetRequestURLVariables(r)
	}
	urls := make([]string, 0)
	seen := make(map[string]struct{})
	appendOrigin := func(candidateHost, candidatePort string) {
		candidateHost = strings.Trim(strings.TrimSpace(candidateHost), "[]")
		if candidateHost == "" || strings.HasSuffix(strings.ToLower(candidateHost), ".onion") || strings.HasSuffix(strings.ToLower(candidateHost), ".i2p") {
			return
		}
		if ip := net.ParseIP(candidateHost); ip != nil && ip.To4() == nil {
			candidateHost = "[" + ip.String() + "]"
		}
		if candidatePort != "" && candidatePort != "80" && candidatePort != "443" {
			candidateHost = net.JoinHostPort(strings.Trim(candidateHost, "[]"), candidatePort)
		}
		origin := proto + "://" + candidateHost
		if _, ok := seen[origin]; !ok {
			seen[origin] = struct{}{}
			urls = append(urls, origin)
		}
	}
	appendOrigin(host, port)
	for _, domain := range strings.Split(os.Getenv("DOMAIN"), ",") {
		domain = strings.TrimSpace(domain)
		if domain == "" {
			continue
		}
		parsed, err := url.Parse("//" + domain)
		if err != nil || parsed.Host == "" || parsed.Path != "" || parsed.User != nil {
			continue
		}
		domainHost, domainPort, err := net.SplitHostPort(parsed.Host)
		if err != nil {
			domainHost = parsed.Hostname()
			domainPort = parsed.Port()
		}
		appendOrigin(domainHost, domainPort)
	}
	if s.urlResolver != nil {
		for _, learnedHost := range s.urlResolver.GetLearnedHosts() {
			appendOrigin(learnedHost, "")
		}
	}
	return urls
}
