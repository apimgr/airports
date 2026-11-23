package server

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/apimgr/airports/src/common/i18n"
)

const preferenceCookieAge = 365 * 24 * 60 * 60

var preferenceKeyPattern = regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`)
var preferenceValuePattern = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,64}$`)

func (s *Server) setPreferenceCookie(w http.ResponseWriter, r *http.Request, name, value string, maxAge int) {
	http.SetCookie(w, &http.Cookie{
		Name: name, Value: value, Path: "/", MaxAge: maxAge,
		SameSite: http.SameSiteLaxMode, Secure: strings.HasPrefix(s.publicBaseURL(r), "https://"),
	})
}

func (s *Server) preferenceRedirect(w http.ResponseWriter, r *http.Request, fallback string) {
	target := safePreferenceRedirect(r.FormValue("redirect"))
	if excludedPreferenceTarget(target) {
		target = ""
	}
	if target == "" {
		if referer, err := url.Parse(r.Referer()); err == nil && referer.IsAbs() && s.sameRequestOrigin(r, referer) {
			candidate := safePreferenceRedirect(referer.RequestURI())
			if !excludedPreferenceTarget(candidate) {
				target = candidate
			}
		}
	}
	if target == "" {
		target = fallback
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Location", target)
	w.WriteHeader(http.StatusSeeOther)
}

func excludedPreferenceTarget(target string) bool {
	parsed, err := url.Parse(target)
	if err != nil {
		return true
	}
	for _, route := range []string{"/server/preferences", "/server/preferences/sync", "/server/preferences/sync/import"} {
		if parsed.Path == route || strings.HasPrefix(parsed.Path, route+"/") {
			return true
		}
	}
	return false
}

func safePreferenceRedirect(raw string) string {
	if !strings.HasPrefix(raw, "/") || strings.HasPrefix(raw, "//") || strings.HasPrefix(raw, "/\\") || strings.ContainsAny(raw, "\r\n") {
		return ""
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.IsAbs() || parsed.Host != "" || parsed.Scheme != "" {
		return ""
	}
	return parsed.RequestURI()
}

func (s *Server) sameRequestOrigin(r *http.Request, parsed *url.URL) bool {
	base, err := url.Parse(strings.TrimSuffix(s.publicBaseURL(r), "/"))
	return err == nil && strings.EqualFold(base.Scheme, parsed.Scheme) && strings.EqualFold(base.Host, parsed.Host)
}

func (s *Server) handleServerPreferencesSave(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	if theme := strings.TrimSpace(r.FormValue("theme")); validThemes[theme] {
		s.setPreferenceCookie(w, r, themeCookieName, theme, preferenceCookieAge)
	}
	if lang := strings.TrimSpace(r.FormValue("lang")); lang != "" && lang != "auto" {
		if normalized := normalizePreferenceLanguage(lang); normalized != "" {
			s.setPreferenceCookie(w, r, i18n.LanguageCookieName, normalized, preferenceCookieAge)
		}
	} else if r.FormValue("lang") == "auto" {
		s.setPreferenceCookie(w, r, i18n.LanguageCookieName, "", -1)
	}
	if r.FormValue("consent_form") == "1" {
		state := ConsentState{Essential: true, Preferences: r.FormValue("consent_preferences") == "true", Analytics: r.FormValue("consent_analytics") == "true", Timestamp: time.Now().Unix()}
		if !s.config.Server.Privacy.Cookies.Preferences.Enabled {
			state.Preferences = false
		}
		if !s.config.Server.Privacy.Cookies.Analytics.Enabled {
			state.Analytics = false
		}
		previous := consentFromRequest(r, s.config)
		s.applyConsentWithdrawal(w, r, *previous, state, false)
		if encoded, err := json.Marshal(state); err == nil {
			s.setPreferenceCookie(w, r, consentCookieName, url.QueryEscape(string(encoded)), preferenceCookieAge)
		}
		if s.config.Server.Privacy.Data.Sold {
			value := r.FormValue("ccpa_opt_out") == "true"
			if value {
				s.setPreferenceCookie(w, r, ccpaOptOutCookieName, "true", preferenceCookieAge)
			} else {
				s.setPreferenceCookie(w, r, ccpaOptOutCookieName, "", -1)
			}
		}
	}
	if r.FormValue("action") == "reset" {
		for _, name := range []string{themeCookieName, i18n.LanguageCookieName, consentCookieName, ccpaOptOutCookieName, "airports_build", dismissedAnnouncementsCookieName} {
			s.setPreferenceCookie(w, r, name, "", -1)
		}
		for _, cookie := range r.Cookies() {
			if strings.HasPrefix(cookie.Name, "airports_pref_") {
				s.setPreferenceCookie(w, r, cookie.Name, "", -1)
			}
		}
		w.Header().Set("Clear-Site-Data", `"storage"`)
	}
	s.preferenceRedirect(w, r, "/")
}

func (s *Server) handleServerPreferencesSync(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		_ = r.ParseForm()
		values := parsePreferenceImport(r.URL.Query(), r.FormValue("code"))
		s.applyPreferenceValues(w, r, values)
		s.preferenceRedirect(w, r, "/server/preferences/sync")
		return
	}
	query := s.canonicalPreferences(r)
	base := strings.TrimSuffix(s.publicBaseURL(r), "/")
	fullURL := base + "/server/preferences/sync/import?" + query.Encode()
	code := base64.RawURLEncoding.EncodeToString([]byte(query.Encode()))
	if acceptsJSON(r) {
		data := s.preferenceResponse(r, query, nil, nil)
		data["url"] = fullURL
		data["code"] = code
		writePreferenceJSON(w, data)
		return
	}
	if strings.Contains(strings.ToLower(r.Header.Get("Accept")), "text/plain") {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = fmt.Fprintf(w, "URL: %s\nCode: %s\n", fullURL, code)
		return
	}
	s.renderTemplate(w, r, "server_preferences_sync", map[string]interface{}{
		"Title": "Sync preferences", "Theme": s.config.Web.UI.Theme,
		"ExportURL": fullURL, "ExportCode": code,
	})
}

func (s *Server) handleServerPreferencesImport(w http.ResponseWriter, r *http.Request) {
	values := parsePreferenceImport(r.URL.Query(), r.URL.Query().Get("code"))
	applied, ignored := s.applyPreferenceValues(w, r, values)
	if acceptsJSON(r) {
		writePreferenceJSON(w, map[string]interface{}{"applied": applied, "ignored": ignored})
		return
	}
	s.preferenceRedirect(w, r, "/")
}

func (s *Server) handleServerPreferencesAPI(w http.ResponseWriter, r *http.Request) {
	values := s.canonicalPreferences(r)
	writePreferenceJSON(w, s.preferenceResponse(r, values, nil, nil))
}

func (s *Server) handleServerPreferencesSaveAPI(w http.ResponseWriter, r *http.Request) {
	values := make(url.Values)
	if strings.Contains(strings.ToLower(r.Header.Get("Content-Type")), "application/json") {
		var payload map[string]string
		if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&payload); err != nil {
			writeJSONError(w, r, http.StatusBadRequest, "INVALID_REQUEST", "invalid JSON body")
			return
		}
		for key, value := range payload {
			values.Set(key, value)
		}
	} else if err := r.ParseForm(); err == nil {
		values = r.Form
	} else {
		writeJSONError(w, r, http.StatusBadRequest, "INVALID_REQUEST", "invalid form body")
		return
	}
	applied, ignored := s.applyPreferenceValues(w, r, values)
	if len(applied) == 0 {
		writeJSONError(w, r, http.StatusBadRequest, "VALIDATION_FAILED", "no supplied preference value was valid")
		return
	}
	writePreferenceJSON(w, preferenceResponseAfterApply(s, r, values, applied, ignored))
}

