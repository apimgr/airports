package i18n

import (
	"context"
	"net/http"
	"net/http/httptest"
	"regexp"
	"sort"
	"testing"
)

func allKeys(l *locale) map[string]bool {
	keys := map[string]bool{}
	for ns, kv := range l.Data {
		for k := range kv {
			keys[ns+"."+k] = true
		}
	}
	return keys
}

func TestLoad(t *testing.T) {
	if err := LoadEmbeddedLocales(); err != nil {
		t.Fatalf("LoadEmbeddedLocales() error = %v", err)
	}
}

func TestLanguageRegistryMinimumAndNormalization(t *testing.T) {
	wantRequired := []string{"en", "es", "fr", "ar", "de", "it"}
	if got := RequiredLanguageCodes(); !equalStrings(got, wantRequired) {
		t.Fatalf("RequiredLanguageCodes() = %v, want %v", got, wantRequired)
	}
	got := NormalizeAvailableLanguages([]string{"ja", "en", "xx", "ja"})
	want := []string{"ja", "en", "es", "fr", "ar", "de", "it"}
	if !equalStrings(got, want) {
		t.Errorf("NormalizeAvailableLanguages() = %v, want %v", got, want)
	}
	if got := NormalizeAvailableLanguages(nil); !equalStrings(got, RegisteredLanguageCodes()) {
		t.Errorf("empty available-language config = %v, want all registered languages", got)
	}
}

func equalStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

func TestKeyParity(t *testing.T) {
	if err := LoadEmbeddedLocales(); err != nil {
		t.Fatalf("LoadEmbeddedLocales() error = %v", err)
	}

	mu.RLock()
	defer mu.RUnlock()

	enLocale, ok := locales[DefaultLanguage]
	if !ok {
		t.Fatalf("missing %s locale", DefaultLanguage)
	}
	enKeys := allKeys(enLocale)

	for _, code := range RegisteredLanguageCodes() {
		l, ok := locales[code]
		if !ok {
			t.Errorf("missing locale file for %q", code)
			continue
		}
		gotKeys := allKeys(l)

		var missing, extra []string
		for k := range enKeys {
			if !gotKeys[k] {
				missing = append(missing, k)
			}
		}
		for k := range gotKeys {
			if !enKeys[k] {
				extra = append(extra, k)
			}
		}
		sort.Strings(missing)
		sort.Strings(extra)

		if len(missing) > 0 {
			t.Errorf("locale %q missing keys present in %s: %v", code, DefaultLanguage, missing)
		}
		if len(extra) > 0 {
			t.Errorf("locale %q has keys not present in %s: %v", code, DefaultLanguage, extra)
		}
	}
}

func TestPlaceholderParity(t *testing.T) {
	if err := LoadEmbeddedLocales(); err != nil {
		t.Fatalf("LoadEmbeddedLocales() error = %v", err)
	}

	mu.RLock()
	defer mu.RUnlock()

	base := locales[DefaultLanguage]
	for _, code := range RegisteredLanguageCodes() {
		localized := locales[code]
		for namespace, messages := range base.Data {
			for key, source := range messages {
				localizedValue, exists := localized.Data[namespace][key]
				if !exists {
					continue
				}
				if !equalStrings(messagePlaceholders(localizedValue), messagePlaceholders(source)) {
					t.Errorf("locale %q key %s.%s has placeholders %v, want %v", code, namespace, key, messagePlaceholders(localizedValue), messagePlaceholders(source))
				}
			}
		}
	}
}

var messagePlaceholderPattern = regexp.MustCompile(`\{[A-Za-z0-9_]+\}`)
var messageFormatVerbPattern = regexp.MustCompile(`%[a-zA-Z]`)

func messagePlaceholders(value string) []string {
	placeholders := messagePlaceholderPattern.FindAllString(value, -1)
	placeholders = append(placeholders, messageFormatVerbPattern.FindAllString(value, -1)...)
	sort.Strings(placeholders)
	return placeholders
}

func TestMetaComplete(t *testing.T) {
	if err := LoadEmbeddedLocales(); err != nil {
		t.Fatalf("LoadEmbeddedLocales() error = %v", err)
	}

	mu.RLock()
	defer mu.RUnlock()

	for _, code := range RegisteredLanguageCodes() {
		l, ok := locales[code]
		if !ok {
			t.Errorf("missing locale file for %q", code)
			continue
		}
		if l.Meta.Language != code {
			t.Errorf("locale %q: meta.language = %q, want %q", code, l.Meta.Language, code)
		}
		if l.Meta.Name == "" {
			t.Errorf("locale %q: meta.name is empty", code)
		}
		if l.Meta.NativeName == "" {
			t.Errorf("locale %q: meta.native_name is empty", code)
		}
		if l.Meta.Direction != "ltr" && l.Meta.Direction != "rtl" {
			t.Errorf("locale %q: meta.direction = %q, want ltr or rtl", code, l.Meta.Direction)
		}
	}
}

