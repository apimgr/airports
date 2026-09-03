package server

import (
	"encoding/json"
	"net/http"
	"net/url"
	"time"
)

// consentCookieName is the cookie used to persist the visitor's cookie
// consent choice per AI.md PART 16 "Cookie Consent Banner > Persistence".
const consentCookieName = "cookie_consent"

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
	target := "/"
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
// (used by a future "Manage Preferences" UI). Works with zero JavaScript.
func (s *Server) handleServerConsent(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		s.respondError(w, r, http.StatusBadRequest, "INVALID_REQUEST", "errors.form_parse_failed")
		return
	}

	choice := r.FormValue("choice")
	state := ConsentState{Essential: true, Timestamp: time.Now().Unix()}

	switch choice {
	case "accept":
		state.Preferences = true
		state.Analytics = true
	case "decline":
		state.Preferences = false
		state.Analytics = false
	case "save":
		state.Preferences = r.FormValue("preferences") == "on" || r.FormValue("preferences") == "true"
		state.Analytics = r.FormValue("analytics") == "on" || r.FormValue("analytics") == "true"
	default:
		s.respondError(w, r, http.StatusBadRequest, "INVALID_CHOICE", "errors.invalid_choice")
		return
	}

	writeConsentCookie(w, state)
	redirectBack(w, r)
}
