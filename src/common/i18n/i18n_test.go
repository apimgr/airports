package i18n

import (
	"context"
	"net/http"
	"net/http/httptest"
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
	if err := Load(); err != nil {
		t.Fatalf("Load() error = %v", err)
	}
}

func TestKeyParity(t *testing.T) {
	if err := Load(); err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	mu.RLock()
	defer mu.RUnlock()

	enLocale, ok := locales[DefaultLanguage]
	if !ok {
		t.Fatalf("missing %s locale", DefaultLanguage)
	}
	enKeys := allKeys(enLocale)

	for _, code := range SupportedLanguages {
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

func TestMetaComplete(t *testing.T) {
	if err := Load(); err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	mu.RLock()
	defer mu.RUnlock()

	for _, code := range SupportedLanguages {
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
	if got := T("es", "common.save"); got != "Guardar" {
		t.Errorf("T(es, common.save) = %q, want Guardar", got)
	}
	if got := T("xx", "common.save"); got != "Save" {
		t.Errorf("T(xx, common.save) = %q, want fallback to English %q", got, "Save")
	}
	if got := T("en", "common.does_not_exist"); got != "common.does_not_exist" {
		t.Errorf("T with missing key = %q, want key echoed back", got)
	}
}

func TestTf(t *testing.T) {
	got := Tf("en", "common.page_x_of_y", "current", 2, "total", 5)
	want := "Page 2 of 5"
	if got != want {
		t.Errorf("Tf() = %q, want %q", got, want)
	}
}

func TestTp(t *testing.T) {
	if got := Tp("ar", "common.save", 0); got == "" {
		t.Error("Tp() returned empty string")
	}
	got := Tp("en", "common.showing_x_of_y", 3, "total", 10)
	if got == "" {
		t.Error("Tp() returned empty string")
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
	if got := ResolveLanguage(req); got != DefaultLanguage {
		t.Errorf("ResolveLanguage(none) = %q, want %q", got, DefaultLanguage)
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

func TestFromContextDefault(t *testing.T) {
	if got := FromContext(context.Background()); got != DefaultLanguage {
		t.Errorf("FromContext(empty) = %q, want %q", got, DefaultLanguage)
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
