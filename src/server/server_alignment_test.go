package server

import (
	"bytes"
	"context"
	"encoding/json"
	"html"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/apimgr/airports/src/common/i18n"
	"github.com/apimgr/airports/src/common/urlvars"
	"github.com/apimgr/airports/src/config"
	"github.com/apimgr/airports/src/db"
)

func TestServerPageAPIContracts(t *testing.T) {
	s := newTestServer(t)
	cases := []struct {
		name    string
		handler http.HandlerFunc
		keys    []string
	}{
		{"about", s.handleServerAboutAPI, []string{`"title"`, `"content"`, `"format"`}},
		{"help", s.handleServerHelpAPI, []string{`"title"`, `"content"`, `"format"`, `"pages"`}},
		{"terms", s.handleServerTermsAPI, []string{`"title"`, `"content"`, `"format"`, `without regard to conflict-of-law rules`}},
		{"mcp help", s.handleServerMCPHelpAPI, []string{`"enabled"`, `"url"`, `"aliases"`, `"auth"`, `"tools"`, `"project"`}},
		{"privacy", s.handleServerPrivacyAPI, []string{`"cookies"`, `"retention"`, `"sharing"`, `"data_security"`}},
		{"security", s.handleServerSecurityAPI, []string{`"contacts"`, `"expires"`, `"pgp"`, `"policy_url"`, `"thanks_url"`}},
		{"security policy", s.handleServerSecurityPolicyAPI, []string{`"policy"`, `Scope`, `Safe harbor`, `90 days`}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/api/v1/server/"+strings.ReplaceAll(tc.name, " ", "/"), nil)
			w := httptest.NewRecorder()
			tc.handler(w, r)
			if w.Code != http.StatusOK {
				t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
			}
			var body map[string]interface{}
			if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			for _, key := range tc.keys {
				if !strings.Contains(w.Body.String(), key) {
					t.Errorf("response missing %s: %s", key, w.Body.String())
				}
			}
		})
	}
}

func TestMCPHelpUsesToolsListAndShellSafeCommands(t *testing.T) {
	s := newTestServer(t)
	request := httptest.NewRequest(http.MethodGet, "/server/help/mcp", nil)
	request.Host = "example.test;$(touch /tmp/pwned)"
	request.Header.Set("Accept", "text/html")
	response := httptest.NewRecorder()
	s.handleServerMCPHelp(response, request)
	body := html.UnescapeString(response.Body.String())
	for _, required := range []string{
		"claude mcp add --transport http airports",
		"MCP-Protocol-Version: " + currentMCPVersion,
		"Mcp-Method: tools/list",
		`"method":"tools/list"`,
		"--url '",
	} {
		if !strings.Contains(body, required) {
			t.Errorf("MCP help is missing %q", required)
		}
	}
	if strings.Contains(body, "Mcp-Method: initialize") {
		t.Fatalf("MCP help contains an initialize handshake: %s", body)
	}
	if got, want := shellSingleQuote("https://example.test/$(touch pwned)'x"), `'https://example.test/$(touch pwned)'"'"'x'`; got != want {
		t.Fatalf("shellSingleQuote() = %q, want %q", got, want)
	}
}

