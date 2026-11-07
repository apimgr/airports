// Package i18n provides translation, pluralization, and language-detection
// support shared by the airports server and airports-cli client (PART 30).
package i18n

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

//go:embed locales/*.json
var localeFS embed.FS

// DefaultLanguage is the fallback language used when a requested language
// or key is unavailable.
var DefaultLanguage = "en"

// SupportedLanguages lists every language code shipped with the binary,
// per PART 30's required language set.
var SupportedLanguages = []string{"en", "es", "zh", "fr", "ar", "de", "ja"}

// Config controls runtime language availability and preference persistence.
type Config struct {
	Enabled            bool
	DefaultLanguage    string
	AvailableLanguages []string
	FallbackLanguage   string
	CookieName         string
	CookieMaxAge       string
}

var settingsMu sync.RWMutex
var languageConfig = Config{
	Enabled: true, DefaultLanguage: "en", AvailableLanguages: []string{"en", "es", "zh", "fr", "ar", "de", "ja"},
	FallbackLanguage: "en", CookieName: "lang", CookieMaxAge: "365d",
}

// Configure applies server language settings before requests are served.
func Configure(cfg Config) {
	_ = Load()
	mu.RLock()
	available := make([]string, 0, len(cfg.AvailableLanguages))
	seen := make(map[string]bool, len(cfg.AvailableLanguages))
	for _, code := range cfg.AvailableLanguages {
		code = strings.ToLower(strings.TrimSpace(code))
		if _, exists := locales[code]; exists && !seen[code] {
			available = append(available, code)
			seen[code] = true
		}
	}
	mu.RUnlock()
	if len(available) == 0 {
		available = []string{"en"}
	}
	fallback := strings.ToLower(strings.TrimSpace(cfg.FallbackLanguage))
	if !contains(available, fallback) {
		fallback = "en"
	}
	defaultLanguage := strings.ToLower(strings.TrimSpace(cfg.DefaultLanguage))
	if !contains(available, defaultLanguage) {
		defaultLanguage = fallback
	}
	cookieName := strings.TrimSpace(cfg.CookieName)
	if cookieName == "" {
		cookieName = "lang"
	}
	cfg.AvailableLanguages = available
	cfg.DefaultLanguage = defaultLanguage
	cfg.FallbackLanguage = fallback
	cfg.CookieName = cookieName
	settingsMu.Lock()
	languageConfig = cfg
	DefaultLanguage = defaultLanguage
	SupportedLanguages = append([]string(nil), available...)
	settingsMu.Unlock()
}

func currentConfig() Config {
	settingsMu.RLock()
	defer settingsMu.RUnlock()
	cfg := languageConfig
	cfg.AvailableLanguages = append([]string(nil), cfg.AvailableLanguages...)
	return cfg
}

func contains(values []string, value string) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}

func cookieAge(value string) int {
	value = strings.TrimSpace(value)
	if strings.HasSuffix(value, "d") {
		days, err := strconv.Atoi(strings.TrimSuffix(value, "d"))
		if err == nil && days > 0 {
			return days * 24 * 60 * 60
		}
	}
	duration, err := time.ParseDuration(value)
	if err != nil || duration <= 0 {
		return 365 * 24 * 60 * 60
	}
	return int(duration.Seconds())
}

// Meta holds the metadata block from a locale file.
type Meta struct {
	Language   string `json:"language"`
	Name       string `json:"name"`
	NativeName string `json:"native_name"`
	Direction  string `json:"direction"`
	Version    string `json:"version"`
}

type locale struct {
	Meta Meta
	Data map[string]map[string]string
}

var (
	mu       sync.RWMutex
	locales  = map[string]*locale{}
	loadOnce sync.Once
	loadErr  error
)

// Load parses every embedded locale file into memory. It is safe to call
// multiple times; the actual load runs only once.
func Load() error {
	loadOnce.Do(func() {
		loadErr = loadAll()
	})
	return loadErr
}

func loadAll() error {
	entries, err := localeFS.ReadDir("locales")
	if err != nil {
		return fmt.Errorf("i18n: read locales dir: %w", err)
	}

	mu.Lock()
	defer mu.Unlock()

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		raw, err := localeFS.ReadFile("locales/" + entry.Name())
		if err != nil {
			return fmt.Errorf("i18n: read %s: %w", entry.Name(), err)
		}

		var full map[string]json.RawMessage
		if err := json.Unmarshal(raw, &full); err != nil {
			return fmt.Errorf("i18n: parse %s: %w", entry.Name(), err)
		}

		l := &locale{Data: map[string]map[string]string{}}

		if metaRaw, ok := full["meta"]; ok {
			if err := json.Unmarshal(metaRaw, &l.Meta); err != nil {
				return fmt.Errorf("i18n: parse meta in %s: %w", entry.Name(), err)
			}
		}

		for ns, nsRaw := range full {
			if ns == "meta" {
				continue
			}
			var kv map[string]string
			if err := json.Unmarshal(nsRaw, &kv); err != nil {
				return fmt.Errorf("i18n: parse namespace %q in %s: %w", ns, entry.Name(), err)
			}
			l.Data[ns] = kv
		}

		lang := l.Meta.Language
		if lang == "" {
			lang = strings.TrimSuffix(entry.Name(), ".json")
		}
		locales[lang] = l
	}

	return nil
}

