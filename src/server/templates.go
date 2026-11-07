package server

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"html"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	commonhttp "github.com/apimgr/airports/src/common/httputil"
	"github.com/apimgr/airports/src/common/i18n"
	"github.com/apimgr/airports/src/mode"
)

//go:embed template/layout/*.tmpl template/partial/*.tmpl template/partial/public/*.tmpl template/page/*.tmpl template/component/*.tmpl
var templateFS embed.FS

//go:embed static
var staticFS embed.FS

// pageFiles maps a page key (used by renderTemplate callers) to its
// content file under template/page/. Every page's file is parsed together
// with layout/public.tmpl and all shared partial/component files into its
// own isolated *template.Template (see initTemplates) rather than one
// shared set, because every page file independently defines
// {{define "content"}} — html/template's ParseFS resolves same-named
// define blocks across an entire parse set with "last one wins" semantics
// (silently, no error), so a single shared template.Template for all pages
// would make every page render whichever file happened to parse last.
var pageFiles = map[string]string{
	"home":           "index.tmpl",
	"search":         "search.tmpl",
	"nearby":         "nearby.tmpl",
	"airport":        "airport.tmpl",
	"stats":          "stats.tmpl",
	"geoip":          "geoip.tmpl",
	"server_about":   "server-about.tmpl",
	"server_help":    "server-help.tmpl",
	"server_privacy": "server-privacy.tmpl",
	"server_terms":   "server-terms.tmpl",
	"server_contact": "server-contact.tmpl",
	"server_security": "server-security.tmpl",
	"server_security_policy": "server-security-policy.tmpl",
	"server_security_thanks": "server-security-thanks.tmpl",
	"server_healthz": "healthz.tmpl",
	"error":          "error.tmpl",
}

// sharedTemplateFiles are the layout/partial/component files parsed into
// every page's isolated *template.Template alongside its own page/*.tmpl
// content file (see pageFiles and initTemplates).
var sharedTemplateFiles = []string{
	"template/layout/public.tmpl",
	"template/partial/head.tmpl",
	"template/partial/scripts.tmpl",
	"template/partial/public/header.tmpl",
	"template/partial/public/nav.tmpl",
	"template/partial/public/footer.tmpl",
	"template/component/modal.tmpl",
	"template/component/toast.tmpl",
	"template/component/cookie-consent.tmpl",
	"template/component/site-banner.tmpl",
}

var pageTemplates map[string]*template.Template

// i18nPlaceholderFuncs registers the "t"/"tf" function names at parse
// time so html/template accepts calls to them in template source. The
// real, language-bound implementations are attached to a per-request
// clone in renderTemplate — templates are parsed once at startup, long
// before any request (and its resolved language) exists.
var i18nPlaceholderFuncs = template.FuncMap{
	"t":              func(string) string { return "" },
	"tf":             func(string, ...interface{}) string { return "" },
	"markdownToHTML": renderPrivacyMarkdown,
	"humanize":       humanizePrivacyLabel,
}

var privacyBoldPattern = regexp.MustCompile(`\*\*(.+?)\*\*`)

func renderPrivacyMarkdown(source string) template.HTML {
	var out strings.Builder
	var paragraph []string
	listOpen := false
	flushParagraph := func() {
		if len(paragraph) == 0 {
			return
		}
		out.WriteString("<p>")
		out.WriteString(strings.Join(paragraph, " "))
		out.WriteString("</p>")
		paragraph = nil
	}
	closeList := func() {
		if listOpen {
			out.WriteString("</ul>")
			listOpen = false
		}
	}
	for _, line := range strings.Split(source, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			flushParagraph()
			closeList()
			continue
		}
		if strings.HasPrefix(line, "- ") {
			flushParagraph()
			if !listOpen {
				out.WriteString("<ul>")
				listOpen = true
			}
			out.WriteString("<li>")
			out.WriteString(renderPrivacyInline(line[2:]))
			out.WriteString("</li>")
			continue
		}
		closeList()
		paragraph = append(paragraph, renderPrivacyInline(line))
	}
	flushParagraph()
	closeList()
	return template.HTML(out.String())
}

