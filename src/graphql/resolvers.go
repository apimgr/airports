package graphql

import (
	"encoding/json"
	"fmt"
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

type graphQLToken struct {
	kind       byte
	value      string
	start, end int
}

type graphQLRange struct{ start, end int }

func tokenizeGraphQL(query string) []graphQLToken {
	tokens := make([]graphQLToken, 0, len(query)/4)
	for i := 0; i < len(query); {
		switch c := query[i]; {
		case c == ' ' || c == '\t' || c == '\r' || c == '\n' || c == ',':
			i++
		case c == '#':
			for i < len(query) && query[i] != '\n' && query[i] != '\r' {
				i++
			}
		case c == '"':
			start := i
			if strings.HasPrefix(query[i:], `"""`) {
				i += 3
				for i < len(query) {
					if strings.HasPrefix(query[i:], `\"""`) {
						i += 4
						continue
					}
					if strings.HasPrefix(query[i:], `"""`) {
						i += 3
						break
					}
					i++
				}
			} else {
				i++
				for i < len(query) {
					if query[i] == '\\' {
						if i+1 < len(query) {
							i += 2
						} else {
							i++
						}
						continue
					}
					if query[i] == '"' {
						i++
						break
					}
					i++
				}
			}
			tokens = append(tokens, graphQLToken{kind: 's', value: query[start:i], start: start, end: i})
		case isGraphQLNameStart(c):
			start := i
			for i < len(query) && isGraphQLNameContinue(query[i]) {
				i++
			}
			tokens = append(tokens, graphQLToken{kind: 'n', value: query[start:i], start: start, end: i})
		default:
			tokens = append(tokens, graphQLToken{kind: c, value: string(c), start: i, end: i + 1})
			i++
		}
	}
	return tokens
}

func isGraphQLNameStart(c byte) bool {
	return c == '_' || c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z'
}

func isGraphQLNameContinue(c byte) bool {
	return isGraphQLNameStart(c) || c >= '0' && c <= '9'
}

func graphQLVariableDefinitionRanges(tokens []graphQLToken) []graphQLRange {
	var ranges []graphQLRange
	braceDepth, parenDepth, bracketDepth := 0, 0, 0
	for i := 0; i < len(tokens); i++ {
		token := tokens[i]
		if token.kind == 'n' && braceDepth == 0 && parenDepth == 0 && bracketDepth == 0 &&
			(token.value == "query" || token.value == "mutation" || token.value == "subscription") {
			j := i + 1
			if j < len(tokens) && tokens[j].kind == 'n' {
				j++
			}
			if j < len(tokens) && tokens[j].kind == '(' {
				depth := 1
			definition:
				for k := j + 1; k < len(tokens); k++ {
					switch tokens[k].kind {
					case '(':
						depth++
					case ')':
						depth--
						if depth == 0 {
							ranges = append(ranges, graphQLRange{start: tokens[j].start, end: tokens[k].end})
							i = k
							break definition
						}
					}
				}
			}
		}
		switch token.kind {
		case '{':
			braceDepth++
		case '}':
			braceDepth--
		case '(':
			parenDepth++
		case ')':
			parenDepth--
		case '[':
			bracketDepth++
		case ']':
			bracketDepth--
		}
	}
	return ranges
}

func applyGraphQLVariables(query string, variables map[string]interface{}) (string, error) {
	tokens := tokenizeGraphQL(query)
	definitions := graphQLVariableDefinitionRanges(tokens)
	var output strings.Builder
	output.Grow(len(query))
	last := 0
	for i := 0; i+1 < len(tokens); i++ {
		dollar, name := tokens[i], tokens[i+1]
		if dollar.kind != '$' || name.kind != 'n' || dollar.end != name.start || inGraphQLRange(dollar.start, definitions) {
			continue
		}
		value, ok := variables[name.value]
		if !ok {
			return "", fmt.Errorf("missing variable %s", name.value)
		}
		encoded, err := json.Marshal(value)
		if err != nil {
			return "", fmt.Errorf("invalid variable %s", name.value)
		}
		output.WriteString(query[last:dollar.start])
		output.Write(encoded)
		last = name.end
		i++
	}
	output.WriteString(query[last:])
	return output.String(), nil
}

func inGraphQLRange(position int, ranges []graphQLRange) bool {
	for _, current := range ranges {
		if position >= current.start && position < current.end {
			return true
		}
	}
	return false
}

// serveQuery is the resolver entry point invoked by QueryHandler.
func serveQuery(w http.ResponseWriter, r *http.Request, src AirportSource, geo GeoIPSource) {
	lang := i18n.FromContext(r.Context())

	var req request
	if r.Method == http.MethodGet {
		values := r.URL.Query()
		for key := range values {
			if key != "query" && key != "variables" && key != "operationName" {
				writeJSON(w, http.StatusBadRequest, response{Errors: []errorMessage{{Message: i18n.TranslateFormattedText(lang, "graphql.err_invalid_get_parameter", "parameter", key)}}})
				return
			}
		}
		req.Query = values.Get("query")
		req.OperationName = values.Get("operationName")
		if raw := values.Get("variables"); raw != "" {
			if len(raw) > 64*1024 || json.Unmarshal([]byte(raw), &req.Variables) != nil {
				writeJSON(w, http.StatusBadRequest, response{Errors: []errorMessage{{Message: i18n.TranslateText(lang, "graphql.err_invalid_variables")}}})
				return
			}
		}
	} else {
		// Cap request bodies to protect against unbounded JSON input.
		r.Body = http.MaxBytesReader(w, r.Body, 64*1024)
		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()
		if err := dec.Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, response{
				Errors: []errorMessage{{Message: i18n.TranslateFormattedText(lang, "graphql.err_invalid_body", "error", err.Error())}},
			})
			return
		}
	}

	q := strings.TrimSpace(req.Query)
	if q != "" {
		var err error
		q, err = applyGraphQLVariables(q, req.Variables)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, response{
				Errors: []errorMessage{{Message: i18n.TranslateText(lang, "graphql.err_invalid_variables")}},
			})
			return
		}
	}
	if q == "" {
		writeJSON(w, http.StatusBadRequest, response{
			Errors: []errorMessage{{Message: i18n.TranslateText(lang, "graphql.err_query_required")}},
		})
		return
	}

	// `airport(code: "XXXX") { ... }` — extract the code argument.
	if code := extractStringArg(q, "airport", "code"); code != "" {
		apt, err := src.GetByCode(code)
		if err != nil {
			writeJSON(w, http.StatusOK, response{
				Errors: []errorMessage{{Message: i18n.TranslateFormattedText(lang, "graphql.err_airport_not_found", "code", code)}},
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
				Errors: []errorMessage{{Message: i18n.TranslateText(lang, "graphql.err_nearby_requires_lon")}},
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
				Errors: []errorMessage{{Message: i18n.TranslateText(lang, "graphql.err_within_requires_bounds")}},
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
				Errors: []errorMessage{{Message: i18n.TranslateText(lang, "graphql.err_query_too_short")}},
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
				Errors: []errorMessage{{Message: i18n.TranslateFormattedText(lang, "graphql.err_geoip_lookup_failed", "ip", ipArg)}},
			})
			return
		}
		location, err := geo.LookupString(ipArg)
		if err != nil {
			writeJSON(w, http.StatusOK, response{
				Errors: []errorMessage{{Message: i18n.TranslateFormattedText(lang, "graphql.err_geoip_lookup_failed", "ip", ipArg)}},
			})
			return
		}
		writeJSON(w, http.StatusOK, response{
			Data: map[string]interface{}{"geoip": location},
		})
		return
	}

	writeJSON(w, http.StatusOK, response{
		Errors: []errorMessage{{Message: i18n.TranslateText(lang, "graphql.err_unsupported_query")}},
	})
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