func (s *Server) handleServerPreferencesSyncAPI(w http.ResponseWriter, r *http.Request) {
	values := s.canonicalPreferences(r)
	base := strings.TrimSuffix(s.publicBaseURL(r), "/")
	query := values.Encode()
	data := s.preferenceResponse(r, values, nil, nil)
	data["url"] = base + "/server/preferences/sync/import?" + query
	data["code"] = base64.RawURLEncoding.EncodeToString([]byte(query))
	writePreferenceJSON(w, data)
}

func (s *Server) handleServerPreferencesImportAPI(w http.ResponseWriter, r *http.Request) {
	values := parsePreferenceImport(r.URL.Query(), r.URL.Query().Get("code"))
	applied, ignored := s.applyPreferenceValues(w, r, values)
	writePreferenceJSON(w, preferenceResponseAfterApply(s, r, values, applied, ignored))
}

func preferenceResponseAfterApply(s *Server, r *http.Request, submitted url.Values, applied, ignored []string) map[string]interface{} {
	data := s.preferenceResponse(r, nil, applied, ignored)
	prefs := data["prefs"].(map[string]string)
	for _, key := range applied {
		value := submitted.Get(key)
		switch key {
		case "theme":
			data["theme"] = value
		case "lang":
			if language := normalizePreferenceLanguage(value); language != "" {
				data["lang"] = language
			}
		default:
			if strings.HasPrefix(key, "airports_pref_") {
				prefs[strings.TrimPrefix(key, "airports_pref_")] = value
			}
		}
	}
	return data
}

