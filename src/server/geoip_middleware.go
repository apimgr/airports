package server

import (
	"encoding/json"
	"log"
	"net"
	"net/http"
	"strings"

	"github.com/apimgr/airports/src/common/i18n"
	"github.com/apimgr/airports/src/geoip"
)

// respondGeoBlocked sends the 403 response for a country-blocked IP,
// following the same content-negotiation and no-detail-leak pattern as
// respondBlocked. The blocked country is logged internally but never
// exposed to the client, per AI.md PART 9 backend Tier 1 rules.
func respondGeoBlocked(w http.ResponseWriter, r *http.Request) {
	accept := r.Header.Get("Accept")
	if strings.Contains(accept, "text/html") {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(blockedPageHTML))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusForbidden)
	lang := i18n.FromContext(r.Context())
	resp := ErrorResponse{
		OK:      false,
		Error:   "IP_BLOCKED",
		Message: i18n.T(lang, "errors.forbidden"),
		Details: map[string]interface{}{},
	}
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		log.Printf("respondGeoBlocked: encode failed: %v", err)
	}
}

// countryAllowed evaluates deny_countries/allow_countries per AI.md PART 19
// "Country Blocking Behavior": both empty means no blocking; allow_countries
// (if non-empty) wins over deny_countries when both are set.
func countryAllowed(country string, allowCountries, denyCountries []string) bool {
	if len(allowCountries) > 0 {
		for _, c := range allowCountries {
			if strings.EqualFold(c, country) {
				return true
			}
		}
		return false
	}
	for _, c := range denyCountries {
		if strings.EqualFold(c, country) {
			return false
		}
	}
	return true
}

// GeoIPMiddleware rejects requests from denied/non-allowed countries with
// 403, per AI.md PART 19 "GeoIP". GeoIP is a risk signal only, never the
// sole access gate — it runs after allowlist/blocklist/rate-limit, never
// replacing them, and fails open (never blocks) whenever:
//   - geoip.enabled is false
//   - the request is already allowlisted (server.security.allowlist)
//   - the client IP is a private/loopback/link-local address (RFC 1918/4193)
//   - the GeoIP service or its country database is unavailable
//   - the country lookup itself errors
//
// Only a definitive "this country is not permitted" result blocks.
func GeoIPMiddleware(svc *geoip.Service, enabled bool, allowCountries, denyCountries []string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !enabled || (len(allowCountries) == 0 && len(denyCountries) == 0) {
				next.ServeHTTP(w, r)
				return
			}
			if IsAllowlisted(r.Context()) {
				next.ServeHTTP(w, r)
				return
			}
			if !svc.Available() {
				next.ServeHTTP(w, r)
				return
			}

			ipStr := getClientIP(r)
			ip := net.ParseIP(ipStr)
			if ip == nil || ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() {
				next.ServeHTTP(w, r)
				return
			}

			location, err := svc.Lookup(ip)
			if err != nil || location == nil || location.Country == "" {
				// Fail open: missing/stale database or lookup error never
				// blocks a request per AI.md PART 19 "Failure mode".
				next.ServeHTTP(w, r)
				return
			}

			if !countryAllowed(location.Country, allowCountries, denyCountries) {
				log.Printf("security.geoip_blocked: rejected request ip=%s country=%s path=%s",
					ipStr, location.Country, r.URL.Path)
				respondGeoBlocked(w, r)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
