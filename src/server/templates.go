package server

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	commonhttp "github.com/apimgr/airports/src/common/httputil"
	"github.com/apimgr/airports/src/common/i18n"
	"github.com/apimgr/airports/src/config"
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
	"home":                    "index.tmpl",
	"airport_list":            "airport-list.tmpl",
	"search":                  "search.tmpl",
	"nearby":                  "nearby.tmpl",
	"airport":                 "airport.tmpl",
	"stats":                   "stats.tmpl",
	"geoip":                   "geoip.tmpl",
	"server_about":            "server-about.tmpl",
	"server_help":             "server-help.tmpl",
	"server_help_mcp":         "server-help-mcp.tmpl",
	"server_docs_swagger":     "server-docs-swagger.tmpl",
	"server_docs_graphql":     "server-docs-graphql.tmpl",
	"server_privacy":          "server-privacy.tmpl",
	"server_preferences":      "server-preferences.tmpl",
	"server_preferences_sync": "server-preferences-sync.tmpl",
	"server_owner_tokens":     "server-tokens.tmpl",
	"server_dpo":              "server-dpo.tmpl",
	"server_terms":            "server-terms.tmpl",
	"server_contact":          "server-contact.tmpl",
	"server_security":         "server-security.tmpl",
	"server_security_policy":  "server-security-policy.tmpl",
	"server_security_report":  "server-security-report.tmpl",
	"server_security_thanks":  "server-security-thanks.tmpl",
	"server_healthz":          "healthz.tmpl",
	"error":                   "error.tmpl",
}

var pageTitleTranslationKeys = map[string]string{
	"home":                    "home.title",
	"airport_list":            "airport_list.title",
	"search":                  "search.title",
	"nearby":                  "nearby.title",
	"stats":                   "stats.title",
	"geoip":                   "geoip.title",
	"server_about":            "about.title",
	"server_help":             "help.title",
	"server_help_mcp":         "help.mcp_title",
	"server_docs_swagger":     "swagger.page_title",
	"server_docs_graphql":     "graphql.playground_title",
	"server_privacy":          "privacy.title",
	"server_preferences":      "nav.preferences",
	"server_preferences_sync": "preferences.sync_title",
	"server_owner_tokens":     "owner_tokens.title",
	"server_dpo":              "dpo.title",
	"server_terms":            "terms.title",
	"server_contact":          "contact.title",
	"server_security":         "security.title",
	"server_security_policy":  "security.policy_title",
	"server_security_report":  "security.status_title",
	"server_security_thanks":  "security.thanks_title",
	"server_healthz":          "health.title",
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
	"t":                  func(string) string { return "" },
	"tf":                 func(string, ...interface{}) string { return "" },
	"clientTranslations": func() string { return "{}" },
	"nextTheme":          func(string) string { return "dark" },
	"themeLabel":         func(string) string { return "" },
	"path":               func(string) string { return "" },
	"operatorHTML":       func(string) template.HTML { return "" },
	"operatorContentSet": func(string) bool { return false },
	"markdownToHTML": func(source string) template.HTML {
		return template.HTML(sanitizeOperatorHTML(renderOperatorMarkdown(source), false))
	},
	"humanize":         humanizePrivacyLabel,
	"healthCheckLabel": func(string) string { return "" },
}

