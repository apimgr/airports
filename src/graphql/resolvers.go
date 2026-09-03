package graphql

import (
	"encoding/json"
	"net"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/apimgr/airports/src/common/i18n"
)

// graphQLArgPattern builds a regex that matches `field(... name: VALUE ...)`.
// The argument value is captured in group 1. This is a minimalist parser —
// it intentionally does not aspire to be a real GraphQL grammar.
func graphQLArgPattern(field, arg, valueRE string) *regexp.Regexp {
	return regexp.MustCompile(
		`\b` + regexp.QuoteMeta(field) + `\s*\([^)]*\b` + regexp.QuoteMeta(arg) + `\s*:\s*` + valueRE + `[^)]*\)`,
	)
}

// extractStringArg parses a quoted string argument out of
// `field(arg: "value", ...)`. Returns "" when not found.
func extractStringArg(query, field, arg string) string {
	m := graphQLArgPattern(field, arg, `"([^"]*)"`).FindStringSubmatch(query)
	if len(m) < 2 {
		return ""
	}
	return m[1]
}

// extractFloatArg parses a numeric argument out of
// `field(arg: 12.34, ...)`. Returns (0, false) when not found.
func extractFloatArg(query, field, arg string) (float64, bool) {
	m := graphQLArgPattern(field, arg, `(-?\d+(?:\.\d+)?)`).FindStringSubmatch(query)
	if len(m) < 2 {
		return 0, false
	}
	v, err := strconv.ParseFloat(m[1], 64)
	if err != nil {
		return 0, false
	}
	return v, true
}

// extractIntArg parses an integer argument out of `field(arg: 12, ...)`.
// Returns (0, false) when not found. Reuses extractFloatArg's number
// grammar since GraphQL Int literals are a subset of Float literals.
func extractIntArg(query, field, arg string) (int, bool) {
	v, ok := extractFloatArg(query, field, arg)
	if !ok {
		return 0, false
	}
	return int(v), true
}

// fieldPresent reports whether the query contains a call to the given
// field name, either with arguments (`field(...)`) or a bare selection
// set (`field { ... }`). Used for no-arg or optional-arg fields where
// extractStringArg/extractFloatArg can't anchor on a required argument.
func fieldPresent(query, field string) bool {
	re := regexp.MustCompile(`\b` + regexp.QuoteMeta(field) + `\b\s*[({]`)
	return re.MatchString(query)
}

// nameCount pairs a name (country or state) with its airport count —
// the shape returned by the `countries` and `states` GraphQL queries.
type nameCount struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

// sortedNameCounts converts a name->count map into a slice sorted by
// name, giving deterministic output for the `countries`/`states` queries.
func sortedNameCounts(counts map[string]int) []nameCount {
	names := make([]string, 0, len(counts))
	for name := range counts {
		names = append(names, name)
	}
	sort.Strings(names)

	results := make([]nameCount, 0, len(names))
	for _, name := range names {
		results = append(results, nameCount{Name: name, Count: counts[name]})
	}
	return results
}