func TestDirection(t *testing.T) {
	if got := Direction("ar"); got != "rtl" {
		t.Errorf("Direction(ar) = %q, want rtl", got)
	}
	if got := Direction("en"); got != "ltr" {
		t.Errorf("Direction(en) = %q, want ltr", got)
	}
	if got := Direction("xx"); got != "ltr" {
		t.Errorf("Direction(xx) = %q, want ltr (default)", got)
	}
}

func TestIsSupported(t *testing.T) {
	if !IsSupported("en") {
		t.Error("IsSupported(en) = false, want true")
	}
	if IsSupported("xx") {
		t.Error("IsSupported(xx) = true, want false")
	}
}

func TestT(t *testing.T) {
	if got := TranslateText("es", "common.save"); got != "Guardar" {
		t.Errorf("TranslateText(es, common.save) = %q, want Guardar", got)
	}
	if got := TranslateText("xx", "common.save"); got != "Save" {
		t.Errorf("TranslateText(xx, common.save) = %q, want fallback to English %q", got, "Save")
	}
	if got := TranslateText("en", "common.does_not_exist"); got != "common.does_not_exist" {
		t.Errorf("T with missing key = %q, want key echoed back", got)
	}
}

func TestTf(t *testing.T) {
	got := TranslateFormattedText("en", "common.page_x_of_y", "current", 2, "total", 5)
	want := "Page 2 of 5"
	if got != want {
		t.Errorf("TranslateFormattedText() = %q, want %q", got, want)
	}
}

func TestTp(t *testing.T) {
	if got := TranslatePluralText("ar", "common.save", 0); got == "" {
		t.Error("TranslatePluralText() returned empty string")
	}
	got := TranslatePluralText("en", "common.showing_x_of_y", 3, "total", 10)
	if got == "" {
		t.Error("TranslatePluralText() returned empty string")
	}
}

func TestPluralCategory(t *testing.T) {
	cases := []struct {
		lang  string
		count int
		want  string
	}{
		{"en", 1, "one"},
		{"en", 2, "other"},
		{"en", 0, "other"},
		{"fr", 0, "one"},
		{"fr", 1, "one"},
		{"fr", 2, "other"},
		{"it", 1, "one"},
		{"it", 2, "other"},
		{"it", 1_000_000, "many"},
		{"it", 2_000_000, "many"},
		{"it", 1_000_001, "other"},
		{"zh", 5, "other"},
		{"ja", 5, "other"},
		{"ar", 0, "zero"},
		{"ar", 1, "one"},
		{"ar", 2, "two"},
		{"ar", 5, "few"},
		{"ar", 20, "many"},
		{"ar", 100, "other"},
	}
	for _, c := range cases {
		if got := pluralCategory(c.lang, c.count); got != c.want {
			t.Errorf("pluralCategory(%q, %d) = %q, want %q", c.lang, c.count, got, c.want)
		}
	}
}

func TestParseAcceptLanguage(t *testing.T) {
	cases := []struct {
		header string
		want   string
	}{
		{"", ""},
		{"es", "es"},
		{"fr-CA,fr;q=0.9,en;q=0.8", "fr"},
		{"xx,yy;q=0.9", ""},
		{"xx;q=0.9,de;q=0.5", "de"},
	}
	for _, c := range cases {
		if got := ParseAcceptLanguage(c.header); got != c.want {
			t.Errorf("ParseAcceptLanguage(%q) = %q, want %q", c.header, got, c.want)
		}
	}
}

func TestResolveLanguage(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/?lang=es", nil)
	if got := ResolveLanguage(req); got != "es" {
		t.Errorf("ResolveLanguage(query) = %q, want es", got)
	}

	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: "lang", Value: "de"})
	if got := ResolveLanguage(req); got != "de" {
		t.Errorf("ResolveLanguage(cookie) = %q, want de", got)
	}

	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept-Language", "ja,en;q=0.5")
	if got := ResolveLanguage(req); got != "ja" {
		t.Errorf("ResolveLanguage(header) = %q, want ja", got)
	}

	req = httptest.NewRequest(http.MethodGet, "/", nil)
	if got := ResolveLanguage(req); got != "en" {
		t.Errorf("ResolveLanguage(none) = %q, want en", got)
	}
}