func renderPrivacyInline(source string) string {
	escaped := html.EscapeString(source)
	return privacyBoldPattern.ReplaceAllString(escaped, "<strong>$1</strong>")
}

func humanizePrivacyLabel(value string) string {
	words := strings.Fields(strings.ReplaceAll(strings.ReplaceAll(value, "_", " "), "-", " "))
	for i, word := range words {
		if len(word) > 0 {
			words[i] = strings.ToUpper(word[:1]) + word[1:]
		}
	}
	return strings.Join(words, " ")
}

// initTemplates loads and parses all templates, one isolated
// *template.Template per page (layout/public.tmpl + all shared
// partial/component files + that page's own template/page/*.tmpl file) so
// no page's {{define "content"}} block can ever shadow another's.
func initTemplates() error {
	pageTemplates = make(map[string]*template.Template, len(pageFiles))
	for page, file := range pageFiles {
		files := append(append([]string{}, sharedTemplateFiles...), "template/page/"+file)
		t, err := template.New("public.tmpl").Funcs(i18nPlaceholderFuncs).ParseFS(templateFS, files...)
		if err != nil {
			return err
		}
		pageTemplates[page] = t
	}
	return nil
}

// renderTemplate renders the named page (a key of pageFiles, e.g. "home",
// "server_about") through public.tmpl with data. When data is a
// map[string]interface{}, "APIBase" (the versioned API base path, e.g.
// "/api/{api_version}" per AI.md PART 14) is injected automatically unless the
// caller already set it, so templates never need to hardcode "v1".
// "ThemeClass" is likewise injected from the "theme" cookie so the
// server renders theme-light/theme-dark/theme-auto directly on <html>
// with no init JS and no flash of unstyled content (AI.md PART 16).
// The cookie-consent banner fields ("ShowCookieConsent", "ConsentMessage",
// etc.) are injected the same way so public.tmpl can render the banner
// without every page handler wiring it manually (AI.md PART 16 "Cookie
// Consent Banner" — always enabled, server-rendered, works with zero JS).
// "Lang", "Dir", and "AvailableLanguages" are likewise injected from the
// request's resolved language (AI.md PART 30) so public.tmpl can render
// <html lang dir> and the zero-JS language-selector form without every
// handler wiring it manually. "CurrentYear" is injected from the server
// clock so footer copyright text never hardcodes a stale year (CLAUDE.md
// "Hardcode dev values -> Detect at runtime").
func (s *Server) renderTemplate(w http.ResponseWriter, r *http.Request, page string, data interface{}) {
	// Normal pages render with 200 OK. A pre-write failure (missing template,
	// clone/parse error) falls back to the themed 500 error page (AI.md PART
	// 16 — no generic/unstyled error pages).
	if err := s.executeTemplateStatus(w, r, page, http.StatusOK, data); err != nil {
		log.Printf("renderTemplate: %q failed before write: %v", page, err)
		s.renderErrorPage(w, r, http.StatusInternalServerError)
	}
}

