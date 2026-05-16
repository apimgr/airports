package graphql

import (
	"encoding/json"
	"net/http"
	"regexp"
	"strconv"
	"strings"
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

// serveQuery is the resolver entry point invoked by QueryHandler.
func serveQuery(w http.ResponseWriter, r *http.Request, src AirportSource) {
	// Cap request body to a sane upper bound — protects against an
	// attacker streaming an unbounded "query" body to exhaust memory.
	r.Body = http.MaxBytesReader(w, r.Body, 64*1024)

	var req request
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, response{
			Errors: []errorMessage{{Message: "invalid GraphQL request body: " + err.Error()}},
		})
		return
	}

	q := strings.TrimSpace(req.Query)
	if q == "" {
		writeJSON(w, http.StatusBadRequest, response{
			Errors: []errorMessage{{Message: "query is required"}},
		})
		return
	}

	// `airport(code: "XXXX") { ... }` — extract the code argument.
	if code := extractStringArg(q, "airport", "code"); code != "" {
		apt, err := src.GetByCode(code)
		if err != nil {
			writeJSON(w, http.StatusOK, response{
				Errors: []errorMessage{{Message: "airport not found: " + code}},
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
				Errors: []errorMessage{{Message: "nearby query requires lon"}},
			})
			return
		}
		radius, ok := extractFloatArg(q, "nearby", "radius")
		if !ok {
			radius = 50
		}
		results := src.GetNearbyWithDistance(lat, lon, radius, 20, "metric")
		writeJSON(w, http.StatusOK, response{
			Data: map[string]interface{}{"nearby": results},
		})
		return
	}

	writeJSON(w, http.StatusOK, response{
		Errors: []errorMessage{{Message: "unsupported query — supported: airport(code:), nearby(lat:, lon:, radius:)"}},
	})
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
