package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/apimgr/airports/src/config"
)

func TestHandleServerConsentChoices(t *testing.T) {
	tests := []struct {
		name        string
		form        url.Values
		preferences bool
		analytics   bool
	}{
		{name: "accept enabled categories", form: url.Values{"choice": {"accept"}}, preferences: true, analytics: true},
		{name: "decline optional categories", form: url.Values{"choice": {"decline"}}},
		{name: "save granular choices", form: url.Values{"choice": {"save"}, "preferences": {"true"}, "analytics": {"false"}}, preferences: true},
		{name: "save no optional categories", form: url.Values{"choice": {"save"}, "preferences": {"false"}, "analytics": {"false"}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := config.DefaultApplicationConfig()
			s := &Server{config: cfg}
			r := httptest.NewRequest(http.MethodPost, "/server/consent", strings.NewReader(tt.form.Encode()))
			r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			r.Header.Set("Referer", "http://example.com/server/privacy")
			w := httptest.NewRecorder()

			s.handleServerConsent(w, r)

			if w.Code != http.StatusSeeOther {
				t.Fatalf("status = %d, want %d; body=%q", w.Code, http.StatusSeeOther, w.Body.String())
			}
			if got := w.Header().Get("Location"); got != "/server/privacy" {
				t.Errorf("redirect location = %q, want %q", got, "/server/privacy")
			}
			cookie := findCookie(t, w.Result().Cookies(), consentCookieName)
			if cookie.Path != "/" || cookie.MaxAge <= 0 || cookie.SameSite != http.SameSiteLaxMode || cookie.HttpOnly {
				t.Errorf("consent cookie attributes = %+v, want path=/, persistent, SameSite=Lax, readable by progressive enhancement", cookie)
			}
			r.AddCookie(cookie)
			state, ok := readConsentCookie(r)
			if !ok {
				t.Fatal("response consent cookie could not be parsed")
			}
			if !state.Essential || state.Preferences != tt.preferences || state.Analytics != tt.analytics || state.Timestamp <= 0 {
				t.Errorf("consent state = %+v, want essential=true preferences=%t analytics=%t and a timestamp", state, tt.preferences, tt.analytics)
			}
		})
	}
}

func TestConsentDefaultsOffAndUsesPathEscapedCookie(t *testing.T) {
	cfg := config.DefaultApplicationConfig()
	cfg.Server.Privacy.Cookies.Preferences.Enabled = true
	cfg.Server.Privacy.Cookies.Analytics.Enabled = true
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	state := consentFromRequest(request, cfg)
	if state.Preferences || state.Analytics {
		t.Fatalf("non-essential consent without a visitor choice = %+v, want both categories off", state)
	}
	w := httptest.NewRecorder()
	writeConsentCookie(w, ConsentState{Essential: true, Preferences: true, Timestamp: 1})
	cookie := findCookie(t, w.Result().Cookies(), consentCookieName)
	if !strings.HasPrefix(cookie.Value, "%7B") {
		t.Fatalf("cookie value = %q, want percent-encoded JSON starting with %%7B", cookie.Value)
	}
	decoded, err := url.PathUnescape(cookie.Value)
	if err != nil || !strings.HasPrefix(decoded, "{\"essential\"") {
		t.Fatalf("PathUnescape(%q) = %q, %v; want consent JSON", cookie.Value, decoded, err)
	}
}

func TestHandleServerConsentWritesAnonymizedAuditEvent(t *testing.T) {
	recorder := &ownerTokenAuditRecorder{}
	s := &Server{config: config.DefaultApplicationConfig(), auditLogger: recorder}
	r := httptest.NewRequest(http.MethodPost, "/server/consent", strings.NewReader("choice=decline"))
	r.RemoteAddr = "203.0.113.47:1234"
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	s.handleServerConsent(w, r)

	if len(recorder.events) != 1 {
		t.Fatalf("audit events = %d, want 1", len(recorder.events))
	}
	line := recorder.events[0]
	for _, want := range []string{"event=compliance.consent_updated", "actor_type=visitor", "source=web", "consent_type=cookie_consent", `ip="203.0.113.0"`, `\"analytics\":false`} {
		if !strings.Contains(line, want) {
			t.Errorf("audit event %q does not contain %q", line, want)
		}
	}
	if strings.Contains(line, "actor=") || !strings.Contains(line, `policy_version="1"`) {
		t.Errorf("anonymous event contains an owner identity or lacks the default policy version: %q", line)
	}
}

func TestHandleServerConsentUsesTorCircuitIdentity(t *testing.T) {
	recorder := &ownerTokenAuditRecorder{}
	s := &Server{config: config.DefaultApplicationConfig(), auditLogger: recorder}
	r := httptest.NewRequest(http.MethodPost, "/server/consent", strings.NewReader("choice=decline"))
	r.RemoteAddr = "127.0.0.1:1234"
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r = r.WithContext(context.WithValue(r.Context(), torCircuitContextKey{}, "fc00::1234"))
	w := httptest.NewRecorder()

	s.handleServerConsent(w, r)

	if len(recorder.events) != 1 {
		t.Fatalf("audit events = %d, want 1", len(recorder.events))
	}
	line := recorder.events[0]
	if !strings.Contains(line, `ip="tor:fc00::1234"`) || strings.Contains(line, "127.0.0.1") {
		t.Errorf("Tor consent event must use the circuit identity, got %q", line)
	}
}