// injectCommonTemplateData populates the layout-wide fields public.tmpl expects
// (API base, canonical URL, theme, language, cookie-consent banner, current
// year) into m, without clobbering values a caller already set.
func (s *Server) injectCommonTemplateData(r *http.Request, m map[string]interface{}) {
	if _, exists := m["BaseURL"]; !exists {
		m["BaseURL"] = strings.TrimSuffix(RequestBaseURL(r), "/")
	}
	if _, exists := m["APIBase"]; !exists {
		m["APIBase"] = strings.TrimSuffix(RequestBaseURL(r), "/") + s.APIBasePath()
	}
	if _, exists := m["CanonicalURL"]; !exists {
		m["CanonicalURL"] = s.publicBaseURL(r) + r.URL.Path
	}
	if _, exists := m["Robots"]; !exists {
		m["Robots"] = robotsDirective(r)
	}
	if _, exists := m["Description"]; !exists {
		m["Description"] = "Global airport location information API with GeoIP integration"
	}
	if _, exists := m["ThemeClass"]; !exists {
		m["ThemeClass"] = themeClassFromRequest(r)
	}
	if _, exists := m["ConsentPreferences"]; !exists {
		state := consentStateFromContext(r)
		if state == nil {
			state = consentFromRequest(r, s.config)
		}
		m["ConsentPreferences"] = state.Preferences
	}
	if _, exists := m["ConsentAnalytics"]; !exists {
		state := consentStateFromContext(r)
		if state == nil {
			state = consentFromRequest(r, s.config)
		}
		m["ConsentAnalytics"] = state.Analytics
	}
	privacy := s.config.Server.Privacy
	m["ConsentPreferencesDescription"] = privacy.Cookies.Preferences.Description
	m["ConsentAnalyticsDescription"] = privacy.GetAnalyticsDescription()
	m["PreferencesCategoryEnabled"] = privacy.Cookies.Preferences.Enabled
	m["AnalyticsCategoryEnabled"] = privacy.Cookies.Analytics.Enabled
	m["ConsentShowPreferences"] = privacy.Consent.ShowPreferences
	m["ConsentPreferencesText"] = privacy.Consent.PreferencesText
	m["ConsentPosition"] = privacy.Consent.Position
	if _, exists := m["Lang"]; !exists {
		lang := i18n.FromContext(r.Context())
		m["Lang"] = lang
		m["Dir"] = i18n.Direction(lang)
		m["AvailableLanguages"] = i18n.AvailableLanguages()
	}
	if _, exists := m["Announcements"]; !exists {
		m["Announcements"] = s.announcementViews(r)
	}
	if _, exists := m["ShowCookieConsent"]; !exists {
		m["ShowCookieConsent"] = privacy.Consent.ShowUntilAcknowledged && !hasValidConsentCookie(r)
		m["ConsentMessage"] = privacy.GetConsentMessage()
		m["ConsentPolicyURL"] = privacy.Consent.Policy.URL
		m["ConsentPolicyText"] = privacy.Consent.Policy.Text
		m["ConsentDeclineText"] = privacy.Consent.Buttons.Decline
		m["ConsentAcceptText"] = privacy.Consent.Buttons.Accept
		m["ConsentDataSold"] = privacy.Data.Sold
	}
	tracking := s.config.Server.Tracking
	m["TrackingEnabled"] = canLoadAnalytics(r, s.config) && !s.isTorRequest(r)
	m["TrackingType"] = tracking.Type
	m["TrackingID"] = tracking.ID
	m["TrackingURL"] = tracking.URL
	m["TrackingSimple"] = tracking.Type == "simple"
	if _, exists := m["CurrentYear"]; !exists {
		m["CurrentYear"] = time.Now().Year()
	}
	if _, exists := m["CSRFToken"]; !exists {
		m["CSRFToken"] = csrfTokenFromContext(r.Context())
	}
}