// IsSupported reports whether lang is one of the shipped languages.
func IsSupported(lang string) bool {
	return contains(currentConfig().AvailableLanguages, normalizeLanguage(lang))
}

func normalizeLanguage(lang string) string {
	return strings.ToLower(strings.TrimSpace(lang))
}

// Direction returns "ltr" or "rtl" for the given language, defaulting to
// "ltr" for unknown languages.
func Direction(lang string) string {
	_ = Load()
	mu.RLock()
	defer mu.RUnlock()
	if l, ok := locales[lang]; ok && l.Meta.Direction != "" {
		return l.Meta.Direction
	}
	return "ltr"
}

// AvailableLanguage describes one language entry for a language selector UI.
type AvailableLanguage struct {
	Code       string
	Name       string
	NativeName string
	Direction  string
}

// RawLocale returns the raw embedded JSON bytes for lang, for serving
// directly to WebUI JavaScript at /locales/{lang}.json (PART 30 "WebUI
// JavaScript" row). Falls back to DefaultLanguage when lang is unsupported.
func RawLocale(lang string) ([]byte, error) {
	cfg := currentConfig()
	if !IsSupported(lang) {
		lang = cfg.FallbackLanguage
	}
	return localeFS.ReadFile("locales/" + lang + ".json")
}

// AvailableLanguages returns metadata for every enabled language, in config order.
func AvailableLanguages() []AvailableLanguage {
	_ = Load()
	mu.RLock()
	defer mu.RUnlock()

	cfg := currentConfig()
	out := make([]AvailableLanguage, 0, len(cfg.AvailableLanguages))
	for _, code := range cfg.AvailableLanguages {
		l, ok := locales[code]
		if !ok {
			continue
		}
		out = append(out, AvailableLanguage{
			Code:       code,
			Name:       l.Meta.Name,
			NativeName: l.Meta.NativeName,
			Direction:  l.Meta.Direction,
		})
	}
	return out
}

// lookup splits a dotted key ("namespace.key") and returns the raw string
// for lang, falling back to DefaultLanguage, then to the key itself.
func lookup(lang, key string) string {
	_ = Load()

	ns, k, ok := strings.Cut(key, ".")
	if !ok {
		return key
	}

	mu.RLock()
	defer mu.RUnlock()

	if l, ok := locales[lang]; ok {
		if v, ok := l.Data[ns][k]; ok {
			return v
		}
	}

	fallback := currentConfig().FallbackLanguage
	if lang != fallback {
		if l, ok := locales[fallback]; ok {
			if v, ok := l.Data[ns][k]; ok {
				return v
			}
		}
	}

	return key
}

// T translates key ("namespace.key") into lang, falling back to English
// and finally to the raw key when no translation exists.
func T(lang, key string) string {
	return lookup(lang, key)
}

// Tf translates key and substitutes named placeholders of the form
// "{name}" using args, an alternating sequence of name, value pairs.
func Tf(lang, key string, args ...interface{}) string {
	s := lookup(lang, key)
	for i := 0; i+1 < len(args); i += 2 {
		name, ok := args[i].(string)
		if !ok {
			continue
		}
		s = strings.ReplaceAll(s, "{"+name+"}", toStr(args[i+1]))
	}
	return s
}

// Tp translates a pluralizable key. It first looks for "key.category"
// using the CLDR plural category for count in lang (e.g. "errors.item.one",
// "errors.item.other"); if that specific key is missing it falls back to
// the bare key.
func Tp(lang, key string, count int, args ...interface{}) string {
	category := pluralCategory(lang, count)
	candidate := key + "." + category
	s := lookup(lang, candidate)
	if s == candidate {
		s = lookup(lang, key)
	}
	s = strings.ReplaceAll(s, "{count}", strconv.Itoa(count))
	for i := 0; i+1 < len(args); i += 2 {
		name, ok := args[i].(string)
		if !ok {
			continue
		}
		s = strings.ReplaceAll(s, "{"+name+"}", toStr(args[i+1]))
	}
	return s
}

func toStr(v interface{}) string {
	switch t := v.(type) {
	case string:
		return t
	case fmt.Stringer:
		return t.String()
	default:
		return fmt.Sprintf("%v", v)
	}
}

