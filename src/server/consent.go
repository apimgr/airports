package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"time"

	"github.com/apimgr/airports/src/config"
)

// consentCookieName is the cookie used to persist the visitor's cookie
// consent choice per AI.md PART 16 "Cookie Consent Banner > Persistence".
const consentCookieName = "cookie_consent"

type consentContextKey struct{}

// ConsentState is the JSON value stored in the cookie_consent cookie, per
// AI.md PART 16 "Consent Storage":
//
//	cookie_consent = {"essential":true,"preferences":true,"analytics":false,"timestamp":1704067200}
type ConsentState struct {
	Essential   bool  `json:"essential"`
	Preferences bool  `json:"preferences"`
	Analytics   bool  `json:"analytics"`
	Timestamp   int64 `json:"timestamp"`
}

// readConsentCookie parses the cookie_consent cookie, if present and valid.
func readConsentCookie(r *http.Request) (*ConsentState, bool) {
	c, err := r.Cookie(consentCookieName)
	if err != nil || c.Value == "" {
		return nil, false
	}
	raw, err := url.QueryUnescape(c.Value)
	if err != nil {
		return nil, false
	}
	var state ConsentState
	if err := json.Unmarshal([]byte(raw), &state); err != nil {
		return nil, false
	}
	return &state, true
}

// hasValidConsentCookie reports whether the request already carries a
// parseable cookie_consent cookie. When true, the server must not render
// the consent banner again per AI.md PART 16 "Banner Behavior > Already set".
func hasValidConsentCookie(r *http.Request) bool {
	_, ok := readConsentCookie(r)
	return ok
}

func consentFromRequest(r *http.Request, cfg *config.Config) *ConsentState {
	if state, ok := readConsentCookie(r); ok {
		state.Essential = true
		if cfg != nil {
			state.Preferences = state.Preferences && cfg.Server.Privacy.Cookies.Preferences.Enabled
			state.Analytics = state.Analytics && cfg.Server.Privacy.Cookies.Analytics.Enabled
		}
		return state
	}
	enabled := cfg != nil && cfg.Server.Privacy.Consent.DefaultEnabled
	preferences := enabled && cfg.Server.Privacy.Cookies.Preferences.Enabled
	analytics := enabled && cfg.Server.Privacy.Cookies.Analytics.Enabled
	return &ConsentState{Essential: true, Preferences: preferences, Analytics: analytics}
}

// ConsentMiddleware puts the effective consent state in the request context.
func ConsentMiddleware(next http.Handler) http.Handler {
	return consentMiddlewareWithConfig(next, config.Get)
}

func (s *Server) consentMiddleware(next http.Handler) http.Handler {
	return consentMiddlewareWithConfig(next, func() *config.Config {
		s.configMu.RLock()
		defer s.configMu.RUnlock()
		return s.config
	})
}

func consentMiddlewareWithConfig(next http.Handler, currentConfig func() *config.Config) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		state := consentFromRequest(r, currentConfig())
		ctx := context.WithValue(r.Context(), consentContextKey{}, state)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// CanSetPreferenceCookie reports whether the visitor allows preference cookies.
func CanSetPreferenceCookie(r *http.Request) bool {
	state := consentStateFromContext(r)
	return state != nil && state.Preferences
}

// CanLoadAnalytics reports whether analytics consent is enabled and a provider
// is configured for this request.
func CanLoadAnalytics(r *http.Request) bool {
	return canLoadAnalytics(r, config.Get())
}

func canLoadAnalytics(r *http.Request, cfg *config.Config) bool {
	if cfg == nil || r == nil || !cfg.Server.Privacy.Cookies.Analytics.Enabled || cfg.Server.Tracking.Type == "" || cfg.Server.Tracking.Type == "none" {
		return false
	}
	if cookie, err := r.Cookie("ccpa_opt_out"); err == nil && cookie.Value == "true" {
		return false
	}
	state := consentStateFromContext(r)
	if state == nil {
		state = consentFromRequest(r, cfg)
	}
	return state.Analytics
}

const ccpaOptOutCookieName = "ccpa_opt_out"