func TestLanguageMiddleware(t *testing.T) {
	var seenLang string
	handler := LanguageMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenLang = FromContext(r.Context())
	}))

	req := httptest.NewRequest(http.MethodGet, "/?lang=fr", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if seenLang != "fr" {
		t.Errorf("context language = %q, want fr", seenLang)
	}

	found := false
	for _, c := range rec.Result().Cookies() {
		if c.Name == "lang" && c.Value == "fr" {
			found = true
			if c.SameSite != http.SameSiteLaxMode {
				t.Errorf("cookie SameSite = %v, want Lax", c.SameSite)
			}
			if !c.HttpOnly {
				t.Error("cookie HttpOnly = false, want true")
			}
		}
	}
	if !found {
		t.Error("lang cookie not set")
	}
}

func TestConfigureAppliesLanguageSettings(t *testing.T) {
	ConfigureLanguageSupport(LanguageConfig{
		DefaultLanguage: "fr", AvailableLanguages: []string{"en", "fr"},
	})
	t.Cleanup(func() {
		ConfigureLanguageSupport(LanguageConfig{
			DefaultLanguage: "en", AvailableLanguages: []string{"en", "es", "zh", "fr", "ar", "de", "ja"},
		})
	})

	if got := ResolveLanguage(httptest.NewRequest(http.MethodGet, "/", nil)); got != "en" {
		t.Fatalf("ResolveLanguage(no preference) = %q, want fixed English fallback", got)
	}
	if got := NotificationLanguage(); got != "fr" {
		t.Fatalf("NotificationLanguage() = %q, want configured fr", got)
	}
	if got := ResolveLanguage(httptest.NewRequest(http.MethodGet, "/?lang=FR", nil)); got != "fr" {
		t.Fatalf("ResolveLanguage(query) = %q, want normalized fr", got)
	}
	if !IsSupported("es") || !IsSupported("fr") {
		t.Fatal("required and configured languages were not applied")
	}

	request := httptest.NewRequest(http.MethodGet, "/?lang=en", nil)
	response := httptest.NewRecorder()
	LanguageMiddleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})).ServeHTTP(response, request)
	cookie := response.Result().Cookies()
	if len(cookie) != 1 || cookie[0].Name != LanguageCookieName || cookie[0].MaxAge != LanguageCookieMaxAge {
		t.Fatalf("language cookie = %#v, want fixed lang name and one-year age", cookie)
	}
}

func TestResolveOutputLanguage(t *testing.T) {
	tests := []struct {
		name       string
		flagLang   string
		configLang string
		lcAll      string
		lang       string
		want       string
	}{
		{name: "flag first", flagLang: "fr", configLang: "de", lcAll: "es_ES.UTF-8", lang: "ja", want: "fr"},
		{name: "config before environment", configLang: "de", lcAll: "es_ES.UTF-8", lang: "ja", want: "de"},
		{name: "LC_ALL before LANG", lcAll: "es_ES.UTF-8", lang: "ja", want: "es"},
		{name: "LANG fallback", lang: "zh_CN.UTF-8", want: "zh"},
		{name: "unsupported falls back to English", flagLang: "xx", configLang: "fr", lang: "de", want: "en"},
		{name: "auto detects environment", configLang: "auto", lang: "ja_JP.UTF-8", want: "ja"},
		{name: "default English", configLang: "auto", want: "en"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("LC_ALL", tt.lcAll)
			t.Setenv("LANG", tt.lang)
			if got := ResolveOutputLanguage(tt.flagLang, tt.configLang); got != tt.want {
				t.Errorf("ResolveOutputLanguage(%q, %q) = %q, want %q", tt.flagLang, tt.configLang, got, tt.want)
			}
		})
	}
}

func TestFromContextDefault(t *testing.T) {
	if got := FromContext(context.Background()); got != "en" {
		t.Errorf("FromContext(empty) = %q, want en", got)
	}
}

func TestAvailableLanguages(t *testing.T) {
	langs := AvailableLanguages()
	if len(langs) != len(SupportedLanguages) {
		t.Errorf("AvailableLanguages() len = %d, want %d", len(langs), len(SupportedLanguages))
	}
	for _, l := range langs {
		if l.NativeName == "" {
			t.Errorf("language %q has empty NativeName", l.Code)
		}
	}
}