// pluralCategory returns the CLDR plural category (zero, one, two, few,
// many, other) for count in lang, per PART 30's Supported Languages table.
func pluralCategory(lang string, count int) string {
	n := count
	if n < 0 {
		n = -n
	}

	switch lang {
	case "zh", "ja":
		return "other"
	case "fr":
		if n == 0 || n == 1 {
			return "one"
		}
		return "other"
	case "ar":
		switch {
		case n == 0:
			return "zero"
		case n == 1:
			return "one"
		case n == 2:
			return "two"
		case n%100 >= 3 && n%100 <= 10:
			return "few"
		case n%100 >= 11 && n%100 <= 99:
			return "many"
		default:
			return "other"
		}
	default:
		if n == 1 {
			return "one"
		}
		return "other"
	}
}

// contextKey is an unexported type for context keys defined in this package.
type contextKey int

const langContextKey contextKey = 0

// SetLanguage returns a new context carrying the resolved language.
func SetLanguage(ctx context.Context, lang string) context.Context {
	return context.WithValue(ctx, langContextKey, lang)
}

// FromContext returns the resolved language stored in ctx, or
// DefaultLanguage if none was set.
func FromContext(ctx context.Context) string {
	if lang, ok := ctx.Value(langContextKey).(string); ok && lang != "" {
		return lang
	}
	return currentConfig().DefaultLanguage
}

// ParseAcceptLanguage parses an Accept-Language header and returns the
// highest-quality supported language, or "" if none match.
func ParseAcceptLanguage(header string) string {
	if header == "" {
		return ""
	}

	type candidate struct {
		lang string
		q    float64
	}

	var candidates []candidate
	for _, part := range strings.Split(header, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		segs := strings.Split(part, ";")
		tag := strings.TrimSpace(segs[0])
		q := 1.0
		for _, seg := range segs[1:] {
			seg = strings.TrimSpace(seg)
			if strings.HasPrefix(seg, "q=") {
				if parsed, err := strconv.ParseFloat(strings.TrimPrefix(seg, "q="), 64); err == nil {
					q = parsed
				}
			}
		}
		base, _, _ := strings.Cut(tag, "-")
		candidates = append(candidates, candidate{lang: strings.ToLower(base), q: q})
	}

	best := ""
	bestQ := -1.0
	for _, c := range candidates {
		if IsSupported(c.lang) && c.q > bestQ {
			best = c.lang
			bestQ = c.q
		}
	}
	return best
}

// ResolveLanguage applies the documented fallback chain for HTTP requests:
// ?lang= query param -> lang cookie -> Accept-Language header -> default.
func ResolveLanguage(r *http.Request) string {
	return resolveLanguage(r, true)
}

func resolveLanguage(r *http.Request, allowCookie bool) string {
	cfg := currentConfig()
	if !cfg.Enabled {
		return cfg.DefaultLanguage
	}
	if q := normalizeLanguage(r.URL.Query().Get("lang")); q != "" && IsSupported(q) {
		return q
	}
	if allowCookie {
		if c, err := r.Cookie(cfg.CookieName); err == nil && IsSupported(c.Value) {
			return normalizeLanguage(c.Value)
		}
	}
	if al := ParseAcceptLanguage(r.Header.Get("Accept-Language")); al != "" {
		return al
	}
	return cfg.DefaultLanguage
}

// LanguageMiddleware resolves the request language per PART 30's fallback
// chain, stores it in the request context, and sets the lang cookie when
// the language was chosen via ?lang=.
func LanguageMiddleware(next http.Handler) http.Handler {
	return LanguageMiddlewareWithPreferenceGate(next, nil)
}

// LanguageMiddlewareWithPreferenceGate resolves language for the current
// request and optionally gates persistence of the ?lang= choice.
func LanguageMiddlewareWithPreferenceGate(next http.Handler, canSetPreferenceCookie func(*http.Request) bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg := currentConfig()
		canPersist := canSetPreferenceCookie == nil || canSetPreferenceCookie(r)
		lang := resolveLanguage(r, canPersist)
		queryLanguage := normalizeLanguage(r.URL.Query().Get("lang"))
		if cfg.Enabled && canPersist && queryLanguage != "" && IsSupported(queryLanguage) {
			http.SetCookie(w, &http.Cookie{
				Name:     cfg.CookieName,
				Value:    queryLanguage,
				Path:     "/",
				MaxAge:   cookieAge(cfg.CookieMaxAge),
				SameSite: http.SameSiteLaxMode,
				Secure:   r.TLS != nil,
				HttpOnly: true,
			})
		}

		ctx := SetLanguage(r.Context(), lang)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