// serveQuery is the resolver entry point invoked by QueryHandler.
func serveQuery(w http.ResponseWriter, r *http.Request, src AirportSource, geo GeoIPSource) {
	// Cap request body to a sane upper bound — protects against an
	// attacker streaming an unbounded "query" body to exhaust memory.
	r.Body = http.MaxBytesReader(w, r.Body, 64*1024)

	lang := i18n.FromContext(r.Context())

	var req request
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, response{
			Errors: []errorMessage{{Message: i18n.Tf(lang, "graphql.err_invalid_body", "error", err.Error())}},
		})
		return
	}

	q := strings.TrimSpace(req.Query)
	if q == "" {
		writeJSON(w, http.StatusBadRequest, response{
			Errors: []errorMessage{{Message: i18n.T(lang, "graphql.err_query_required")}},
		})
		return
	}

	// `airport(code: "XXXX") { ... }` — extract the code argument.
	if code := extractStringArg(q, "airport", "code"); code != "" {
		apt, err := src.GetByCode(code)
		if err != nil {
			writeJSON(w, http.StatusOK, response{
				Errors: []errorMessage{{Message: i18n.Tf(lang, "graphql.err_airport_not_found", "code", code)}},
			})
			return
		}
		writeJSON(w, http.StatusOK, response{
			Data: map[string]interface{}{"airport": apt},
		})
		return
	}

	// `nearby(lat: N, lon: N, radius: N) { ... }` — extract numeric args.
	if lat, latOK := extractFloatArg(q, "nearby", "lat"); latOK {
		lon, lonOK := extractFloatArg(q, "nearby", "lon")
		if !lonOK {
			writeJSON(w, http.StatusBadRequest, response{
				Errors: []errorMessage{{Message: i18n.T(lang, "graphql.err_nearby_requires_lon")}},
			})
			return
		}
		radius, ok := extractFloatArg(q, "nearby", "radius")
		if !ok {
			radius = 50
		}
		// Bounds-check the caller-supplied radius: keep it positive and
		// capped so a single query can't scan the whole dataset.
		if !(radius > 0) {
			radius = 50
		}
		if radius > 500 {
			radius = 500
		}
		results := src.GetNearbyWithDistance(lat, lon, radius, 20, "metric")
		writeJSON(w, http.StatusOK, response{
			Data: map[string]interface{}{"nearby": results},
		})
		return
	}

	// `airports(limit: N, page: N)` — paginated list, mirrors
	// GET /api/{api_version}/airports.
	if fieldPresent(q, "airports") {
		limit, ok := extractIntArg(q, "airports", "limit")
		if !ok || limit <= 0 || limit > 1000 {
			limit = 250
		}
		page, ok := extractIntArg(q, "airports", "page")
		if !ok || page < 1 {
			page = 1
		}
		// Guard against integer overflow: a very large page makes
		// (page-1)*limit overflow to a negative int, which would panic the
		// slice inside GetAll. Compute the offset in 64-bit space and, when it
		// lands outside the valid int range, return an empty list instead.
		offset := int64(page-1) * int64(limit)
		var results interface{} = []interface{}{}
		if offset >= 0 && offset <= int64(int(^uint(0)>>1)) {
			results = src.GetAll(limit, int(offset))
		}
		writeJSON(w, http.StatusOK, response{
			Data: map[string]interface{}{"airports": results},
		})
		return
	}

	// `search(q: "...", limit: N, page: N)` — mirrors
	// GET /api/{api_version}/airports/search.
	if term := extractStringArg(q, "search", "q"); term != "" {
		limit, ok := extractIntArg(q, "search", "limit")
		if !ok || limit <= 0 || limit > 1000 {
			limit = 50
		}
		page, ok := extractIntArg(q, "search", "page")
		if !ok || page < 1 {
			page = 1
		}
		// Guard against integer overflow: a very large page makes
		// (page-1)*limit overflow to a negative int, which would panic the
		// slice inside Search. Compute the offset in 64-bit space and, when it
		// lands outside the valid int range, return an empty list instead.
		offset := int64(page-1) * int64(limit)
		var results interface{} = []interface{}{}
		if offset >= 0 && offset <= int64(int(^uint(0)>>1)) {
			results = src.Search(term, limit, int(offset))
		}
		writeJSON(w, http.StatusOK, response{
			Data: map[string]interface{}{"search": results},
		})
		return
	}

	// `within(latMin:, latMax:, lonMin:, lonMax:)` — bounding-box lookup,
	// mirrors GET /api/{api_version}/airports/within.
	if latMin, ok := extractFloatArg(q, "within", "latMin"); ok {
		latMax, latMaxOK := extractFloatArg(q, "within", "latMax")
		lonMin, lonMinOK := extractFloatArg(q, "within", "lonMin")
		lonMax, lonMaxOK := extractFloatArg(q, "within", "lonMax")
		if !latMaxOK || !lonMinOK || !lonMaxOK {
			writeJSON(w, http.StatusBadRequest, response{
				Errors: []errorMessage{{Message: i18n.T(lang, "graphql.err_within_requires_bounds")}},
			})
			return
		}
		results := src.GetInBoundingBox(latMin, latMax, lonMin, lonMax)
		writeJSON(w, http.StatusOK, response{
			Data: map[string]interface{}{"within": results},
		})
		return
	}

	// `autocomplete(q: "...", limit: N)` — mirrors
	// GET /api/{api_version}/airports/autocomplete.
	if term := extractStringArg(q, "autocomplete", "q"); term != "" {
		if len(term) < 2 {
			writeJSON(w, http.StatusBadRequest, response{
				Errors: []errorMessage{{Message: i18n.T(lang, "graphql.err_query_too_short")}},
			})
			return
		}
		limit, ok := extractIntArg(q, "autocomplete", "limit")
		if !ok || limit <= 0 || limit > 50 {
			limit = 10
		}
		results := src.Search(term, limit, 0)
		writeJSON(w, http.StatusOK, response{
			Data: map[string]interface{}{"autocomplete": results},
		})
		return
	}

	// `countries` — mirrors GET /api/{api_version}/countries.
	if fieldPresent(q, "countries") {
		results := sortedNameCounts(src.GetCountries())
		writeJSON(w, http.StatusOK, response{
			Data: map[string]interface{}{"countries": results},
		})
		return
	}

	// `states(country: "...")` — mirrors GET /api/{api_version}/states/{country}.
	if country := extractStringArg(q, "states", "country"); country != "" {
		results := sortedNameCounts(src.GetStatesInCountry(country))
		writeJSON(w, http.StatusOK, response{
			Data: map[string]interface{}{"states": results},
		})
		return
	}

	// `stats` — mirrors GET /api/{api_version}/stats.
	if fieldPresent(q, "stats") {
		raw := src.Stats()
		stats := map[string]interface{}{
			"totalAirports": raw["total_airports"],
			"countries":     raw["countries"],
			"cities":        raw["cities"],
			"withIata":      raw["with_iata"],
		}
		writeJSON(w, http.StatusOK, response{
			Data: map[string]interface{}{"stats": stats},
		})
		return
	}

	// `geoip(ip: "...")` — mirrors GET /api/{api_version}/geoip and
	// GET /api/{api_version}/geoip/{ip}. When `ip` is omitted, falls back
	// to the requester's own address, like the REST endpoint does.
	if fieldPresent(q, "geoip") {
		ipArg := extractStringArg(q, "geoip", "ip")
		if ipArg == "" {
			if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
				ipArg = host
			} else {
				ipArg = r.RemoteAddr
			}
		}
		if geo == nil {
			writeJSON(w, http.StatusOK, response{
				Errors: []errorMessage{{Message: i18n.Tf(lang, "graphql.err_geoip_lookup_failed", "ip", ipArg)}},
			})
			return
		}
		location, err := geo.LookupString(ipArg)
		if err != nil {
			writeJSON(w, http.StatusOK, response{
				Errors: []errorMessage{{Message: i18n.Tf(lang, "graphql.err_geoip_lookup_failed", "ip", ipArg)}},
			})
			return
		}
		writeJSON(w, http.StatusOK, response{
			Data: map[string]interface{}{"geoip": location},
		})
		return
	}

	writeJSON(w, http.StatusOK, response{
		Errors: []errorMessage{{Message: i18n.T(lang, "graphql.err_unsupported_query")}},
	})
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