// executeTemplateStatus injects common layout data, sets the HTTP status code,
// and renders page's public.tmpl with the request's resolved language. It
// returns an error only for failures that occur BEFORE the first byte is
// written (so the caller may still substitute a different response); an
// execute failure after headers are sent is logged and returns nil, since the
// response can no longer be changed.
func (s *Server) executeTemplateStatus(w http.ResponseWriter, r *http.Request, page string, status int, data interface{}) error {
	// AI.md PART 6: template caching is enabled in production and disabled
	// (hot reload) in development. mode.ShouldCacheTemplates() is the
	// production-side default (templates parsed once in initTemplates at
	// startup); when it is false, re-parse on every request so edits under
	// template/page/ take effect without a restart.
	if !mode.ShouldCacheTemplates() && mode.ShouldEnableAutoReload() {
		if err := initTemplates(); err != nil {
			return err
		}
	}
	if m, ok := data.(map[string]interface{}); ok {
		s.injectCommonTemplateData(r, m)
	}
	t, ok := pageTemplates[page]
	if !ok {
		return fmt.Errorf("unknown template page %q", page)
	}
	lang := i18n.FromContext(r.Context())
	clone, err := t.Clone()
	if err != nil {
		return err
	}
	clone = clone.Funcs(template.FuncMap{
		"t": func(key string) string {
			return i18n.T(lang, key)
		},
		"tf": func(key string, args ...interface{}) string {
			return i18n.Tf(lang, key, args...)
		},
	})
	var rendered bytes.Buffer
	if err := clone.ExecuteTemplate(&rendered, "public.tmpl", data); err != nil {
		return err
	}
	format := commonhttp.GetFrontendResponseFormat(r)
	w.Header().Add("Vary", "Accept")
	w.Header().Add("Vary", "User-Agent")
	contentType := "text/html; charset=utf-8"
	body := rendered.Bytes()
	switch format {
	case "json":
		contentType = "application/json; charset=utf-8"
		jsonData := frontendJSONData(data)
		encoded, err := json.Marshal(jsonData)
		if err != nil {
			return fmt.Errorf("encode frontend JSON: %w", err)
		}
		body = encoded
	case "text":
		contentType = "text/plain; charset=utf-8"
		body = []byte(commonhttp.HTML2TextConverter(rendered.String(), 80))
	case "nojs":
		var noJS string
		noJS, err = commonhttp.RenderNoJSHTML(rendered.String())
		if err != nil {
			return err
		}
		body = []byte(noJS)
	}
	w.Header().Set("Content-Type", contentType)
	if r.Method == http.MethodGet && status >= http.StatusOK && status < http.StatusMultipleChoices &&
		strings.HasPrefix(contentType, "text/html") && !s.isTorRequest(r) && s.config.Server.Tor.OnionAddress != "" {
		onionURL := url.URL{
			Scheme:   "http",
			Host:     strings.TrimSpace(s.config.Server.Tor.OnionAddress),
			Path:     r.URL.Path,
			RawQuery: r.URL.RawQuery,
		}
		w.Header().Set("Onion-Location", onionURL.String())
	}
	w.WriteHeader(status)
	if _, err := w.Write(body); err != nil {
		log.Printf("executeTemplateStatus: writing %q failed after headers sent: %v", page, err)
	}
	return nil
}

// frontendJSONData removes layout and request-security values from the
// page-specific data before returning it to the first-party CLI client.
func frontendJSONData(data interface{}) interface{} {
	values, ok := data.(map[string]interface{})
	if !ok {
		return data
	}
	result := make(map[string]interface{}, len(values))
	for key, value := range values {
		switch key {
		case "CSRFToken", "BaseURL", "CanonicalURL", "Robots", "ThemeClass", "Lang", "Dir",
			"AvailableLanguages", "Announcements", "ShowCookieConsent", "ConsentMessage",
			"ConsentPolicyURL", "ConsentPolicyText", "ConsentDeclineText", "ConsentAcceptText",
			"ConsentDataSold", "CurrentYear":
			continue
		default:
			result[key] = value
		}
	}
	return result
}

// robotsDirective computes the per-route <meta name="robots"> value per AI.md
// PART 16 "Robots Directive". Only routes explicitly published as public
// content (publicSitemapEntries, plus individual airport detail pages) are
// indexable; everything else — API, health, debug, internal and alias routes —
// fails closed to "noindex,nofollow".
func robotsDirective(r *http.Request) string {
	if r == nil {
		return "noindex,nofollow"
	}
	path := strings.TrimSuffix(r.URL.Path, "/")
	if path == "" {
		path = "/"
	}
	for _, e := range publicSitemapEntries {
		if path == e.Path {
			return "index,follow"
		}
	}
	// Individual airport detail pages are the primary public content; the
	// search and nearby tools under the same prefix are already listed above.
	if ident := strings.TrimPrefix(path, "/airports/"); ident != path && ident != "" && !strings.Contains(ident, "/") {
		return "index,follow"
	}
	return "noindex,nofollow"
}

// themeClassFromRequest resolves the "theme" cookie to a CSS class name
// ("theme-light", "theme-dark", or "theme-auto"), defaulting to
// "theme-dark" (the app's default theme) when the cookie is absent or
// holds an unrecognized value.
func themeClassFromRequest(r *http.Request) string {
	if r == nil {
		return "theme-dark"
	}
	if state := consentStateFromContext(r); state != nil && !state.Preferences {
		return "theme-dark"
	}
	c, err := r.Cookie("theme")
	if err != nil {
		return "theme-dark"
	}
	switch c.Value {
	case "light":
		return "theme-light"
	case "auto":
		return "theme-auto"
	case "dark":
		return "theme-dark"
	default:
		return "theme-dark"
	}
}