func TestPrivacyPageAndAPILocalizeBuiltInContent(t *testing.T) {
	s := newTestServer(t)
	r := httptest.NewRequest(http.MethodGet, "/server/privacy", nil)
	r = r.WithContext(i18n.SetLanguage(r.Context(), "fr"))
	w := httptest.NewRecorder()
	s.handleServerPrivacy(w, r)
	body := w.Body.String()
	if w.Code != http.StatusOK || !strings.Contains(body, "Comment nous protégeons les données") || strings.Contains(body, "How we protect your data") {
		t.Fatalf("privacy page did not localize built-in copy: status=%d body=%q", w.Code, body)
	}

	r = httptest.NewRequest(http.MethodGet, "/api/v1/server/privacy", nil)
	r = r.WithContext(i18n.SetLanguage(r.Context(), "fr"))
	w = httptest.NewRecorder()
	s.handleServerPrivacyAPI(w, r)
	var privacyResponse struct {
		Data struct {
			Content struct {
				DataUsage string `json:"data_usage"`
			} `json:"content"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &privacyResponse); err != nil {
		t.Fatalf("decode localized privacy API response: %v", err)
	}
	if w.Code != http.StatusOK || privacyResponse.Data.Content.DataUsage != i18n.TranslateText("fr", "privacy.data_usage_default") || strings.Contains(privacyResponse.Data.Content.DataUsage, "Airports uses information processed to") {
		t.Fatalf("privacy API did not localize built-in copy: status=%d body=%q", w.Code, w.Body.String())
	}
}

func TestTermsPageRendersGoverningLaw(t *testing.T) {
	s := newTestServer(t)
	r := httptest.NewRequest(http.MethodGet, "/server/terms", nil)
	r.Header.Set("Accept", "text/html")
	w := httptest.NewRecorder()
	s.renderTemplate(w, r, "server_terms", map[string]interface{}{"Title": "Terms"})
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `id="terms-governing-law"`) || !strings.Contains(w.Body.String(), "laws of the United States, without regard to conflict-of-law rules") {
		t.Fatalf("terms page is missing the governing-law section: status=%d body=%q", w.Code, w.Body.String())
	}
}

func TestErrorPageStatusTitleIsLocalized(t *testing.T) {
	s := newTestServer(t)
	r := httptest.NewRequest(http.MethodGet, "/missing", nil)
	r.Header.Set("Accept", "text/html")
	r = r.WithContext(i18n.SetLanguage(r.Context(), "fr"))
	w := httptest.NewRecorder()
	s.renderErrorPage(w, r, http.StatusNotFound)
	if w.Code != http.StatusNotFound || !strings.Contains(w.Body.String(), "Erreur 404") || strings.Contains(w.Body.String(), "Not Found") {
		t.Fatalf("French error title missing or English text leaked: status=%d body=%q", w.Code, w.Body.String())
	}
}

func TestThemeToggleUsesPreferencesAndCurrentMode(t *testing.T) {
	s := newTestServer(t)
	r := httptest.NewRequest(http.MethodGet, "/server/about", nil)
	r.Header.Set("Accept", "text/html")
	r.AddCookie(&http.Cookie{Name: themeCookieName, Value: "auto"})
	r = r.WithContext(i18n.SetLanguage(r.Context(), "fr"))
	w := httptest.NewRecorder()
	s.renderTemplate(w, r, "server_about", map[string]interface{}{})
	body := w.Body.String()
	accessibleBody := html.UnescapeString(body)
	if w.Code != http.StatusOK || !strings.Contains(body, `action="/server/preferences"`) || !strings.Contains(body, `name="theme" value="dark"`) || !strings.Contains(accessibleBody, `aria-label="Changer de thème (actuel : Auto)"`) || !strings.Contains(accessibleBody, "<title>"+i18n.TranslateText("fr", "about.title")) {
		t.Fatalf("theme toggle does not follow the current mode and canonical save route: status=%d body=%q", w.Code, body)
	}
}

func TestServerPageAPIUsesOperatorContentSource(t *testing.T) {
	s := newTestServer(t)
	s.config.Server.Branding.Content = map[string]config.BrandingContent{
		"about": {Format: "md", Body: map[string]interface{}{"default": "Run by {project_name}.", "fr": "Géré par {project_name}."}},
		"terms": {Format: "txt", Body: "Terms for {project_org}."},
	}
	r := httptest.NewRequest(http.MethodGet, "/api/v1/server/about", nil)
	r = r.WithContext(i18n.SetLanguage(r.Context(), "fr"))
	w := httptest.NewRecorder()
	s.handleServerAboutAPI(w, r)
	if !strings.Contains(w.Body.String(), `"format": "markdown"`) || !strings.Contains(w.Body.String(), `Géré par airports.`) {
		t.Fatalf("About API did not return localized source content: %s", w.Body.String())
	}
	r = httptest.NewRequest(http.MethodGet, "/api/v1/server/terms", nil)
	w = httptest.NewRecorder()
	s.handleServerTermsAPI(w, r)
	if !strings.Contains(w.Body.String(), `"format": "text"`) || !strings.Contains(w.Body.String(), `Terms for apimgr.`) {
		t.Fatalf("Terms API did not return configured source content: %s", w.Body.String())
	}
}

func TestDefaultSecurityPolicyContainsRequiredSectionsAndScope(t *testing.T) {
	s := newTestServer(t)
	s.config.Server.FQDN = "ignored.example.org"
	s.config.Server.Tor.OnionAddress = "example.onion"
	s.urlResolver = urlvars.NewURLResolverManager(urlvars.URLResolverOptions{DomainEnv: "airports.example.com"})
	r := httptest.NewRequest(http.MethodGet, "/server/security/policy", nil)
	policy := s.defaultSecurityPolicy(r)
	for _, want := range []string{"airports.example.com", "example.onion", "## Scope", "## Out of scope", "## Coordinated disclosure", "## Safe harbor", "## Reporting", "90 days", "/server/security"} {
		if !strings.Contains(policy, want) {
			t.Errorf("default security policy missing %q", want)
		}
	}
}

func TestDefaultSecurityPolicyUsesRequestLanguage(t *testing.T) {
	s := newTestServer(t)
	s.config.Server.FQDN = "ignored.example.org"
	s.urlResolver = urlvars.NewURLResolverManager(urlvars.URLResolverOptions{DomainEnv: "airports.example.com"})
	r := httptest.NewRequest(http.MethodGet, "/server/security/policy", nil)
	r = r.WithContext(i18n.SetLanguage(r.Context(), "fr"))
	policy := s.defaultSecurityPolicy(r)
	for _, want := range []string{"## Périmètre", "## Divulgation coordonnée", "page de sécurité", "airports.example.com"} {
		if !strings.Contains(policy, want) {
			t.Errorf("French security policy missing %q: %s", want, policy)
		}
	}
	if strings.Contains(policy, "## Scope") {
		t.Error("French security policy contains an untranslated English heading")
	}
}

func TestSitemapAndRobotsRespectConfiguredRules(t *testing.T) {
	entries := sitemapEntries([]string{"/"}, []string{"/server/help"})
	for _, entry := range entries {
		if entry.Path == "/server/help" {
			t.Fatal("sitemap included a route disallowed by robots.txt")
		}
	}
	if !robotsPathAllowed("/server/about", []string{"/"}, nil) || robotsPathAllowed("/server/help", []string{"/"}, []string{"/server/help"}) {
		t.Fatal("robots path filtering did not apply allow minus deny rules")
	}
}

func TestSEOBrandingIsRenderedInPublicPageHead(t *testing.T) {
	s := newTestServer(t)
	s.config.Server.Branding.Title = "Airport Directory"
	s.config.Server.Branding.Tagline = "Find your next destination"
	s.config.Server.Branding.Description = "A directory of airports"
	s.config.Server.SEO.Keywords = []string{"airport", "directory"}
	s.config.Server.SEO.Author = "Airport Team"
	s.config.Server.SEO.MetaTags = map[string]string{"application-name": "Airport Directory"}
	s.config.Server.SEO.Verification.Google = "abc_123"
	s.config.Server.SEO.OpenGraph.Enabled = true
	s.config.Server.SEO.Twitter.Enabled = true
	r := httptest.NewRequest(http.MethodGet, "/server/about", nil)
	r.Header.Set("Accept", "text/html")
	w := httptest.NewRecorder()
	s.renderTemplate(w, r, "server_about", map[string]interface{}{"Title": "About"})
	body := html.UnescapeString(w.Body.String())
	for _, want := range []string{
		`<title>About Airports API - Airport Directory - Find your next destination</title>`,
		`<meta name="description" content="A directory of airports">`,
		`<meta name="keywords" content="airport, directory">`,
		`<meta name="author" content="Airport Team">`,
		`<meta name="application-name" content="Airport Directory">`,
		`<meta name="google-site-verification" content="abc_123">`,
	} {
		if !strings.Contains(body, want) {
			titleStart := strings.Index(body, "<title>")
			titleEnd := strings.Index(body, "</title>")
			if titleStart >= 0 && titleEnd > titleStart {
				t.Errorf("page head is missing %q; rendered title is %q", want, body[titleStart:titleEnd+len("</title>")])
			} else {
				t.Errorf("page head is missing %q", want)
			}
		}
	}
}

func TestPreferencesPageAndSave(t *testing.T) {
	s := newTestServer(t)
	r := httptest.NewRequest(http.MethodGet, "/server/preferences", nil)
	r.Header.Set("Accept", "text/html")
	w := httptest.NewRecorder()
	s.handleServerPreferences(w, r)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `name="consent_form"`) || !strings.Contains(w.Body.String(), `popovertarget="preferences-reset"`) || !strings.Contains(w.Body.String(), `Sync preferences`) {
		t.Fatalf("preferences page status/body = %d %q", w.Code, w.Body.String())
	}

	r = httptest.NewRequest(http.MethodPost, "/server/preferences", strings.NewReader("lang=fr&theme=dark"))
	r.Header.Set("Accept", "text/html")
	r = r.WithContext(context.WithValue(r.Context(), consentContextKey{}, &ConsentState{Preferences: true}))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w = httptest.NewRecorder()
	s.handleServerPreferencesSave(w, r)
	if w.Code != http.StatusSeeOther || len(w.Result().Cookies()) != 2 {
		t.Fatalf("save status/cookies = %d/%v", w.Code, w.Result().Cookies())
	}
	if got := w.Header().Get("Location"); got != "/" {
		t.Fatalf("redirect = %q", got)
	}
	r = httptest.NewRequest(http.MethodPost, "/server/preferences", strings.NewReader("lang=fr&theme=invalid"))
	r = r.WithContext(context.WithValue(r.Context(), consentContextKey{}, &ConsentState{Preferences: true}))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w = httptest.NewRecorder()
	s.handleServerPreferencesSave(w, r)
	if w.Code != http.StatusSeeOther || len(w.Result().Cookies()) != 1 {
		t.Fatalf("invalid theme should be dropped while valid language is saved: status=%d cookies=%v", w.Code, w.Result().Cookies())
	}
}

func TestMCPRequestFlow(t *testing.T) {
	s := newTestServer(t)
	s.config.Server.Token = "adm_test-token"
	tools := s.mcpTools()
	if len(tools) == 0 {
		t.Fatal("MCP tool registry is empty")
	}
	var graphqlTool *mcpTool
	for i := range tools {
		if tools[i].path == "/server/graphql" {
			graphqlTool = &tools[i]
		}
		if strings.HasPrefix(tools[i].path, "/server/preferences/tokens") {
			t.Errorf("token management route %q must not be exposed as an MCP tool", tools[i].path)
		}
	}
	if graphqlTool == nil {
		t.Fatal("GraphQL operation is missing from MCP tools")
	}
	graphqlProperties := graphqlTool.InputSchema["properties"].(map[string]interface{})
	if graphqlProperties["query"] == nil {
		t.Fatal("GraphQL MCP tool has no query input schema")
	}
	required, _ := graphqlTool.InputSchema["required"].([]string)
	if len(required) != 1 || required[0] != "query" {
		t.Fatalf("GraphQL required inputs = %v, want [query]", required)
	}
	list := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28"}}}`))
	list.Header.Set("Content-Type", "application/json")
	list.Header.Set("Authorization", "Bearer adm_test-token")
	list.Header.Set("MCP-Protocol-Version", currentMCPVersion)
	list.Header.Set("Mcp-Method", "tools/list")
	list.Host = "example.com"
	response := httptest.NewRecorder()
	s.handleMCP(response, list)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"tools"`) {
		t.Fatalf("tools/list status/body = %d %q", response.Code, response.Body.String())
	}

	missingToken := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`))
	missingToken.Host = "example.com"
	response = httptest.NewRecorder()
	s.handleMCP(response, missingToken)
	if response.Code != http.StatusUnauthorized || response.Header().Get("WWW-Authenticate") == "" {
		t.Fatalf("missing-token status/headers = %d %v", response.Code, response.Header())
	}

	badOrigin := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"ping"}`))
	badOrigin.Host = "example.com"
	badOrigin.Header.Set("Origin", "https://attacker.example")
	response = httptest.NewRecorder()
	s.handleMCP(response, badOrigin)
	if response.Code != http.StatusForbidden {
		t.Fatalf("untrusted origin status = %d, want %d", response.Code, http.StatusForbidden)
	}

	unsupported := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":2,"method":"ping","params":{"_meta":{"io.modelcontextprotocol/protocolVersion":"1900-01-01"}}}`))
	unsupported.Host = "example.com"
	unsupported.Header.Set("Authorization", "Bearer adm_test-token")
	unsupported.Header.Set("MCP-Protocol-Version", "1900-01-01")
	unsupported.Header.Set("Mcp-Method", "ping")
	response = httptest.NewRecorder()
	s.handleMCP(response, unsupported)
	if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), "Unsupported protocol version") {
		t.Fatalf("unsupported version status/body = %d %q", response.Code, response.Body.String())
	}

	legacy := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":3,"method":"initialize","params":{"protocolVersion":"2025-11-25"}}`))
	legacy.Host = "example.com"
	legacy.Header.Set("Authorization", "Bearer adm_test-token")
	response = httptest.NewRecorder()
	s.handleMCP(response, legacy)
	if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), "2026-07-28") {
		t.Fatalf("legacy initialize status/body = %d %q", response.Code, response.Body.String())
	}
}