func TestAnonymizeConsentIP(t *testing.T) {
	tests := map[string]string{
		"203.0.113.47":                           "203.0.113.0",
		"2001:db8:1234:5678:9abc:def0:1234:5678": "2001:db8:1234::",
		"not-an-ip":                              "",
	}
	for input, want := range tests {
		if got := anonymizeConsentIP(input); got != want {
			t.Errorf("anonymizeConsentIP(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestHandleServerConsentAcceptHonorsDisabledCategories(t *testing.T) {
	cfg := config.DefaultApplicationConfig()
	cfg.Server.Privacy.Cookies.Preferences.Enabled = false
	cfg.Server.Privacy.Cookies.Analytics.Enabled = false
	s := &Server{config: cfg}
	r := httptest.NewRequest(http.MethodPost, "/server/consent", strings.NewReader("choice=accept"))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	s.handleServerConsent(w, r)
	r.AddCookie(findCookie(t, w.Result().Cookies(), consentCookieName))
	state, ok := readConsentCookie(r)
	if !ok {
		t.Fatal("response consent cookie could not be parsed")
	}
	if state.Preferences || state.Analytics {
		t.Errorf("disabled categories were granted: %+v", state)
	}
}

func TestConsentMiddlewareGatesPreferencesAndAnalytics(t *testing.T) {
	tests := []struct {
		name               string
		consent            ConsentState
		preferencesEnabled bool
		analyticsEnabled   bool
		trackingType       string
		optOut             bool
		wantPreferences    bool
		wantAnalytics      bool
	}{
		{name: "explicit consent with provider", consent: ConsentState{Preferences: true, Analytics: true}, preferencesEnabled: true, analyticsEnabled: true, trackingType: "simple", wantPreferences: true, wantAnalytics: true},
		{name: "declined consent", consent: ConsentState{}, preferencesEnabled: true, analyticsEnabled: true, trackingType: "simple"},
		{name: "preference category disabled", consent: ConsentState{Preferences: true, Analytics: true}, preferencesEnabled: false, analyticsEnabled: true, trackingType: "simple", wantAnalytics: true},
		{name: "analytics category disabled", consent: ConsentState{Preferences: true, Analytics: true}, preferencesEnabled: true, analyticsEnabled: false, trackingType: "simple", wantPreferences: true},
		{name: "no tracking provider", consent: ConsentState{Preferences: true, Analytics: true}, preferencesEnabled: true, analyticsEnabled: true, wantPreferences: true},
		{name: "ccpa opt out suppresses analytics", consent: ConsentState{Preferences: true, Analytics: true}, preferencesEnabled: true, analyticsEnabled: true, trackingType: "simple", optOut: true, wantPreferences: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := config.DefaultApplicationConfig()
			cfg.Server.Privacy.Cookies.Preferences.Enabled = tt.preferencesEnabled
			cfg.Server.Privacy.Cookies.Analytics.Enabled = tt.analyticsEnabled
			cfg.Server.Tracking.Type = tt.trackingType
			cookieValue, err := json.Marshal(tt.consent)
			if err != nil {
				t.Fatal(err)
			}
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			r.AddCookie(&http.Cookie{Name: consentCookieName, Value: url.PathEscape(string(cookieValue))})
			if tt.optOut {
				r.AddCookie(&http.Cookie{Name: ccpaOptOutCookieName, Value: "true"})
			}
			var gotPreferences, gotAnalytics bool
			next := http.HandlerFunc(func(_ http.ResponseWriter, request *http.Request) {
				gotPreferences = CanSetPreferenceCookie(request)
				gotAnalytics = canLoadAnalytics(request, cfg)
			})
			consentMiddlewareWithConfig(next, func() *config.ApplicationConfig { return cfg }).ServeHTTP(httptest.NewRecorder(), r)

			if gotPreferences != tt.wantPreferences {
				t.Errorf("CanSetPreferenceCookie() = %t, want %t", gotPreferences, tt.wantPreferences)
			}
			if gotAnalytics != tt.wantAnalytics {
				t.Errorf("canLoadAnalytics() = %t, want %t", gotAnalytics, tt.wantAnalytics)
			}
		})
	}
}

func TestHandleServerConsentWithdrawalClearsCategoryCookies(t *testing.T) {
	cfg := config.DefaultApplicationConfig()
	cfg.Server.Privacy.Cookies.Preferences.Enabled = true
	cfg.Server.Privacy.Cookies.Analytics.Enabled = true
	s := &Server{config: cfg}
	r := httptest.NewRequest(http.MethodPost, "/server/consent", strings.NewReader("choice=decline"))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.AddCookie(&http.Cookie{Name: consentCookieName, Value: url.PathEscape(`{"essential":true,"preferences":true,"analytics":true,"timestamp":1}`)})
	r.AddCookie(&http.Cookie{Name: "theme", Value: "dark"})
	r.AddCookie(&http.Cookie{Name: "lang", Value: "fr"})
	r.AddCookie(&http.Cookie{Name: "airports_pref_map", Value: "satellite"})
	r.AddCookie(&http.Cookie{Name: "airports_owner_token_XXXXXX", Value: "credential"})
	w := httptest.NewRecorder()

	s.handleServerConsent(w, r)

	if got := w.Header().Get("Clear-Site-Data"); got != `"cache", "storage"` {
		t.Fatalf("Clear-Site-Data = %q, want cache and storage only", got)
	}
	for _, name := range []string{"theme", "lang", "airports_pref_map"} {
		cookie := findCookie(t, w.Result().Cookies(), name)
		if cookie.MaxAge != -1 {
			t.Errorf("%s MaxAge = %d, want expired cookie", name, cookie.MaxAge)
		}
	}
	for _, cookie := range w.Result().Cookies() {
		if cookie.Name == "airports_owner_token_XXXXXX" {
			t.Fatal("consent withdrawal cleared the owner credential cookie")
		}
	}
}

func TestHandleServerCCPAOptOutWithdrawsNonessentialConsent(t *testing.T) {
	cfg := config.DefaultApplicationConfig()
	cfg.Server.Privacy.Cookies.Preferences.Enabled = true
	cfg.Server.Privacy.Cookies.Analytics.Enabled = true
	s := &Server{config: cfg}
	r := httptest.NewRequest(http.MethodPost, "/server/ccpa", strings.NewReader("choice=opt-out"))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.AddCookie(&http.Cookie{Name: consentCookieName, Value: url.PathEscape(`{"essential":true,"preferences":true,"analytics":true,"timestamp":1}`)})
	r.AddCookie(&http.Cookie{Name: "theme", Value: "dark"})
	w := httptest.NewRecorder()

	s.handleServerCCPA(w, r)

	if got := w.Header().Get("Clear-Site-Data"); got != `"cache", "storage"` {
		t.Fatalf("Clear-Site-Data = %q, want cache and storage only", got)
	}
	if cookie := findCookie(t, w.Result().Cookies(), ccpaOptOutCookieName); cookie.Value != "true" {
		t.Fatalf("CCPA opt-out cookie = %+v", cookie)
	}
	if cookie := findCookie(t, w.Result().Cookies(), consentCookieName); cookie.Value == "" {
		t.Fatal("CCPA opt-out did not preserve the consent choice")
	}
}

func TestCanLoadAnalyticsSuppressesTorTracking(t *testing.T) {
	cfg := config.DefaultApplicationConfig()
	cfg.Server.Tracking.Type = "simple"
	cfg.Server.Tor.OnionAddress = "exampleexampleexampleexampleexampleexampleexampleexample.onion"
	s := &Server{config: cfg}
	consentJSON := `{"essential":true,"preferences":true,"analytics":true,"timestamp":1}`
	consentCookie := &http.Cookie{Name: consentCookieName, Value: url.PathEscape(consentJSON)}
	r := httptest.NewRequest(http.MethodGet, "http://exampleexampleexampleexampleexampleexampleexampleexample.onion/", nil)
	r.AddCookie(consentCookie)
	r = r.WithContext(context.WithValue(r.Context(), torCircuitContextKey{}, "fc00::1"))

	if scripts, connects := s.trackingCSPSources(r); scripts != "" || connects != "" {
		t.Errorf("Tor CSP tracking origins = (%q, %q), want both empty", scripts, connects)
	}
	clearnet := httptest.NewRequest(http.MethodGet, "http://airports.example/", nil)
	clearnet.AddCookie(consentCookie)
	scripts, connects := s.trackingCSPSources(clearnet)
	if scripts == "" || connects == "" {
		t.Errorf("clearnet CSP tracking origins = (%q, %q), want configured provider origins", scripts, connects)
	}
	if !canLoadAnalytics(r, cfg) {
		t.Fatal("analytics consent should remain valid; Tor suppression belongs to the request-specific loading gate")
	}
}

func TestInvalidConsentCookieFallsBackToConfiguredDefaults(t *testing.T) {
	cfg := config.DefaultApplicationConfig()
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.AddCookie(&http.Cookie{Name: consentCookieName, Value: "%ZZ"})

	if hasValidConsentCookie(request) {
		t.Fatal("malformed consent cookie should not count as valid consent")
	}
	state := consentFromRequest(request, cfg)
	if !state.Essential || state.Preferences || state.Analytics {
		t.Errorf("fallback consent state = %+v, want essential cookies only until opt-in", state)
	}
}

func findCookie(t *testing.T, cookies []*http.Cookie, name string) *http.Cookie {
	t.Helper()
	for _, cookie := range cookies {
		if cookie.Name == name {
			return cookie
		}
	}
	t.Fatalf("response is missing %q cookie", name)
	return nil
}