func (s *Server) canonicalPreferences(r *http.Request) url.Values {
	values := url.Values{}
	theme := s.config.Web.UI.Theme
	if cookie, err := r.Cookie(themeCookieName); err == nil && validThemes[cookie.Value] {
		theme = cookie.Value
	}
	if theme == "" || !validThemes[theme] {
		theme = "auto"
	}
	lang := "en"
	if cookie, err := r.Cookie(i18n.LanguageCookieName); err == nil {
		if normalized := normalizePreferenceLanguage(cookie.Value); normalized != "" {
			lang = normalized
		}
	}
	values.Set("theme", theme)
	values.Set("lang", lang)
	for key, value := range preferenceCookieValues(r) {
		values.Set("airports_pref_"+key, value)
	}
	return values
}

func preferenceCookieValues(r *http.Request) map[string]string {
	prefs := make(map[string]string)
	for _, cookie := range r.Cookies() {
		if !strings.HasPrefix(cookie.Name, "airports_pref_") {
			continue
		}
		key := strings.TrimPrefix(cookie.Name, "airports_pref_")
		if preferenceKeyPattern.MatchString(key) && preferenceValuePattern.MatchString(cookie.Value) {
			prefs[key] = cookie.Value
		}
	}
	return prefs
}

func (s *Server) preferenceResponse(r *http.Request, canonical url.Values, applied, ignored []string) map[string]interface{} {
	if canonical == nil {
		canonical = s.canonicalPreferences(r)
	}
	prefs := make(map[string]string)
	for key, value := range preferenceCookieValues(r) {
		prefs[key] = value
	}
	data := map[string]interface{}{"theme": canonical.Get("theme"), "lang": canonical.Get("lang"), "prefs": prefs}
	if applied != nil {
		data["applied"] = applied
	}
	if ignored != nil {
		data["ignored"] = ignored
	}
	return data
}

func parsePreferenceImport(direct url.Values, code string) url.Values {
	values := url.Values{}
	code = strings.TrimSpace(code)
	if len(code) > 2048 {
		code = ""
	}
	if strings.Contains(code, "?") {
		code = code[strings.Index(code, "?")+1:]
	}
	if strings.TrimRight(code, "=") != "" && strings.Contains(strings.TrimRight(code, "="), "=") {
		if parsed, err := url.ParseQuery(code); err == nil {
			values = parsed
		}
	} else if code != "" {
		decoded, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(code, "="))
		if err != nil {
			decoded, err = base64.URLEncoding.DecodeString(code)
		}
		if err == nil {
			if parsed, parseErr := url.ParseQuery(string(decoded)); parseErr == nil {
				values = parsed
			}
		}
	}
	merged := make(url.Values)
	for key, vals := range values {
		merged[key] = append([]string(nil), vals...)
	}
	for key, vals := range direct {
		if key != "code" && len(vals) > 0 {
			merged[key] = append([]string(nil), vals...)
		}
	}
	return merged
}

func (s *Server) applyPreferenceValues(w http.ResponseWriter, r *http.Request, values url.Values) ([]string, []string) {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	if len(keys) > 32 {
		keys = keys[:32]
	}
	control := map[string]bool{"csrf_token": true, "code": true, "redirect": true, "action": true, "consent_form": true, "consent_preferences": true, "consent_analytics": true, "ccpa_opt_out": true}
	var applied, ignored []string
	for _, key := range keys {
		if control[key] {
			continue
		}
		vals := values[key]
		if len(vals) == 0 || len(vals[0]) > 64 {
			ignored = append(ignored, key)
			continue
		}
		value := vals[0]
		switch key {
		case "theme":
			if validThemes[value] {
				s.setPreferenceCookie(w, r, key, value, preferenceCookieAge)
				applied = append(applied, key)
			} else {
				ignored = append(ignored, key)
			}
		case "lang":
			if normalized := normalizePreferenceLanguage(value); normalized != "" {
				s.setPreferenceCookie(w, r, key, normalized, preferenceCookieAge)
				applied = append(applied, key)
			} else {
				ignored = append(ignored, key)
			}
		default:
			if strings.HasPrefix(key, "airports_pref_") {
				prefKey := strings.TrimPrefix(key, "airports_pref_")
				if preferenceKeyPattern.MatchString(prefKey) && preferenceValuePattern.MatchString(value) {
					s.setPreferenceCookie(w, r, key, value, preferenceCookieAge)
					applied = append(applied, key)
				} else {
					ignored = append(ignored, key)
				}
			} else {
				ignored = append(ignored, key)
			}
		}
	}
	return applied, ignored
}

func normalizePreferenceLanguage(value string) string {
	value = strings.TrimSpace(value)
	for _, supported := range i18n.AvailableLanguages() {
		if strings.EqualFold(value, supported.Code) {
			return supported.Code
		}
	}
	if base, _, ok := strings.Cut(value, "-"); ok {
		for _, supported := range i18n.AvailableLanguages() {
			if strings.EqualFold(base, supported.Code) {
				return supported.Code
			}
		}
	}
	return ""
}

func writePreferenceJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "data": data})
}

func acceptsJSON(r *http.Request) bool {
	return strings.Contains(strings.ToLower(r.Header.Get("Accept")), "application/json")
}