func ccpaOptOutFromRequest(r *http.Request) bool {
	cookie, err := r.Cookie(ccpaOptOutCookieName)
	return err == nil && cookie.Value == "true"
}

// handleServerCCPA records or clears the visitor's Do Not Sell choice.
func (s *Server) handleServerCCPA(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		s.respondError(w, r, http.StatusBadRequest, "INVALID_REQUEST", "errors.form_parse_failed")
		return
	}
	switch r.FormValue("choice") {
	case "opt-out":
		http.SetCookie(w, &http.Cookie{Name: ccpaOptOutCookieName, Value: "true", Path: "/", MaxAge: 365 * 24 * 60 * 60})
		writeConsentCookie(w, ConsentState{Essential: true, Timestamp: time.Now().Unix()})
	case "opt-in":
		http.SetCookie(w, &http.Cookie{Name: ccpaOptOutCookieName, Value: "", Path: "/", MaxAge: -1})
	default:
		s.respondError(w, r, http.StatusBadRequest, "INVALID_CHOICE", "errors.invalid_choice")
		return
	}
	redirectBack(w, r)
}

func consentStateFromContext(r *http.Request) *ConsentState {
	if r == nil {
		return nil
	}
	state, _ := r.Context().Value(consentContextKey{}).(*ConsentState)
	return state
}

// writeConsentCookie sets cookie_consent as a 1-year, SameSite=Lax cookie
// (readable by the client-side progressive-enhancement module in app.js,
// so not HttpOnly) per AI.md PART 16 "Implementation > Progressive enhancement".
func writeConsentCookie(w http.ResponseWriter, state ConsentState) {
	body, _ := json.Marshal(state)
	http.SetCookie(w, &http.Cookie{
		Name:     consentCookieName,
		Value:    url.QueryEscape(string(body)),
		Path:     "/",
		MaxAge:   365 * 24 * 60 * 60,
		SameSite: http.SameSiteLaxMode,
	})
}

// redirectBack sends the visitor back to the page that submitted the
// consent form (Referer), falling back to "/" when absent or off-site.
func redirectBack(w http.ResponseWriter, r *http.Request) {
	target := RequestBaseURL(r)
	if ref := r.Header.Get("Referer"); ref != "" {
		if u, err := url.Parse(ref); err == nil && u.Host == r.Host {
			target = u.RequestURI()
		}
	}
	http.Redirect(w, r, target, http.StatusSeeOther)
}

// handleServerConsent implements POST /server/consent per AI.md PART 16
// "Banner Behavior": choice=accept grants all categories, choice=decline
// grants essential only, choice=save reads the granular preferences form
// (available on the banner and privacy page). Works with zero JavaScript.
func (s *Server) handleServerConsent(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		s.respondError(w, r, http.StatusBadRequest, "INVALID_REQUEST", "errors.form_parse_failed")
		return
	}

	choice := r.FormValue("choice")
	state := ConsentState{Essential: true, Timestamp: time.Now().Unix()}

	switch choice {
	case "accept":
		state.Preferences = s.config.Server.Privacy.Cookies.Preferences.Enabled
		state.Analytics = s.config.Server.Privacy.Cookies.Analytics.Enabled
	case "decline":
		state.Preferences = false
		state.Analytics = false
	case "save":
		preferences, err := config.ParseBool(r.FormValue("preferences"), false)
		if err != nil {
			s.respondError(w, r, http.StatusBadRequest, "VALIDATION_FAILED", "errors.invalid_choice")
			return
		}
		analytics, err := config.ParseBool(r.FormValue("analytics"), false)
		if err != nil {
			s.respondError(w, r, http.StatusBadRequest, "VALIDATION_FAILED", "errors.invalid_choice")
			return
		}
		state.Preferences = preferences && s.config.Server.Privacy.Cookies.Preferences.Enabled
		state.Analytics = analytics && s.config.Server.Privacy.Cookies.Analytics.Enabled
	default:
		s.respondError(w, r, http.StatusBadRequest, "INVALID_CHOICE", "errors.invalid_choice")
		return
	}

	writeConsentCookie(w, state)
	redirectBack(w, r)
}
