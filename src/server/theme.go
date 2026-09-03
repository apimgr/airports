package server

import "net/http"

// themeCookieName matches the cookie name the client-side progressive
// enhancement (applyTheme in static/js/app.js) already writes directly via
// document.cookie, so the server-rendered <noscript> form and the JS toggle
// stay consistent — both read back through themeClassFromRequest.
const themeCookieName = "theme"

// validThemes are the only values themeClassFromRequest recognizes.
var validThemes = map[string]bool{"light": true, "dark": true, "auto": true}

// handleSetTheme implements POST /theme, the no-JavaScript fallback for the
// theme toggle (AI.md PART 16 "No JavaScript-Disabled Broken State"). Sets
// the same "theme" cookie the JS-enhanced toggle writes client-side, then
// redirects back to the referring page so the next render picks it up via
// themeClassFromRequest with no flash of unstyled content.
func (s *Server) handleSetTheme(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		s.respondError(w, r, http.StatusBadRequest, "INVALID_REQUEST", "errors.form_parse_failed")
		return
	}

	theme := r.FormValue("theme")
	if !validThemes[theme] {
		s.respondError(w, r, http.StatusBadRequest, "INVALID_THEME", "errors.invalid_choice")
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     themeCookieName,
		Value:    theme,
		Path:     "/",
		MaxAge:   365 * 24 * 60 * 60,
		SameSite: http.SameSiteLaxMode,
	})
	redirectBack(w, r)
}