func TestContactAPIIssuesFreshCaptcha(t *testing.T) {
	s := contactTestServer(t)
	requestContact := func() map[string]interface{} {
		r := httptest.NewRequest(http.MethodGet, "/api/v1/server/contact?type=abuse", nil)
		w := httptest.NewRecorder()
		s.handleServerContactAPI(w, r)
		if w.Code != http.StatusOK {
			t.Fatalf("contact API status/body = %d %q", w.Code, w.Body.String())
		}
		var result struct {
			Data map[string]interface{} `json:"data"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
			t.Fatalf("decode contact API response: %v", err)
		}
		return result.Data["captcha"].(map[string]interface{})
	}
	first, second := requestContact(), requestContact()
	if first["provider"] != "simple" || first["captcha_id"] == "" || first["question"] == "" || first["captcha_id"] == second["captcha_id"] {
		t.Fatalf("contact CAPTCHA responses are incomplete or reused: %#v %#v", first, second)
	}
}

func TestContactAPIDoesNotEchoSecurityID(t *testing.T) {
	s := contactTestServer(t)
	secret := bytes.Repeat([]byte{0x35}, 32)
	if _, err := s.db.Exec(`UPDATE app_secrets SET value = ? WHERE name = 'installation_secret'`, db.EncodeAppSecret("installation_secret", secret)); err != nil {
		t.Fatal(err)
	}
	id := securityIDAtSecret(time.Now().UTC(), secret)
	r := httptest.NewRequest(http.MethodGet, "/api/v1/server/contact?type=security&security_id="+id, nil)
	w := httptest.NewRecorder()
	s.handleServerContactAPI(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("contact API status/body = %d %q", w.Code, w.Body.String())
	}
	if strings.Contains(w.Body.String(), id) {
		t.Fatalf("contact API echoed the one-shot security id: %s", w.Body.String())
	}
}