var seoMetaNamePattern = regexp.MustCompile(`^[a-z0-9:_.-]+$`)
var seoCustomTagNamePattern = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)
var seoVerificationValuePatterns = map[string]*regexp.Regexp{
	"google":    regexp.MustCompile(`^[A-Za-z0-9_-]{1,43}$`),
	"bing":      regexp.MustCompile(`^[A-F0-9]{1,32}$`),
	"yandex":    regexp.MustCompile(`^[a-f0-9]{1,32}$`),
	"baidu":     regexp.MustCompile(`^[A-Za-z0-9]{1,32}$`),
	"pinterest": regexp.MustCompile(`^[a-f0-9]{1,32}$`),
	"facebook":  regexp.MustCompile(`^[a-z0-9]{1,64}$`),
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

type privacyDisplayText struct {
	ConsentMessage               string
	EssentialCookieDescription   string
	PreferencesCookieDescription string
	AnalyticsCookieDescription   string
	RetentionPeriod              string
	DataUsage                    string
	DataSecurity                 string
	Sharing                      []privacySharingText
}

type privacySharingText struct {
	Condition   string
	When        string
	Data        string
	Description string
}

func localizedPrivacyDisplayText(privacy config.PrivacyConfig, lang string) privacyDisplayText {
	defaults := config.DefaultApplicationConfig().Server.Privacy
	text := privacyDisplayText{
		ConsentMessage:               privacy.GetConsentMessage(),
		EssentialCookieDescription:   privacy.Cookies.Essential.Description,
		PreferencesCookieDescription: privacy.Cookies.Preferences.Description,
		AnalyticsCookieDescription:   privacy.GetAnalyticsDescription(),
		RetentionPeriod:              privacy.Retention.Period,
		DataUsage:                    privacy.GetDataUsageContent(),
		DataSecurity:                 privacy.Content.DataSecurity,
		Sharing:                      make([]privacySharingText, 0, len(privacy.Data.Sharing)),
	}
	if privacy.Consent.Message == defaults.Consent.Message && !privacy.Data.Sold {
		text.ConsentMessage = i18n.TranslateText(lang, "consent.banner_text")
	}
	if privacy.Consent.MessageIfSold == defaults.Consent.MessageIfSold && privacy.Data.Sold {
		text.ConsentMessage = i18n.TranslateText(lang, "privacy.consent_message_sold")
	}
	if privacy.Cookies.Essential.Description == defaults.Cookies.Essential.Description {
		text.EssentialCookieDescription = i18n.TranslateText(lang, "consent.essential_desc")
	}
	if privacy.Cookies.Preferences.Description == defaults.Cookies.Preferences.Description {
		text.PreferencesCookieDescription = i18n.TranslateText(lang, "consent.preferences_desc")
	}
	if privacy.Cookies.Analytics.Description == defaults.Cookies.Analytics.Description &&
		privacy.Cookies.Analytics.DescriptionSuffixNotSold == defaults.Cookies.Analytics.DescriptionSuffixNotSold &&
		privacy.Cookies.Analytics.DescriptionSuffixSold == defaults.Cookies.Analytics.DescriptionSuffixSold {
		key := "privacy.analytics_not_sold"
		if privacy.Data.Sold {
			key = "privacy.analytics_sold"
		}
		text.AnalyticsCookieDescription = i18n.TranslateText(lang, key)
	}
	if privacy.Retention.Period == defaults.Retention.Period {
		text.RetentionPeriod = i18n.TranslateText(lang, "privacy.retention_default")
	}
	if privacy.Content.DataUsage == defaults.Content.DataUsage {
		text.DataUsage = i18n.TranslateText(lang, "privacy.data_usage_default")
	}
	if privacy.Content.DataUsageIfSold == defaults.Content.DataUsageIfSold && privacy.Data.Sold {
		text.DataUsage = i18n.TranslateText(lang, "privacy.data_usage_sold")
	}
	if privacy.Content.DataSecurity == defaults.Content.DataSecurity {
		text.DataSecurity = i18n.TranslateText(lang, "privacy.data_security_default")
	}
	for index, condition := range privacy.Data.Sharing {
		sharing := privacySharingText{Condition: condition.Condition, When: condition.When, Data: condition.Data}
		if index < len(defaults.Data.Sharing) && condition == defaults.Data.Sharing[index] {
			keys := map[string][3]string{
				"analytics":      {"privacy.sharing_analytics_when", "privacy.sharing_analytics_data", "privacy.sharing_analytics"},
				"email":          {"privacy.sharing_email_when", "privacy.sharing_email_data", "privacy.sharing_email"},
				"user_initiated": {"privacy.sharing_user_initiated_when", "privacy.sharing_user_initiated_data", "privacy.sharing_user_initiated"},
			}
			if keys, ok := keys[condition.Condition]; ok {
				sharing.When = i18n.TranslateText(lang, keys[0])
				sharing.Data = i18n.TranslateText(lang, keys[1])
				sharing.Description = i18n.TranslateText(lang, keys[2])
				text.Sharing = append(text.Sharing, sharing)
				continue
			}
		}
		sharing.Description = fmt.Sprintf("%s: %s — %s", humanizePrivacyLabel(condition.Condition), condition.When, condition.Data)
		text.Sharing = append(text.Sharing, sharing)
	}
	return text
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
	robotValue := s.robotsDirective(r)
	s.configMu.RLock()
	noIndex := s.config != nil && s.config.Server.SEO.NoIndex
	s.configMu.RUnlock()
	if noIndex {
		robotValue = "noindex,nofollow"
	}
	w.Header().Set("X-Robots-Tag", robotValue)
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
	s.configMu.RLock()
	cfg := s.config
	s.configMu.RUnlock()
	if cfg == nil {
		return
	}
	branding := cfg.Server.Branding
	seo := cfg.Server.SEO
	separator := seo.TitleSeparator
	if len(separator) > 8 {
		separator = " - "
	}
	if strings.TrimSpace(separator) == "" {
		separator = " - "
	}
	if _, exists := m["BrandTitle"]; !exists {
		m["BrandTitle"] = branding.Title
	}
	if _, exists := m["BrandTagline"]; !exists {
		m["BrandTagline"] = branding.Tagline
	}
	if _, exists := m["LogoURL"]; !exists {
		m["LogoURL"] = s.brandingAssetURL(r, branding.Logo, "")
	}
	if _, exists := m["LogoSrcSet"]; !exists {
		m["LogoSrcSet"] = s.brandingAssetSrcSet(r, branding.Logo, "", 50, 200)
	}
	if _, exists := m["FaviconURL"]; !exists {
		m["FaviconURL"] = s.brandingAssetSizeURL(r, branding.Favicon, "/static/favicon.png", 32, 32)
	}
	if _, exists := m["Favicon16URL"]; !exists {
		m["Favicon16URL"] = s.brandingAssetSizeURL(r, branding.Favicon, "/static/favicon.png", 16, 16)
	}
	if _, exists := m["Favicon48URL"]; !exists {
		m["Favicon48URL"] = s.brandingAssetSizeURL(r, branding.Favicon, "/static/favicon.png", 48, 48)
	}
	if _, exists := m["Favicon192URL"]; !exists {
		m["Favicon192URL"] = s.brandingAssetSizeURL(r, branding.Favicon, "/static/favicon.png", 192, 192)
	}
	if _, exists := m["Favicon512URL"]; !exists {
		m["Favicon512URL"] = s.brandingAssetSizeURL(r, branding.Favicon, "/static/favicon.png", 512, 512)
	}
	if _, exists := m["AppleTouchIconURL"]; !exists {
		m["AppleTouchIconURL"] = s.brandingAssetSizeURL(r, branding.Favicon, "/static/favicon.png", 180, 180)
	}
	if _, exists := m["AccentCSS"]; !exists {
		accent, ok := parseAccentColor(strings.TrimSpace(cfg.Web.UI.Accent))
		m["AccentCSS"] = ok
		if _, exists := m["ThemeColor"]; !exists {
			m["ThemeColor"] = accentHex(accent)
		}
	}
	if _, exists := m["ThemeColor"]; !exists {
		m["ThemeColor"] = "#bd93f9"
	}
	if _, exists := m["TitleSeparator"]; !exists {
		m["TitleSeparator"] = separator
	}
	if _, exists := m["Description"]; !exists || strings.TrimSpace(fmt.Sprint(m["Description"])) == "" {
		if branding.Description != "" {
			m["Description"] = branding.Description
		} else {
			m["Description"] = i18n.TranslateText(i18n.FromContext(r.Context()), "common.default_description")
		}
	}
	if _, exists := m["Keywords"]; !exists {
		m["Keywords"] = strings.Join(seo.Keywords, ", ")
	}
	if _, exists := m["SEOAuthor"]; !exists {
		m["SEOAuthor"] = seo.Author
	}
	if _, exists := m["OGImage"]; !exists {
		m["OGImage"] = s.brandingAssetAbsoluteSizeURL(r, seo.OGImage, "", 1200, 630)
	}
	if _, exists := m["OpenGraphEnabled"]; !exists {
		m["OpenGraphEnabled"] = seo.OpenGraph.Enabled
	}
	if _, exists := m["OpenGraphType"]; !exists {
		m["OpenGraphType"] = firstNonEmpty(seo.OpenGraph.Type, "website")
	}
	if _, exists := m["OpenGraphSiteName"]; !exists {
		m["OpenGraphSiteName"] = firstNonEmpty(seo.OpenGraph.SiteName, branding.Title)
	}
	if _, exists := m["TwitterEnabled"]; !exists {
		m["TwitterEnabled"] = seo.Twitter.Enabled
	}
	if _, exists := m["TwitterCard"]; !exists {
		m["TwitterCard"] = firstNonEmpty(seo.Twitter.Card, "summary_large_image")
	}
	if _, exists := m["TwitterHandle"]; !exists {
		m["TwitterHandle"] = seo.TwitterHandle
	}
	if _, exists := m["TwitterCreator"]; !exists {
		m["TwitterCreator"] = seo.Twitter.Creator
	}
	if _, exists := m["MetaTags"]; !exists {
		m["MetaTags"] = seoMetaTagViews(seo.MetaTags)
	}
	if _, exists := m["VerificationTags"]; !exists {
		m["VerificationTags"] = seoVerificationTagViews(seo.Verification)
	}
	if seo.Canonical != "" {
		if canonical, err := url.Parse(seo.Canonical); err == nil && canonical.IsAbs() && canonical.Scheme == "https" {
			canonical.Path = strings.TrimSuffix(canonical.Path, "/") + r.URL.Path
			canonical.RawQuery = ""
			canonical.Fragment = ""
			m["CanonicalURL"] = canonical.String()
		}
	}
	if seo.NoIndex {
		m["Robots"] = "noindex,nofollow"
	}
	if _, exists := m["BaseURL"]; !exists {
		m["BaseURL"] = strings.TrimSuffix(RequestBaseURL(r), "/")
	}
	if _, exists := m["PublicBaseURL"]; !exists {
		m["PublicBaseURL"] = strings.TrimSuffix(s.publicBaseURL(r), "/")
	}
	if _, exists := m["APIBase"]; !exists {
		m["APIBase"] = strings.TrimSuffix(RequestBaseURL(r), "/") + s.APIBasePath()
	}
	if _, exists := m["APIBaseURL"]; !exists {
		m["APIBaseURL"] = strings.TrimSuffix(s.publicBaseURL(r), "/") + s.APIBasePath()
	}
	if _, exists := m["CanonicalURL"]; !exists {
		m["CanonicalURL"] = s.publicBaseURL(r) + r.URL.Path
	}
	if _, exists := m["Robots"]; !exists {
		m["Robots"] = s.robotsDirective(r)
	}
	if _, exists := m["ThemeClass"]; !exists {
		m["ThemeClass"] = themeClassFromRequest(r)
	}
	m["Theme"] = strings.TrimPrefix(themeClassFromRequest(r), "theme-")
	if _, exists := m["ConsentPreferences"]; !exists {
		state := consentStateFromContext(r)
		if state == nil {
			state = consentFromRequest(r, cfg)
		}
		m["ConsentPreferences"] = state.Preferences
	}
	if _, exists := m["ConsentAnalytics"]; !exists {
		state := consentStateFromContext(r)
		if state == nil {
			state = consentFromRequest(r, cfg)
		}
		m["ConsentAnalytics"] = state.Analytics
	}
	lang := i18n.FromContext(r.Context())
	privacy := cfg.Server.Privacy
	privacyText := localizedPrivacyDisplayText(privacy, lang)
	m["EssentialCookieDescription"] = privacyText.EssentialCookieDescription
	m["ConsentPreferencesDescription"] = privacyText.PreferencesCookieDescription
	m["ConsentAnalyticsDescription"] = privacyText.AnalyticsCookieDescription
	m["PreferencesCategoryEnabled"] = privacy.Cookies.Preferences.Enabled
	m["AnalyticsCategoryEnabled"] = privacy.Cookies.Analytics.Enabled
	m["ConsentShowPreferences"] = privacy.Consent.ShowPreferences
	m["ConsentPreferencesText"] = privacy.Consent.PreferencesText
	if privacy.Consent.PreferencesText == config.DefaultApplicationConfig().Server.Privacy.Consent.PreferencesText {
		m["ConsentPreferencesText"] = i18n.TranslateText(lang, "privacy.manage_preferences")
	}
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
		m["ConsentMessage"] = privacyText.ConsentMessage
		m["ConsentPolicyURL"] = privacy.Consent.Policy.URL
		m["ConsentPolicyText"] = privacy.Consent.Policy.Text
		m["ConsentDeclineText"] = privacy.Consent.Buttons.Decline
		m["ConsentAcceptText"] = privacy.Consent.Buttons.Accept
		defaults := config.DefaultApplicationConfig().Server.Privacy
		if privacy.Consent.Policy.Text == defaults.Consent.Policy.Text {
			m["ConsentPolicyText"] = i18n.TranslateText(lang, "privacy.title")
		}
		if privacy.Consent.Buttons.Decline == defaults.Consent.Buttons.Decline {
			m["ConsentDeclineText"] = i18n.TranslateText(lang, "consent.reject_all")
		}
		if privacy.Consent.Buttons.Accept == defaults.Consent.Buttons.Accept {
			m["ConsentAcceptText"] = i18n.TranslateText(lang, "consent.accept_all")
		}
		m["ConsentDataSold"] = privacy.Data.Sold
	}
	tracking := cfg.Server.Tracking
	m["TrackingEnabled"] = canLoadAnalytics(r, cfg) && !s.isTorRequest(r)
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

func (s *Server) brandingAssetURL(r *http.Request, value, fallback string) string {
	if strings.TrimSpace(value) == "" || s.brandingAssets == nil {
		return fallback
	}
	path, err := s.brandingAssets.Resolve(value)
	if err != nil {
		log.Printf("Warning: ignoring configured branding image: %v", err)
		return fallback
	}
	if strings.HasPrefix(path, "data:") {
		return path
	}
	if s.urlResolver != nil {
		resolved, err := url.Parse(s.urlResolver.BuildURL(r, path))
		if err == nil {
			return resolved.EscapedPath()
		}
	}
	return strings.TrimSuffix(RequestBaseURL(r), "/") + path
}

func (s *Server) brandingAssetVariantURL(r *http.Request, value, fallback string, width int) string {
	return s.brandingAssetSizeURL(r, value, fallback, width, 0)
}

func (s *Server) brandingAssetSizeURL(r *http.Request, value, fallback string, width, height int) string {
	assetURL := s.brandingAssetURL(r, value, fallback)
	if width <= 0 || !strings.Contains(assetURL, "/branding-assets/") || strings.HasPrefix(assetURL, "data:") {
		return assetURL
	}
	separator := "?"
	if strings.Contains(assetURL, "?") {
		separator = "&"
	}
	query := "width=" + strconv.Itoa(width)
	if height > 0 {
		query += "&height=" + strconv.Itoa(height)
	}
	return assetURL + separator + query
}

func (s *Server) brandingAssetSrcSet(r *http.Request, value, fallback string, widths ...int) string {
	assetURL := s.brandingAssetURL(r, value, fallback)
	if !strings.Contains(assetURL, "/branding-assets/") || strings.HasPrefix(assetURL, "data:") {
		return ""
	}
	var candidates []string
	for _, width := range widths {
		candidates = append(candidates, s.brandingAssetVariantURL(r, value, fallback, width)+" "+strconv.Itoa(width)+"w")
	}
	return strings.Join(candidates, ", ")
}

func (s *Server) brandingAssetAbsoluteSizeURL(r *http.Request, value, fallback string, width, height int) string {
	assetURL := s.brandingAssetURL(r, value, fallback)
	if strings.HasPrefix(assetURL, "data:") || !strings.Contains(assetURL, "/branding-assets/") {
		return assetURL
	}
	if s.urlResolver != nil {
		if path, err := s.brandingAssets.Resolve(value); err == nil && path != "" {
			assetURL = s.urlResolver.BuildURL(r, path)
		}
	} else {
		assetURL = strings.TrimSuffix(RequestBaseURL(r), "/") + assetURL
	}
	separator := "?"
	if strings.Contains(assetURL, "?") {
		separator = "&"
	}
	query := "width=" + strconv.Itoa(width)
	if height > 0 {
		query += "&height=" + strconv.Itoa(height)
	}
	return assetURL + separator + query
}

func seoMetaTagViews(tags map[string]string) []map[string]string {
	reserved := map[string]bool{"description": true, "keywords": true, "author": true, "robots": true, "viewport": true}
	names := make([]string, 0, len(tags))
	for name := range tags {
		names = append(names, name)
	}
	sort.Strings(names)
	views := make([]map[string]string, 0, min(len(names), 20))
	for _, name := range names {
		if len(views) == 20 {
			break
		}
		if !seoMetaNamePattern.MatchString(name) || reserved[name] || tags[name] == "" {
			continue
		}
		views = append(views, map[string]string{"Name": name, "Content": tags[name]})
	}
	return views
}

func seoVerificationTagViews(verification config.SEOVerificationConfig) []map[string]string {
	tags := []map[string]string{}
	providers := []struct {
		key, name, property, value string
	}{
		{"google", "google-site-verification", "", verification.Google},
		{"bing", "msvalidate.01", "", verification.Bing},
		{"yandex", "yandex-verification", "", verification.Yandex},
		{"baidu", "baidu-site-verification", "", verification.Baidu},
		{"pinterest", "p:domain_verify", "", verification.Pinterest},
		{"facebook", "", "fb:domain_verification", verification.Facebook},
	}
	for _, provider := range providers {
		if provider.value == "" || !seoVerificationValuePatterns[provider.key].MatchString(provider.value) {
			continue
		}
		tags = append(tags, map[string]string{"Name": provider.name, "Property": provider.property, "Content": provider.value})
	}
	for _, custom := range verification.Custom {
		attribute := custom.Name
		if attribute == "" {
			attribute = custom.Property
		}
		if attribute == "" || len(attribute) > 64 || len(custom.Content) == 0 || len(custom.Content) > 256 || !seoCustomTagNamePattern.MatchString(attribute) {
			continue
		}
		if custom.Name == "" && custom.Property == "" {
			continue
		}
		tags = append(tags, map[string]string{"Name": custom.Name, "Property": custom.Property, "Content": custom.Content})
	}
	return tags
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
	if m, ok := data.(map[string]interface{}); ok {
		if key := pageTitleTranslationKeys[page]; key != "" {
			m["Title"] = i18n.TranslateText(lang, key)
		}
	}
	clone, err := t.Clone()
	if err != nil {
		return err
	}
	clone = clone.Funcs(template.FuncMap{
		"healthCheckLabel": func(key string) string {
			translationKey := "health.check." + key
			translated := i18n.TranslateText(lang, translationKey)
			if translated != translationKey {
				return translated
			}
			if kind, id, ok := strings.Cut(key, "_"); ok {
				return strings.ToUpper(kind[:1]) + kind[1:] + ": " + strings.ReplaceAll(id, "_", " ")
			}
			return humanizePrivacyLabel(key)
		},
		"t": func(key string) string {
			return i18n.TranslateText(lang, key)
		},
		"tf": func(key string, args ...interface{}) string {
			return i18n.TranslateFormattedText(lang, key, args...)
		},
		"clientTranslations": func() string {
			keys := []string{
				"ui.api_request_failed", "ui.toast_dismiss", "ui.copy_json_success",
				"ui.coordinates_required", "ui.geolocation_unsupported", "ui.geolocation_loading",
				"ui.geolocation_detected", "ui.geolocation_failed", "ui.your_location", "ui.location",
				"ui.unknown", "ui.lookup_failed", "ui.no_airports_nearby", "ui.nearby_load_failed",
				"ui.request_failed", "ui.airport_label", "ui.close", "ui.search_no_results",
				"ui.view_nearby_airports", "common.copied", "geoip.your_location", "geoip.nearby_heading",
				"graphql.editor_welcome", "graphql.editor_run_hint", "airport.map_unavailable",
				"geoip.ip_label", "geoip.country_label", "geoip.city_label", "geoip.latitude_label",
				"geoip.longitude_label", "geoip.asn_label", "geoip.isp_label",
			}
			translations := make(map[string]string, len(keys))
			for _, key := range keys {
				translations[key] = i18n.TranslateText(lang, key)
			}
			encoded, err := json.Marshal(translations)
			if err != nil {
				return "{}"
			}
			return string(encoded)
		},
		"nextTheme": func(theme string) string {
			switch theme {
			case "dark":
				return "light"
			case "light":
				return "auto"
			default:
				return "dark"
			}
		},
		"themeLabel": func(theme string) string {
			if theme != "light" && theme != "dark" && theme != "auto" {
				theme = "auto"
			}
			return i18n.TranslateText(lang, "theme."+theme)
		},
		"path": func(path string) string {
			if s.urlResolver == nil {
				return path
			}
			resolved, err := url.Parse(s.urlResolver.BuildURL(r, path))
			if err != nil {
				return path
			}
			return resolved.EscapedPath()
		},
		"operatorHTML": func(name string) template.HTML {
			return s.operatorHTML(r, name)
		},
		"operatorContentSet": func(name string) bool {
			s.configMu.RLock()
			cfg := s.config
			s.configMu.RUnlock()
			if cfg == nil {
				return false
			}
			content, ok := cfg.Server.Branding.Content[name]
			return ok && operatorContentUsable(name, content)
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
			"ConsentDataSold", "CurrentYear", "OwnerActive", "Tokens", "PrimaryTokenID",
			"RevealID", "RevealToken", "TokenNotice", "ResourceLabelPlural":
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
	return robotsDirectiveForRules(r, nil, nil)
}

func (s *Server) robotsDirective(r *http.Request) string {
	if s == nil {
		return robotsDirective(r)
	}
	s.configMu.RLock()
	if s.config == nil {
		s.configMu.RUnlock()
		return robotsDirective(r)
	}
	allow := append([]string(nil), s.config.Web.Robots.Allow...)
	deny := append([]string(nil), s.config.Web.Robots.Deny...)
	s.configMu.RUnlock()
	return robotsDirectiveForRules(r, allow, deny)
}

func robotsDirectiveForRules(r *http.Request, allow, deny []string) string {
	if r == nil {
		return "noindex,nofollow"
	}
	path := strings.TrimSuffix(r.URL.Path, "/")
	if path == "" {
		path = "/"
	}
	for _, e := range publicSitemapEntries {
		if path == e.Path {
			if robotsPathAllowed(path, allow, deny) {
				return "index,follow"
			}
			return "noindex,nofollow"
		}
	}
	// Individual airport detail pages are the primary public content; the
	// search and nearby tools under the same prefix are already listed above.
	if ident := strings.TrimPrefix(path, "/airports/"); ident != path && ident != "" && !strings.Contains(ident, "/") && robotsPathAllowed(path, allow, deny) {
		return "index,follow"
	}
	return "noindex,nofollow"
}

func robotsPathAllowed(path string, allow, deny []string) bool {
	allowed := len(allow) == 0
	for _, pattern := range allow {
		if robotsPathMatches(path, pattern) {
			allowed = true
			break
		}
	}
	if !allowed {
		return false
	}
	for _, pattern := range deny {
		if robotsPathMatches(path, pattern) {
			return false
		}
	}
	return true
}

func robotsPathMatches(path, pattern string) bool {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		return false
	}
	if strings.HasSuffix(pattern, "$") {
		return path == strings.TrimSuffix(pattern, "$")
	}
	pattern = strings.TrimSuffix(pattern, "*")
	return strings.HasPrefix(path, pattern)
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
