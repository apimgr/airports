//go:build e2e

package e2e

import (
	"context"
	"encoding/base64"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/cdproto/fetch"
	"github.com/chromedp/cdproto/log"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
)

func baseURL(t *testing.T) string {
	t.Helper()
	base := strings.TrimRight(os.Getenv("E2E_BASE_URL"), "/")
	if base == "" {
		base = "http://127.0.0.1:64581"
	}
	return base
}

func acceptedPreferencesCookie() *http.Cookie {
	value, _ := json.Marshal(map[string]interface{}{"essential": true, "preferences": true, "analytics": false, "timestamp": time.Now().Unix()})
	return &http.Cookie{Name: "cookie_consent", Value: url.PathEscape(string(value)), Path: "/"}
}

func getPage(t *testing.T, path string) (int, http.Header, string) {
	return getPageAccept(t, path, "text/html")
}

func getPageAccept(t *testing.T, path, accept string) (int, http.Header, string) {
	t.Helper()
	request, err := http.NewRequest(http.MethodGet, baseURL(t)+path, nil)
	if err != nil {
		t.Fatalf("create request for %s: %v", path, err)
	}
	if accept != "" {
		request.Header.Set("Accept", accept)
	}
	response, err := (&http.Client{Timeout: 20 * time.Second}).Do(request)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return response.StatusCode, response.Header, string(body)
}

func assertSSRPage(t *testing.T, path string, expected ...string) {
	t.Helper()
	status, headers, body := getPage(t, path)
	if status != http.StatusOK {
		t.Fatalf("%s returned HTTP %d", path, status)
	}
	if !strings.Contains(headers.Get("Content-Type"), "text/html") {
		t.Fatalf("%s content type = %q, want HTML", path, headers.Get("Content-Type"))
	}
	for _, text := range expected {
		if !strings.Contains(body, text) {
			t.Errorf("SSR for %s does not contain %q", path, text)
		}
	}
	for _, marker := range []string{`<html`, `<title`, `name="viewport"`, `charset`} {
		if !strings.Contains(strings.ToLower(body), marker) {
			t.Errorf("SSR for %s is missing %s", path, marker)
		}
	}
}

func hasInputValue(html, name, value string) bool {
	pattern := regexp.MustCompile(`(?is)<input\b[^>]*\bname="` + regexp.QuoteMeta(name) + `"[^>]*\bvalue="` + regexp.QuoteMeta(value) + `"`)
	return pattern.MatchString(html)
}

func TestSSR_HomeHasRenderedSearchAndNavigation(t *testing.T) {
	assertSSRPage(t, "/", "Search", "/airports/search", "/airports/nearby")
}

func TestSSR_SearchReturnsAirportData(t *testing.T) {
	assertSSRPage(t, "/airports/search?q="+url.QueryEscape("JFK"), "JFK", "airport-card")
}

func TestSSR_AirportDirectoryRendersAirportCardsAndPagination(t *testing.T) {
	assertSSRPage(t, "/airports", "airport-card", "pagination")
}

func TestSSR_AirportDetailContainsCoordinatesAndMapBaseline(t *testing.T) {
	assertSSRPage(t, "/airports/KJFK", "KJFK", "airport-map", "openstreetmap.org")
}

func TestSSR_HealthPageShowsFullStatusAndComponentSections(t *testing.T) {
	assertSSRPage(t, "/server/healthz", "System Status", "Component Status", "Server Statistics")
}

func TestSSR_BoundingBoxSearchRendersAirportData(t *testing.T) {
	assertSSRPage(t, "/airports/nearby?minLat=40&maxLat=41&minLon=-74&maxLon=-73", "results-grid", "airport-map")
}

func TestSSR_NearbyRadiusAndNearestLimitRenderRealResults(t *testing.T) {
	status, headers, body := getPageAccept(t, "/airports/nearby?lat=40.6413&lon=-73.7781&radius=25&limit=1", "text/html")
	if status != http.StatusOK || !strings.Contains(headers.Get("Content-Type"), "text/html") {
		t.Fatalf("nearby search status=%d content-type=%q", status, headers.Get("Content-Type"))
	}
	if !strings.Contains(body, "KJFK") || !hasInputValue(body, "radius", "25") || !hasInputValue(body, "limit", "1") {
		t.Fatal("SSR nearby results do not show the nearest airport with the requested radius and limit")
	}
	if count := strings.Count(body, `class="airport-result-card"`); count != 1 {
		t.Fatalf("SSR nearby search rendered %d airports for limit=1, want 1", count)
	}
}

func TestSSR_SitemapAndFeedsAreValidAndHostScoped(t *testing.T) {
	for _, test := range []struct {
		path, contentType, marker string
	}{
		{path: "/sitemap.xml", contentType: "application/xml", marker: "<urlset"},
		{path: "/feed.xml", contentType: "application/atom+xml", marker: "<feed"},
		{path: "/rss.xml", contentType: "application/rss+xml", marker: "<rss"},
	} {
		t.Run(strings.Trim(test.path, "/"), func(t *testing.T) {
			status, headers, body := getPageAccept(t, test.path, "")
			if status != http.StatusOK || !strings.Contains(headers.Get("Content-Type"), test.contentType) || !strings.Contains(body, test.marker) {
				t.Fatalf("%s status=%d content-type=%q body=%q", test.path, status, headers.Get("Content-Type"), body)
			}
		})
	}
}

func TestAPI_AirportExportFormatsContainData(t *testing.T) {
	t.Run("json", func(t *testing.T) {
		status, headers, body := getPageAccept(t, "/api/v1/airports.json", "application/json")
		var airports map[string]json.RawMessage
		if status != http.StatusOK || !strings.Contains(headers.Get("Content-Type"), "application/json") {
			t.Fatalf("JSON export status=%d content-type=%q", status, headers.Get("Content-Type"))
		}
		if err := json.Unmarshal([]byte(body), &airports); err != nil || len(airports) == 0 {
			t.Fatalf("JSON export contains no airport data (count=%d, err=%v)", len(airports), err)
		}
	})
	t.Run("csv", func(t *testing.T) {
		status, headers, body := getPageAccept(t, "/api/v1/airports.csv", "text/csv")
		records, err := csv.NewReader(strings.NewReader(body)).ReadAll()
		if status != http.StatusOK || !strings.Contains(headers.Get("Content-Type"), "text/csv") {
			t.Fatalf("CSV export status=%d content-type=%q", status, headers.Get("Content-Type"))
		}
		if err != nil || len(records) < 2 {
			t.Fatalf("CSV export has no airport rows (records=%d, err=%v)", len(records), err)
		}
	})
	t.Run("geojson", func(t *testing.T) {
		status, headers, body := getPageAccept(t, "/api/v1/airports.geojson", "application/geo+json")
		var collection struct {
			Type     string            `json:"type"`
			Features []json.RawMessage `json:"features"`
		}
		if status != http.StatusOK || !strings.Contains(headers.Get("Content-Type"), "application/geo+json") {
			t.Fatalf("GeoJSON export status=%d content-type=%q", status, headers.Get("Content-Type"))
		}
		if err := json.Unmarshal([]byte(body), &collection); err != nil || collection.Type != "FeatureCollection" || len(collection.Features) == 0 {
			t.Fatalf("GeoJSON export contains no features (type=%q count=%d err=%v)", collection.Type, len(collection.Features), err)
		}
	})
}

func TestAPI_GraphQLReturnsAirportData(t *testing.T) {
	payload, err := json.Marshal(map[string]string{"query": `{ airport(code: "KJFK") { icao name } }`})
	if err != nil {
		t.Fatal(err)
	}
	request, err := http.NewRequest(http.MethodPost, baseURL(t)+"/api/graphql", strings.NewReader(string(payload)))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")
	response, err := (&http.Client{Timeout: 20 * time.Second}).Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusOK || !strings.Contains(string(body), `"icao":"KJFK"`) {
		t.Fatalf("GraphQL airport query status=%d body=%s", response.StatusCode, body)
	}
}

func assertAirportAPIResponses(t *testing.T, tier string) {
	t.Helper()
	paths := []struct {
		name, path, contentType, marker string
	}{
		{"airport list", "/api/v1/airports?limit=1", "application/json", `"data"`},
		{"airport detail", "/api/v1/airports/KJFK", "application/json", `"icao":"KJFK"`},
		{"search", "/api/v1/airports/search?q=JFK&limit=1", "application/json", `"KJFK"`},
		{"nearby", "/api/v1/airports/nearby?lat=40.6413&lon=-73.7781&radius=25&limit=1", "application/json", `"KJFK"`},
		{"bounding box", "/api/v1/airports/within?minLat=40&maxLat=41&minLon=-74&maxLon=-73", "application/json", `"KJFK"`},
		{"autocomplete", "/api/v1/airports/autocomplete?q=JFK", "application/json", `"KJFK"`},
		{"countries", "/api/v1/countries", "application/json", `"data"`},
		{"states", "/api/v1/states/US", "application/json", `"data"`},
		{"statistics", "/api/v1/stats", "application/json", `"data"`},
		{"JSON export", "/api/v1/airports.json", "application/json", `"KJFK"`},
		{"CSV export", "/api/v1/airports.csv", "text/csv", "KJFK"},
		{"GeoJSON export", "/api/v1/airports.geojson", "application/geo+json", `"FeatureCollection"`},
		{"safe settings", "/api/v1/settings", "application/json", `"data"`},
		{"GeoIP validation", "/api/v1/geoip/not-an-ip", "application/json", `"error"`},
	}
	for _, test := range paths {
		t.Run(test.name, func(t *testing.T) {
			var page string
			if tier == "SSR" {
				status, headers, body := getPageAccept(t, test.path, "application/json")
				if status < 200 || status >= 500 {
					t.Fatalf("API response %s returned HTTP %d", test.path, status)
				}
				if !strings.Contains(headers.Get("Content-Type"), test.contentType) {
					t.Fatalf("API response %s content type = %q, want %q", test.path, headers.Get("Content-Type"), test.contentType)
				}
				page = body
			} else {
				ctx := browserContext(t, tier == "no-JS")
				page = navigate(t, ctx, test.path)
				var bodyText string
				if err := chromedp.Run(ctx, chromedp.Text("body", &bodyText)); err == nil {
					page += bodyText
				}
			}
			if !strings.Contains(page, test.marker) {
				t.Fatalf("API response %s does not contain %q", test.path, test.marker)
			}
		})
	}
	query := url.Values{"query": {`{ airport(code: "KJFK") { icao name } }`}}
	t.Run("GraphQL GET", func(t *testing.T) {
		var page string
		path := "/api/graphql?" + query.Encode()
		if tier == "SSR" {
			status, headers, body := getPageAccept(t, path, "application/json")
			if status != http.StatusOK {
				t.Fatalf("GraphQL GET returned HTTP %d", status)
			}
			if !strings.Contains(headers.Get("Content-Type"), "application/json") {
				t.Fatalf("GraphQL GET content type = %q", headers.Get("Content-Type"))
			}
			page = body
		} else {
			ctx := browserContext(t, tier == "no-JS")
			page = navigate(t, ctx, path)
			var bodyText string
			if err := chromedp.Run(ctx, chromedp.Text("body", &bodyText)); err == nil {
				page += bodyText
			}
		}
		if !strings.Contains(page, `"icao":"KJFK"`) {
			t.Fatalf("GraphQL GET response lacks airport data: %s", page)
		}
	})
}

func TestSSR_AirportDataAPIEndpointsContainRealFeatureData(t *testing.T) {
	assertAirportAPIResponses(t, "SSR")
}

func TestNoJS_AirportDataAPIEndpointsRemainUsable(t *testing.T) {
	assertAirportAPIResponses(t, "no-JS")
}

func TestJS_AirportDataAPIEndpointsRemainUsable(t *testing.T) {
	assertAirportAPIResponses(t, "JS")
}

func TestSSR_ServerPagesAndAPIDocsRender(t *testing.T) {
	for _, route := range []string{"/server/about", "/server/help", "/server/privacy", "/server/terms", "/server/docs/swagger", "/server/docs/graphql"} {
		t.Run(strings.Trim(route, "/"), func(t *testing.T) {
			status, _, body := getPage(t, route)
			if status != http.StatusOK || len(strings.TrimSpace(body)) < 100 {
				t.Fatalf("%s status=%d rendered an empty or incomplete page", route, status)
			}
		})
	}
}

func TestSSR_OwnerTokenPageRendersIssueAndRestoreForms(t *testing.T) {
	status, _, body := getPage(t, "/server/preferences/tokens")
	if status != http.StatusOK || !strings.Contains(body, `action="/server/preferences/token/create"`) || !strings.Contains(body, `action="/server/preferences/token/restore"`) {
		t.Fatalf("owner-token SSR page status=%d lacks New token and Restore forms", status)
	}
	if !strings.Contains(body, `name="csrf_token"`) || !strings.Contains(body, `type="password"`) {
		t.Fatal("owner-token issue/restore forms lack CSRF protection or a concealed restore field")
	}
}

func TestSSR_UnknownAirportRendersNotFound(t *testing.T) {
	status, _, body := getPageAccept(t, "/airports/NOTAREALCODE", "text/html")
	if status != http.StatusNotFound || !strings.Contains(strings.ToLower(body), "not found") || !strings.Contains(body, "theme-") {
		t.Fatalf("unknown airport response status=%d does not render not-found content", status)
	}
}

func TestSSR_LanguageSwitchChangesRenderedStrings(t *testing.T) {
	_, _, english := getPage(t, "/")
	status, headers, spanish := getPage(t, "/?lang=es")
	if status != http.StatusOK || !strings.Contains(spanish, `lang="es"`) || !strings.Contains(spanish, "Buscar") {
		t.Fatal("Spanish language selection did not translate the rendered home page")
	}
	if strings.Contains(english, "Buscar por código") {
		t.Fatal("default English response unexpectedly contains Spanish search text")
	}
	if len(headers.Values("Set-Cookie")) == 0 {
		t.Error("language query did not persist the selected language cookie")
	}
}

func TestSSR_DarkLightAndAutoThemeClasses(t *testing.T) {
	for _, theme := range []string{"dark", "light", "auto"} {
		t.Run(theme, func(t *testing.T) {
			request, err := http.NewRequest(http.MethodGet, baseURL(t)+"/", nil)
			if err != nil {
				t.Fatal(err)
			}
			request.Header.Set("Accept", "text/html")
			request.AddCookie(&http.Cookie{Name: "theme", Value: theme})
			request.AddCookie(acceptedPreferencesCookie())
			response, err := (&http.Client{Timeout: 20 * time.Second}).Do(request)
			if err != nil {
				t.Fatal(err)
			}
			defer response.Body.Close()
			body, err := io.ReadAll(response.Body)
			if err != nil || response.StatusCode != http.StatusOK || !strings.Contains(string(body), `class="theme-`+theme+`"`) {
				t.Fatalf("theme %s not rendered (status %d, err %v)", theme, response.StatusCode, err)
			}
		})
	}
}

func TestSSR_ReferencedStaticAssetsExist(t *testing.T) {
	_, _, body := getPage(t, "/")
	assetPattern := regexp.MustCompile(`(?:src|href)="(/static/[^"?#]+)`)
	assets := assetPattern.FindAllStringSubmatch(body, -1)
	if len(assets) == 0 {
		t.Fatal("home page references no local static assets")
	}
	seen := make(map[string]bool)
	for _, match := range assets {
		asset := match[1]
		if seen[asset] {
			continue
		}
		seen[asset] = true
		status, _, _ := getPage(t, asset)
		if status != http.StatusOK {
			t.Errorf("static asset %s returned HTTP %d", asset, status)
		}
	}
}

func TestSSR_PWAHeadAndPublicPageCacheContract(t *testing.T) {
	status, headers, body := getPageAccept(t, "/", "text/html")
	if status != http.StatusOK || !strings.Contains(headers.Get("Content-Type"), "text/html") {
		t.Fatalf("home page returned status %d and content type %q", status, headers.Get("Content-Type"))
	}
	for _, marker := range []string{`rel="manifest"`, `name="theme-color"`, `name="viewport"`, `apple-mobile-web-app-capable`, `offline-indicator`} {
		if !strings.Contains(body, marker) {
			t.Errorf("SSR home page is missing PWA marker %q", marker)
		}
	}
	if headers.Get("X-Offline-Cache") != "public" || headers.Get("Set-Cookie") != "" {
		t.Fatalf("public home response is not safely cacheable: X-Offline-Cache=%q Set-Cookie=%q", headers.Get("X-Offline-Cache"), headers.Get("Set-Cookie"))
	}
	status, headers, _ = getPageAccept(t, "/server/preferences", "text/html")
	if status != http.StatusOK || headers.Get("X-Offline-Cache") == "public" {
		t.Fatalf("private preferences page cache classification: status=%d X-Offline-Cache=%q", status, headers.Get("X-Offline-Cache"))
	}
}

func TestSSR_FrontendJSONMatchesPairedAPIResponses(t *testing.T) {
	pairs := []struct{ page, api string }{
		{"/", "/api/v1/"},
		{"/airports", "/api/v1/airports"},
		{"/airports/search?q=JFK", "/api/v1/airports/search?q=JFK"},
		{"/airports/nearby?lat=40.6413&lon=-73.7781&radius=25&limit=1", "/api/v1/airports/nearby?lat=40.6413&lon=-73.7781&radius=25&limit=1"},
		{"/airports/KJFK", "/api/v1/airports/KJFK"},
		{"/stats", "/api/v1/stats"},
		{"/geoip?ip=8.8.8.8", "/api/v1/geoip?ip=8.8.8.8"},
		{"/server/about", "/api/v1/server/about"},
		{"/server/help", "/api/v1/server/help"},
		{"/server/help/mcp", "/api/v1/server/help/mcp"},
		{"/server/healthz", "/api/v1/server/healthz"},
		{"/healthz", "/api/v1/healthz"},
		{"/server/privacy", "/api/v1/server/privacy"},
		{"/server/preferences", "/api/v1/server/preferences"},
		{"/server/preferences/sync", "/api/v1/server/preferences/sync"},
		{"/server/preferences/sync/import", "/api/v1/server/preferences/sync/import"},
		{"/server/preferences/tokens", "/api/v1/server/preferences/tokens"},
		{"/server/dpo", "/api/v1/server/dpo"},
		{"/server/terms", "/api/v1/server/terms"},
		{"/server/contact", "/api/v1/server/contact"},
		{"/server/security", "/api/v1/server/security"},
		{"/server/security/policy", "/api/v1/server/security/policy"},
		{"/server/security/thanks", "/api/v1/server/security/thanks"},
		{"/server/docs/swagger", "/api/v1/server/swagger"},
		{"/server/docs/graphql", "/api/v1/server/graphql"},
	}
	// /server/security/report/{tracking_id} deliberately has no API mirror: it
	// is a one-shot emailed status page and its tracking ID is a bearer secret.
	for _, pair := range pairs {
		t.Run(strings.Trim(pair.page, "/"), func(t *testing.T) {
			pageStatus, pageHeaders, pageBody := getPageAccept(t, pair.page, "application/json")
			apiStatus, apiHeaders, apiBody := getPageAccept(t, pair.api, "application/json")
			if pageStatus != apiStatus || !strings.Contains(pageHeaders.Get("Content-Type"), "application/json") || pageBody != apiBody {
				t.Fatalf("frontend JSON does not match paired API byte-for-byte: page status/type=%d/%q API=%d/%q\npage: %s\nAPI: %s", pageStatus, pageHeaders.Get("Content-Type"), apiStatus, apiHeaders.Get("Content-Type"), pageBody, apiBody)
			}
		})
	}
}

func TestNoJS_PWAContentAndNavigationRemainAvailable(t *testing.T) {
	ctx := browserContext(t, true)
	page := navigate(t, ctx, "/")
	for _, marker := range []string{`rel="manifest"`, `name="theme-color"`, `offline-indicator`, `Search`, `Install on iPhone or iPad`, `Add to Home Screen`} {
		if !strings.Contains(page, marker) {
			t.Errorf("no-JS home page is missing %q", marker)
		}
	}
	var navCount int
	if err := chromedp.Run(ctx, chromedp.Evaluate(`document.querySelectorAll('nav a[href]').length`, &navCount)); err != nil {
		t.Fatal(err)
	}
	if navCount == 0 {
		t.Fatal("no-JS PWA page has no navigable application links")
	}
}

func TestJS_PWAOfflineRecoveryAndUpdateApproval(t *testing.T) {
	ctx := browserContext(t, false)
	page := navigate(t, ctx, "/")
	if !strings.Contains(page, `rel="manifest"`) {
		t.Fatal("JavaScript page is missing its install manifest link")
	}
	var registered bool
	if err := chromedp.Run(ctx, chromedp.Evaluate(`'serviceWorker' in navigator`, &registered)); err != nil || !registered {
		t.Fatalf("service workers unavailable for PWA: registered=%v err=%v", registered, err)
	}
	if err := chromedp.Run(ctx,
		network.Enable(),
		chromedp.Evaluate(`navigator.serviceWorker.ready.then(() => new Promise(resolve => { if (navigator.serviceWorker.controller) resolve(true); else navigator.serviceWorker.addEventListener('controllerchange', () => resolve(true), {once:true}); }))`, nil),
	); err != nil {
		t.Fatalf("wait for service worker control: %v", err)
	}
	if err := chromedp.Run(ctx, chromedp.Navigate(baseURL(t)+"/"), chromedp.WaitReady("body")); err != nil {
		t.Fatalf("prime the public page cache under service-worker control: %v", err)
	}
	if err := chromedp.Run(ctx, network.EmulateNetworkConditions(true, 0, 0, 0)); err != nil {
		t.Fatalf("disable browser network: %v", err)
	}
	var cachedHTML string
	if err := chromedp.Run(ctx, chromedp.Navigate(baseURL(t)+"/"), chromedp.OuterHTML("html", &cachedHTML)); err != nil {
		t.Fatalf("load public home page while offline: %v", err)
	}
	if !strings.Contains(cachedHTML, "airport-search") && !strings.Contains(cachedHTML, "/airports/search") {
		t.Fatal("offline public-page fallback did not return cached airport UI")
	}
	var apiUnavailable bool
	if err := chromedp.Run(ctx, chromedp.Evaluate(`fetch('/api/v1/airports/KJFK').then(() => false, () => true)`, &apiUnavailable)); err != nil || !apiUnavailable {
		t.Fatalf("API response was not network-only while offline: unavailable=%v err=%v", apiUnavailable, err)
	}
	var offlineVisible bool
	if err := chromedp.Run(ctx, chromedp.Evaluate(`navigator.onLine === false`, &offlineVisible)); err != nil || !offlineVisible {
		t.Fatalf("browser did not enter offline state: offline=%v err=%v", offlineVisible, err)
	}
	var statusBanner bool
	if err := chromedp.Run(ctx, chromedp.Evaluate(`!!document.querySelector('#connection-status-banner')`, &statusBanner)); err != nil || !statusBanner {
		t.Fatalf("offline transition did not show its status banner: visible=%v err=%v", statusBanner, err)
	}
	if err := chromedp.Run(ctx, network.EmulateNetworkConditions(false, 1000, 500000, 500000)); err != nil {
		t.Fatalf("restore browser network: %v", err)
	}
	if err := chromedp.Run(ctx, chromedp.Evaluate(`window.dispatchEvent(new Event('online'))`, nil)); err != nil {
		t.Fatal(err)
	}
	var cleared bool
	if err := chromedp.Run(ctx, chromedp.Evaluate(`!document.querySelector('#connection-status-banner')`, &cleared)); err != nil || !cleared {
		t.Fatalf("online recovery did not clear the status banner: cleared=%v err=%v", cleared, err)
	}
	if err := chromedp.Run(ctx, chromedp.Evaluate(`(() => {
		window.__skipWaitingMessage = null;
		const waiting = {postMessage(message) { window.__skipWaitingMessage = message; }};
		Object.defineProperty(navigator.serviceWorker, 'ready', {configurable:true, value:Promise.resolve({waiting})});
		showUpdateNotification();
		return true;
	})()`, nil)); err != nil {
		t.Fatalf("prepare waiting service-worker fixture: %v", err)
	}
	var initialMessage string
	if err := chromedp.Run(ctx, chromedp.Evaluate(`JSON.stringify(window.__skipWaitingMessage)`, &initialMessage)); err != nil || initialMessage != "null" {
		t.Fatalf("waiting worker was activated without approval: message=%q err=%v", initialMessage, err)
	}
	if err := chromedp.Run(ctx, chromedp.Click(`#app-update-banner .btn-primary`, chromedp.ByQuery), chromedp.Sleep(50*time.Millisecond)); err != nil {
		t.Fatalf("approve service-worker update: %v", err)
	}
	var skipWaitingMessage string
	if err := chromedp.Run(ctx, chromedp.Evaluate(`JSON.stringify(window.__skipWaitingMessage)`, &skipWaitingMessage)); err != nil || skipWaitingMessage != `{"type":"SKIP_WAITING"}` {
		t.Fatalf("explicit approval did not request worker handover: message=%q err=%v", skipWaitingMessage, err)
	}
}

func TestJS_PWAInstallInstructionsSupportDesktopModeIPad(t *testing.T) {
	ctx := browserContext(t, false)
	userAgent := "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.0 Safari/605.1.15"
	var instructionsVisible bool
	if err := chromedp.Run(ctx,
		emulation.SetUserAgentOverride(userAgent).WithPlatform("MacIntel"),
		emulation.SetTouchEmulationEnabled(true).WithMaxTouchPoints(5),
		chromedp.Navigate(baseURL(t)+"/"),
		chromedp.Evaluate(`navigator.platform === 'MacIntel' && navigator.maxTouchPoints > 1 && !document.querySelector('#pwa-ios-install-instructions').hidden`, &instructionsVisible),
	); err != nil || !instructionsVisible {
		t.Fatalf("desktop-mode iPad did not show Add to Home Screen instructions: visible=%v err=%v", instructionsVisible, err)
	}
}

func TestJS_PWAInstallPromptWaitsForUserChoice(t *testing.T) {
	ctx := browserContext(t, false)
	navigate(t, ctx, "/")
	var dispatched bool
	if err := chromedp.Run(ctx, chromedp.Evaluate(`(() => { const e = new Event('beforeinstallprompt', {cancelable:true}); e.prompt = () => {}; e.userChoice = Promise.resolve({outcome:'accepted'}); window.dispatchEvent(e); return e.defaultPrevented; })()`, &dispatched)); err != nil || !dispatched {
		t.Fatalf("install prompt event was not captured and deferred: prevented=%v err=%v", dispatched, err)
	}
	var visible bool
	if err := chromedp.Run(ctx, chromedp.Evaluate(`!document.querySelector('#pwa-install-button').hidden`, &visible)); err != nil || !visible {
		t.Fatalf("install button did not appear after browser event: visible=%v err=%v", visible, err)
	}
	if err := chromedp.Run(ctx, chromedp.Click(`#pwa-install-button`, chromedp.ByQuery), chromedp.Sleep(50*time.Millisecond)); err != nil {
		t.Fatalf("accept install prompt: %v", err)
	}
	var hidden bool
	if err := chromedp.Run(ctx, chromedp.Evaluate(`document.querySelector('#pwa-install-button').hidden`, &hidden)); err != nil || !hidden {
		t.Fatalf("install button remained visible after user choice: hidden=%v err=%v", hidden, err)
	}
}

func TestSSR_UnknownRouteUsesThemed404(t *testing.T) {
	status, headers, body := getPageAccept(t, "/this-route-does-not-exist", "text/html")
	if status != http.StatusNotFound || !strings.Contains(headers.Get("Content-Type"), "text/html") || !strings.Contains(body, "theme-dark") || len(strings.TrimSpace(body)) < 100 {
		t.Fatalf("unknown route is not a rendered themed 404 (status %d)", status)
	}
}

func TestSSR_Controlled500UsesThemedErrorTemplate(t *testing.T) {
	proxy := controlledThemed500Proxy(t)
	response, err := (&http.Client{Timeout: 20 * time.Second}).Get(proxy.URL + "/__e2e/controlled-500")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusInternalServerError || !strings.Contains(response.Header.Get("Content-Type"), "text/html") || !strings.Contains(string(body), "Internal Server Error") || !strings.Contains(string(body), `class="theme-`) {
		t.Fatalf("controlled 500 did not render themed server error HTML (status=%d content-type=%q)", response.StatusCode, response.Header.Get("Content-Type"))
	}
}

func TestSSR_InternalLinksCrawlWithoutServerErrors(t *testing.T) {
	linkPattern := regexp.MustCompile(`href="(/[^"#]*)`)
	queue := []string{"/"}
	visited := make(map[string]bool)
	for len(queue) > 0 && len(visited) < 100 {
		path := queue[0]
		queue = queue[1:]
		parsed, err := url.Parse(path)
		if err != nil {
			t.Errorf("invalid internal link %q: %v", path, err)
			continue
		}
		canonical := parsed.EscapedPath()
		if canonical == "" {
			canonical = "/"
		}
		if visited[canonical] || strings.HasPrefix(canonical, "/static/") || strings.HasPrefix(canonical, "/locales/") || strings.HasPrefix(canonical, "/api/") {
			continue
		}
		visited[canonical] = true
		status, _, body := getPageAccept(t, path, "text/html")
		if status >= http.StatusBadRequest {
			t.Errorf("internal route %s returned HTTP %d", path, status)
			continue
		}
		for _, link := range linkPattern.FindAllStringSubmatch(body, -1) {
			if !visited[link[1]] {
				queue = append(queue, link[1])
			}
		}
	}
	if len(visited) == 0 {
		t.Fatal("internal link crawl visited no pages")
	}
}

func browserContext(t *testing.T, noJS bool) context.Context {
	t.Helper()
	options := append(chromedp.DefaultExecAllocatorOptions[:], chromedp.NoSandbox, chromedp.DisableGPU)
	if browser := os.Getenv("CHROME_BIN"); browser != "" {
		options = append(options, chromedp.ExecPath(browser))
	}
	allocator, cancelAllocator := chromedp.NewExecAllocator(context.Background(), options...)
	ctx, cancelBrowser := chromedp.NewContext(allocator)
	ctx, cancelTimeout := context.WithTimeout(ctx, 45*time.Second)
	t.Cleanup(func() {
		if t.Failed() {
			if artifactDir := os.Getenv("E2E_ARTIFACT_DIR"); artifactDir != "" {
				name := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
				var screenshot []byte
				var html string
				if err := chromedp.Run(ctx, chromedp.CaptureScreenshot(&screenshot), chromedp.OuterHTML("html", &html)); err == nil {
					_ = os.WriteFile(filepath.Join(artifactDir, name+".png"), screenshot, 0o600)
					_ = os.WriteFile(filepath.Join(artifactDir, name+".html"), []byte(html), 0o600)
				}
			}
		}
		cancelTimeout()
		cancelBrowser()
		cancelAllocator()
	})
	if noJS {
		if err := chromedp.Run(ctx, emulation.SetScriptExecutionDisabled(true)); err != nil {
			t.Fatalf("disable JavaScript: %v", err)
		}
	}
	return ctx
}

func navigate(t *testing.T, ctx context.Context, path string) string {
	t.Helper()
	var html string
	if err := chromedp.Run(ctx, chromedp.Navigate(baseURL(t)+path), chromedp.OuterHTML("html", &html)); err != nil {
		t.Fatalf("navigate %s: %v", path, err)
	}
	return html
}

func navigateAt(t *testing.T, ctx context.Context, origin, path string) string {
	t.Helper()
	var html string
	if err := chromedp.Run(ctx, chromedp.Navigate(strings.TrimRight(origin, "/")+path), chromedp.OuterHTML("html", &html)); err != nil {
		t.Fatalf("navigate %s: %v", path, err)
	}
	return html
}

// secureCookieProxy gives the browser a loopback origin where Secure cookies
// work while forwarding requests to the isolated E2E server.
func secureCookieProxy(t *testing.T) *httptest.Server {
	t.Helper()
	target, err := url.Parse(baseURL(t))
	if err != nil {
		t.Fatalf("parse E2E server URL: %v", err)
	}
	proxy := httputil.NewSingleHostReverseProxy(target)
	director := proxy.Director
	proxy.Director = func(request *http.Request) {
		browserHost := request.Host
		director(request)
		request.Host = browserHost
	}
	server := httptest.NewServer(proxy)
	t.Cleanup(server.Close)
	return server
}

func controlledThemed500Proxy(t *testing.T) *httptest.Server {
	t.Helper()
	sourceOrigin := baseURL(t)
	status, headers, errorHTML := getPageAccept(t, "/this-route-does-not-exist", "text/html")
	if status != http.StatusNotFound || !strings.Contains(errorHTML, `class="theme-`) {
		t.Fatalf("server did not provide its themed error template fixture (status=%d)", status)
	}
	errorHTML = strings.ReplaceAll(errorHTML, "404", "500")
	errorHTML = strings.ReplaceAll(errorHTML, "not found", "Internal Server Error")
	target, err := url.Parse(baseURL(t))
	if err != nil {
		t.Fatalf("parse E2E server URL: %v", err)
	}
	proxy := httputil.NewSingleHostReverseProxy(target)
	director := proxy.Director
	proxy.Director = func(request *http.Request) {
		browserHost := request.Host
		director(request)
		request.Host = browserHost
	}
	var testServer *httptest.Server
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/__e2e/controlled-500" {
			w.Header().Set("Content-Type", headers.Get("Content-Type"))
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = io.WriteString(w, strings.ReplaceAll(errorHTML, sourceOrigin, testServer.URL))
			return
		}
		proxy.ServeHTTP(w, r)
	})
	testServer = httptest.NewServer(handler)
	t.Cleanup(testServer.Close)
	return testServer
}

func dismissCookieConsent(t *testing.T, ctx context.Context) {
	t.Helper()
	var visible bool
	if err := chromedp.Run(ctx, chromedp.Evaluate(`!!document.querySelector("#cookie-consent")`, &visible)); err != nil {
		t.Fatalf("check cookie consent banner: %v", err)
	}
	if !visible {
		return
	}
	if err := chromedp.Run(ctx,
		chromedp.Evaluate(`document.querySelector("#cookie-consent .btn-decline").click()`, nil),
		chromedp.WaitNotPresent("#cookie-consent", chromedp.ByQuery),
	); err != nil {
		t.Fatalf("dismiss cookie consent banner: %v", err)
	}
}

func stubMapTiles(t *testing.T, ctx context.Context) {
	t.Helper()
	chromedp.ListenTarget(ctx, func(event interface{}) {
		paused, ok := event.(*fetch.EventRequestPaused)
		if !ok || !strings.HasPrefix(paused.Request.URL, "https://tile.openstreetmap.org/") {
			return
		}
		go func() {
			_ = chromedp.Run(ctx, fetch.FulfillRequest(paused.RequestID, http.StatusOK).
				WithResponseHeaders([]*fetch.HeaderEntry{{Name: "Content-Type", Value: "image/png"}}).
				WithBody("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+aWuoAAAAASUVORK5CYII="))
		}()
	})
	if err := chromedp.Run(ctx, fetch.Enable().WithPatterns([]*fetch.RequestPattern{{URLPattern: "https://tile.openstreetmap.org/*"}})); err != nil {
		t.Fatalf("enable offline map tile stub: %v", err)
	}
}

func stubJSONResponse(t *testing.T, ctx context.Context, pattern string, status int64, response interface{}) <-chan int64 {
	t.Helper()
	responseStatuses := make(chan int64, 1)
	body, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("encode deterministic browser response: %v", err)
	}
	encodedBody := base64.StdEncoding.EncodeToString(body)
	chromedp.ListenTarget(ctx, func(event interface{}) {
		if response, ok := event.(*network.EventResponseReceived); ok && strings.Contains(response.Response.URL, pattern) {
			select {
			case responseStatuses <- int64(response.Response.Status):
			default:
			}
		}
		paused, ok := event.(*fetch.EventRequestPaused)
		if !ok || !strings.Contains(paused.Request.URL, pattern) {
			return
		}
		go func() {
			_ = chromedp.Run(ctx, fetch.FulfillRequest(paused.RequestID, status).
				WithResponseHeaders([]*fetch.HeaderEntry{{Name: "Content-Type", Value: "application/json"}}).
				WithBody(encodedBody))
		}()
	})
	if err := chromedp.Run(ctx, network.Enable()); err != nil {
		t.Fatalf("enable browser network events: %v", err)
	}
	urlPattern := strings.TrimRight(baseURL(t), "/") + pattern + "*"
	if err := chromedp.Run(ctx, fetch.Enable().WithPatterns([]*fetch.RequestPattern{{URLPattern: urlPattern}})); err != nil {
		t.Fatalf("enable deterministic response stub for %s: %v", pattern, err)
	}
	return responseStatuses
}

func TestJS_GeoIPLocationUsesDeterministicPositiveFixture(t *testing.T) {
	ctx := browserContext(t, false)
	responseStatuses := stubJSONResponse(t, ctx, "/api/v1/geoip/airports/nearby", http.StatusOK, map[string]interface{}{
		"ok": true,
		"data": map[string]interface{}{
			"location": map[string]interface{}{
				"ip": "203.0.113.5", "country": "US", "country_name": "United States",
				"city": "Queens", "latitude": 40.64, "longitude": -73.78,
			},
			"nearby_airports": []map[string]string{{"icao": "KJFK", "name": "John F Kennedy International"}},
		},
	})
	navigate(t, ctx, "/")
	dismissCookieConsent(t, ctx)
	buttonCtx, cancelButtonWait := context.WithTimeout(ctx, 5*time.Second)
	defer cancelButtonWait()
	if err := chromedp.Run(buttonCtx, chromedp.WaitVisible(`[data-action="detect-location"]`, chromedp.ByQuery)); err != nil {
		t.Fatalf("wait for the GeoIP location action after consent handling: %v", err)
	}
	if err := chromedp.Run(ctx,
		chromedp.Evaluate(`document.querySelector('[data-action="detect-location"]').click()`, nil),
	); err != nil {
		t.Fatalf("run positive GeoIP location flow: %v", err)
	}
	select {
	case status := <-responseStatuses:
		if status != http.StatusOK {
			t.Fatalf("GeoIP fixture response status = %d, want %d", status, http.StatusOK)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("browser did not request the deterministic positive GeoIP fixture")
	}
	waitCtx, cancelWait := context.WithTimeout(ctx, 5*time.Second)
	defer cancelWait()
	if err := chromedp.Run(waitCtx, chromedp.WaitVisible("#app-modal", chromedp.ByQuery)); err != nil {
		t.Fatalf("show positive GeoIP location modal: %v", err)
	}
	var modalText string
	if err := chromedp.Run(ctx, chromedp.Text("#app-modal", &modalText)); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"203.0.113.5", "Queens, United States", "40.6400, -73.7800", "Nearby Airports: 1"} {
		if !strings.Contains(modalText, want) {
			t.Errorf("positive GeoIP modal missing %q: %s", want, modalText)
		}
	}
}

func TestJS_GeoIPControlledHTTP500ShowsLookupError(t *testing.T) {
	ctx := browserContext(t, false)
	responseStatuses := stubJSONResponse(t, ctx, "/api/v1/geoip/203.0.113.5", http.StatusInternalServerError, map[string]interface{}{
		"ok": false, "error": "SERVER_ERROR", "message": "Controlled GeoIP failure", "details": map[string]interface{}{},
	})
	navigate(t, ctx, "/geoip")
	if err := chromedp.Run(ctx, chromedp.Evaluate(`document.getElementById("ip-input").value = "203.0.113.5"; lookupIP({preventDefault: function(){}}); true`, nil)); err != nil {
		t.Fatalf("start GeoIP lookup that returns HTTP 500: %v", err)
	}
	waitCtx, cancelWait := context.WithTimeout(ctx, 5*time.Second)
	defer cancelWait()
	if err := chromedp.Run(waitCtx, chromedp.WaitVisible("#geoip-error", chromedp.ByQuery)); err != nil {
		t.Fatalf("submit GeoIP lookup that returns HTTP 500: %v", err)
	}
	var message string
	if err := chromedp.Run(ctx, chromedp.Text("#error-message", &message)); err != nil {
		t.Fatal(err)
	}
	select {
	case status := <-responseStatuses:
		if status != http.StatusInternalServerError {
			t.Fatalf("GeoIP error response status = %d, want %d", status, http.StatusInternalServerError)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("browser did not receive the controlled GeoIP HTTP 500 response")
	}
	if !strings.Contains(message, "Controlled GeoIP failure") {
		t.Fatalf("controlled HTTP 500 was not shown in the lookup error state: %q", message)
	}
}

func TestNoJS_SearchUsesNativeGETForm(t *testing.T) {
	ctx := browserContext(t, true)
	page := navigate(t, ctx, "/airports/search?q=JFK")
	if !strings.Contains(page, "JFK") || !strings.Contains(page, `action="/airports/search"`) || !strings.Contains(page, `method="get"`) {
		t.Fatal("no-JS search page lacks rendered JFK results or native GET form")
	}
}

func TestNoJS_AirportDirectoryKeepsPaginationAndDetailsNavigable(t *testing.T) {
	ctx := browserContext(t, true)
	page := navigate(t, ctx, "/airports")
	if !strings.Contains(page, "airport-card") || !strings.Contains(page, `aria-label="Airports"`) || !strings.Contains(page, "/airports?page=2") {
		t.Fatal("no-JS airport directory lacks airport cards or working pagination")
	}
}

func TestNoJS_GeoIPFormUsesGETAndRendersInvalidInputError(t *testing.T) {
	ctx := browserContext(t, true)
	page := navigate(t, ctx, "/geoip?ip=not-an-ip")
	if !strings.Contains(page, `id="geoip-form"`) || !strings.Contains(page, `method="get"`) || !strings.Contains(page, `action="/geoip"`) {
		t.Fatal("no-JS GeoIP lookup lacks its native GET form")
	}
	if !strings.Contains(page, `id="geoip-error" class="error-container"`) {
		t.Fatal("no-JS GeoIP lookup did not render an invalid-input error")
	}
}

func TestNoJS_ContactFormRendersServerValidationError(t *testing.T) {
	ctx := browserContext(t, true)
	page := navigate(t, ctx, "/server/contact?type=privacy")
	if !strings.Contains(page, `name="csrf_token"`) || !strings.Contains(page, `name="captcha_id"`) || !strings.Contains(page, `id="contact-captcha"`) {
		t.Fatal("no-JS contact form lacks CSRF or its server-issued CAPTCHA challenge")
	}

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Jar: jar, Timeout: 20 * time.Second}
	pageURL := baseURL(t) + "/server/contact?type=privacy"
	getRequest, err := http.NewRequest(http.MethodGet, pageURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	getRequest.Header.Set("Accept", "text/html")
	pageResponse, err := client.Do(getRequest)
	if err != nil {
		t.Fatal(err)
	}
	pageBody, err := io.ReadAll(pageResponse.Body)
	if closeErr := pageResponse.Body.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		t.Fatal(err)
	}
	fieldValue := func(name string) string {
		t.Helper()
		pattern := regexp.MustCompile(`name="` + regexp.QuoteMeta(name) + `" value="([^"]*)"`)
		match := pattern.FindSubmatch(pageBody)
		if len(match) != 2 {
			t.Fatalf("server-rendered contact form lacks hidden field %q", name)
		}
		return string(match[1])
	}
	form := url.Values{
		"csrf_token":     {fieldValue("csrf_token")},
		"form_token":     {fieldValue("form_token")},
		"captcha_id":     {fieldValue("captcha_id")},
		"captcha_answer": {"incorrect"},
		"type":           {"privacy"},
		"step":           {"submit"},
		"name":           {"E2E Visitor"},
		"email":          {"e2e@example.invalid"},
		"request_kind":   {"access"},
		"subject":        {"Privacy request"},
		"message":        {"Testing the server-rendered validation response."},
	}
	time.Sleep(3 * time.Second)
	postRequest, err := http.NewRequest(http.MethodPost, baseURL(t)+"/server/contact", strings.NewReader(form.Encode()))
	if err != nil {
		t.Fatal(err)
	}
	postRequest.Header.Set("Accept", "text/html")
	postRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	postRequest.Header.Set("Origin", baseURL(t))
	postRequest.Header.Set("Referer", pageURL)
	response, err := client.Do(postRequest)
	if err != nil {
		t.Fatal(err)
	}
	responseBody, err := io.ReadAll(response.Body)
	if closeErr := response.Body.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusBadRequest || !strings.Contains(strings.ToLower(string(responseBody)), "captcha") || !strings.Contains(strings.ToLower(string(responseBody)), "invalid") {
		t.Fatalf("invalid CAPTCHA response status=%d did not produce the expected server validation error: %s", response.StatusCode, responseBody)
	}
}

func TestNoJS_AirportDetailKeepsStaticMapLink(t *testing.T) {
	ctx := browserContext(t, true)
	page := navigate(t, ctx, "/airports/KJFK")
	if !strings.Contains(page, "KJFK") || !strings.Contains(page, "openstreetmap.org") {
		t.Fatal("no-JS airport detail lacks airport data or static OpenStreetMap link")
	}
}

func TestNoJS_HealthPageRendersStatusSections(t *testing.T) {
	ctx := browserContext(t, true)
	page := navigate(t, ctx, "/server/healthz")
	for _, text := range []string{"System Status", "Component Status", "Server Statistics"} {
		if !strings.Contains(page, text) {
			t.Errorf("no-JS health page is missing %q", text)
		}
	}
}

func TestNoJS_NearbyCoordinatesRenderBaseline(t *testing.T) {
	ctx := browserContext(t, true)
	page := navigate(t, ctx, "/airports/nearby?lat=40.6413&lon=-73.7781&radius=25")
	if !strings.Contains(page, "Nearby") && !strings.Contains(page, "JFK") {
		t.Fatal("no-JS nearby page lacks rendered nearby-airport content")
	}
}

func TestNoJS_NearbyRadiusAndNearestLimitKeepNativeFormAndResults(t *testing.T) {
	ctx := browserContext(t, true)
	page := navigate(t, ctx, "/airports/nearby?lat=40.6413&lon=-73.7781&radius=25&limit=1")
	if !strings.Contains(page, "KJFK") || !hasInputValue(page, "radius", "25") || !hasInputValue(page, "limit", "1") {
		t.Fatal("no-JS nearby search lacks its nearest result or selected radius/limit")
	}
	if count := strings.Count(page, `class="airport-result-card"`); count != 1 {
		t.Fatalf("no-JS nearby search rendered %d airports for limit=1, want 1", count)
	}
	if !strings.Contains(page, `method="get"`) || !strings.Contains(page, `action="/airports/nearby"`) {
		t.Fatal("no-JS nearby search does not expose a native GET form")
	}
}

func TestNoJS_BoundingBoxSearchRendersCoordinatesAndStaticMapLink(t *testing.T) {
	ctx := browserContext(t, true)
	page := navigate(t, ctx, "/airports/nearby?minLat=40&maxLat=41&minLon=-74&maxLon=-73")
	if !strings.Contains(page, "airport-map") || !strings.Contains(page, "openstreetmap.org") {
		t.Fatal("no-JS bounding-box search lacks its map baseline or static map link")
	}
}

func TestNoJS_ThemeFallbackContainsCSRFProtectedForm(t *testing.T) {
	ctx := browserContext(t, true)
	page := navigate(t, ctx, "/")
	if !strings.Contains(page, `action="/server/preferences"`) || !strings.Contains(page, `name="csrf_token"`) || !strings.Contains(page, `name="theme"`) {
		t.Fatal("no-JS theme fallback lacks its POST form or CSRF token")
	}
}

func TestNoJS_MobileNavigationOpensAndClosesWithCSS(t *testing.T) {
	ctx := browserContext(t, true)
	if err := chromedp.Run(ctx, emulation.SetDeviceMetricsOverride(375, 812, 1, false)); err != nil {
		t.Fatal(err)
	}
	navigate(t, ctx, "/")
	if err := chromedp.Run(ctx, chromedp.Click(".nav-toggle"), chromedp.WaitVisible("#nav-panel", chromedp.ByQuery)); err != nil {
		t.Fatalf("open no-JS mobile navigation: %v", err)
	}
	var links string
	if err := chromedp.Run(ctx, chromedp.Evaluate(`Array.from(document.querySelectorAll('#nav-panel a')).map((link) => link.getAttribute('href')).join(' ')`, &links)); err != nil {
		t.Fatal(err)
	}
	for _, href := range []string{"/", "/airports", "/airports/search", "/airports/nearby", "/stats", "/geoip"} {
		if !strings.Contains(" "+links+" ", " "+href+" ") {
			t.Errorf("no-JS mobile navigation is missing %s: %s", href, links)
		}
	}
	if err := chromedp.Run(ctx, chromedp.MouseClickXY(20, 400)); err != nil {
		t.Fatalf("close no-JS mobile navigation with scrim: %v", err)
	}
	var isClosed bool
	if err := chromedp.Run(ctx, chromedp.Evaluate(`!document.getElementById('nav-toggle').checked`, &isClosed)); err != nil || !isClosed {
		t.Fatalf("no-JS scrim did not close navigation (closed=%t, err=%v)", isClosed, err)
	}
}

func TestNoJS_ServerDocsRemainNavigable(t *testing.T) {
	ctx := browserContext(t, true)
	page := navigate(t, ctx, "/server/docs/swagger")
	if len(strings.TrimSpace(page)) < 100 {
		t.Fatal("no-JS Swagger reference is blank")
	}
}

func TestJS_SearchAndAirportNavigation(t *testing.T) {
	ctx := browserContext(t, false)
	stubMapTiles(t, ctx)
	page := navigate(t, ctx, "/airports/search?q=JFK")
	if !strings.Contains(page, "JFK") {
		t.Fatal("JS search page lacks JFK result")
	}
	var location string
	if err := chromedp.Run(ctx, chromedp.Evaluate(`document.querySelector('.airport-card')?.getAttribute('data-code')`, &location)); err != nil {
		t.Fatal(err)
	}
	if location == "" {
		t.Fatal("search result does not expose an airport code for browser navigation")
	}
	if err := chromedp.Run(ctx, chromedp.Navigate(baseURL(t)+"/airports/"+url.PathEscape(location)), chromedp.WaitReady("body")); err != nil {
		t.Fatalf("open airport detail: %v", err)
	}
	var body string
	if err := chromedp.Run(ctx, chromedp.Text("body", &body)); err != nil || !strings.Contains(body, location) {
		t.Fatalf("airport detail does not show %s (body %q, err %v)", location, body, err)
	}
}

func TestJS_AirportDirectoryPaginationAndDetailNavigation(t *testing.T) {
	ctx := browserContext(t, false)
	page := navigate(t, ctx, "/airports")
	if !strings.Contains(page, "airport-card") {
		t.Fatal("JavaScript airport directory lacks airport cards")
	}
	dismissCookieConsent(t, ctx)
	if err := chromedp.Run(ctx,
		chromedp.Click(`.pagination a[href="/airports?page=2"]`, chromedp.ByQuery),
		chromedp.WaitVisible(`.pagination a[href="/airports?page=1"]`, chromedp.ByQuery),
	); err != nil {
		t.Fatalf("wait for the next airport directory page: %v", err)
	}
	var pageURL, detailURL string
	if err := chromedp.Run(ctx,
		chromedp.Location(&pageURL),
		chromedp.Evaluate(`document.querySelector('.airport-card a').getAttribute('href')`, &detailURL),
	); err != nil {
		t.Fatal(err)
	}
	parsedPageURL, parseErr := url.Parse(pageURL)
	if parseErr != nil || parsedPageURL.Path != "/airports" || parsedPageURL.Query().Get("page") != "2" || !strings.HasPrefix(detailURL, "/airports/") {
		t.Fatalf("airport pagination/detail links are incorrect: page=%q detail=%q", pageURL, detailURL)
	}
}

func TestJS_NearbySearchAddsInteractiveMapMarkers(t *testing.T) {
	ctx := browserContext(t, false)
	stubMapTiles(t, ctx)
	page := navigate(t, ctx, "/airports/nearby?lat=40.6413&lon=-73.7781&radius=25")
	if !strings.Contains(page, "airport-map") {
		t.Fatal("nearby SSR response lacks its map baseline")
	}
	var mapVisible, hasMarkers bool
	if err := chromedp.Run(ctx,
		chromedp.Evaluate(`!document.querySelector('.airport-map').hidden`, &mapVisible),
		chromedp.Evaluate(`document.querySelectorAll('.map-marker').length > 0`, &hasMarkers),
	); err != nil {
		t.Fatal(err)
	}
	if !mapVisible || !hasMarkers {
		t.Fatal("JavaScript did not upgrade the nearby results to an interactive map with airport markers")
	}
	if err := chromedp.Run(ctx, chromedp.Click(".map-marker"), chromedp.WaitReady("body")); err != nil {
		t.Fatalf("open airport from map marker: %v", err)
	}
	var body string
	if err := chromedp.Run(ctx, chromedp.Text("body", &body)); err != nil || !strings.Contains(body, "KJFK") {
		t.Fatalf("map marker did not open the KJFK detail page (err %v)", err)
	}
}

func TestJS_NearbyRadiusAndNearestLimitMapFlow(t *testing.T) {
	ctx := browserContext(t, false)
	stubMapTiles(t, ctx)
	page := navigate(t, ctx, "/airports/nearby?lat=40.6413&lon=-73.7781&radius=25&limit=1")
	if !strings.Contains(page, "KJFK") || !hasInputValue(page, "radius", "25") || !hasInputValue(page, "limit", "1") {
		t.Fatal("JavaScript nearby page lacks its nearest result or requested radius/limit")
	}
	var markers int
	if err := chromedp.Run(ctx,
		chromedp.Evaluate(`document.querySelectorAll('.map-marker').length`, &markers),
	); err != nil {
		t.Fatal(err)
	}
	if markers != 1 {
		t.Fatalf("interactive map has %d markers for limit=1, want 1", markers)
	}
	if err := chromedp.Run(ctx, chromedp.Click(".map-marker"), chromedp.WaitReady("body")); err != nil {
		t.Fatalf("open nearest airport from the map: %v", err)
	}
	var body string
	if err := chromedp.Run(ctx, chromedp.Text("body", &body)); err != nil || !strings.Contains(body, "KJFK") {
		t.Fatalf("nearest map marker did not open KJFK detail page (err %v)", err)
	}
}

func TestJS_ResponsiveLayoutAndConsole(t *testing.T) {
	ctx := browserContext(t, false)
	var consoleErrors []string
	var networkFailures []string
	var eventsMu sync.Mutex
	chromedp.ListenTarget(ctx, func(event interface{}) {
		if failure, ok := event.(*network.EventLoadingFailed); ok {
			if failure.Type == network.ResourceTypeStylesheet || failure.Type == network.ResourceTypeScript || failure.Type == network.ResourceTypeImage || failure.Type == network.ResourceTypeXHR || failure.Type == network.ResourceTypeFetch {
				eventsMu.Lock()
				networkFailures = append(networkFailures, string(failure.Type)+": "+failure.ErrorText)
				eventsMu.Unlock()
			}
		}
		if exception, ok := event.(*runtime.EventExceptionThrown); ok {
			eventsMu.Lock()
			consoleErrors = append(consoleErrors, exception.ExceptionDetails.Text)
			eventsMu.Unlock()
		}
		if entry, ok := event.(*log.EventEntryAdded); ok && entry.Entry.Level == log.LevelError {
			eventsMu.Lock()
			consoleErrors = append(consoleErrors, entry.Entry.Text)
			eventsMu.Unlock()
		}
	})
	if err := chromedp.Run(ctx, network.Enable(), log.Enable()); err != nil {
		t.Fatal(err)
	}
	if err := chromedp.Run(ctx, emulation.SetDeviceMetricsOverride(375, 812, 1, false)); err != nil {
		t.Fatal(err)
	}
	page := navigate(t, ctx, "/")
	if !strings.Contains(page, "airports/search") {
		t.Fatal("mobile home page lacks search navigation")
	}
	var overflow bool
	if err := chromedp.Run(ctx, chromedp.Evaluate(`document.documentElement.scrollWidth > window.innerWidth`, &overflow)); err != nil {
		t.Fatal(err)
	}
	if err := chromedp.Run(ctx, chromedp.Click(".nav-toggle")); err != nil {
		t.Fatalf("open mobile navigation: %v", err)
	}
	var searchNavigationVisible bool
	if err := chromedp.Run(ctx, chromedp.Evaluate(`(() => { const link = document.querySelector('#nav-panel a[href="/airports/search"]'); return !!link && link.getBoundingClientRect().width > 0 && link.getBoundingClientRect().height > 0; })()`, &searchNavigationVisible)); err != nil {
		t.Fatal(err)
	}
	if overflow {
		var dimensions string
		_ = chromedp.Run(ctx, chromedp.Evaluate(`(() => JSON.stringify({innerWidth: window.innerWidth, clientWidth: document.documentElement.clientWidth, scrollWidth: document.documentElement.scrollWidth, offenders: Array.from(document.querySelectorAll('body *')).map((element) => ({tag: element.tagName, id: element.id, className: typeof element.className === 'string' ? element.className : '', left: Math.round(element.getBoundingClientRect().left), right: Math.round(element.getBoundingClientRect().right)})).filter((element) => element.left < 0 || element.right > window.innerWidth).slice(0, 12)}))()`, &dimensions))
		t.Fatalf("home page has horizontal overflow at 375x812: %s", dimensions)
	}
	if !searchNavigationVisible {
		t.Fatal("search navigation is not visible at 375x812")
	}
	eventsMu.Lock()
	defer eventsMu.Unlock()
	if len(consoleErrors) != 0 {
		t.Fatalf("JavaScript runtime errors: %v", consoleErrors)
	}
	if len(networkFailures) != 0 {
		t.Fatalf("failed browser asset/XHR requests: %v", networkFailures)
	}
}

func TestJS_ThemeChangesComputedColors(t *testing.T) {
	ctx := browserContext(t, false)
	readBackground := func(theme string) string {
		t.Helper()
		if err := chromedp.Run(ctx, network.SetCookie("cookie_consent", acceptedPreferencesCookie().Value).WithURL(baseURL(t)).WithPath("/"), network.SetCookie("theme", theme).WithURL(baseURL(t)).WithPath("/"), chromedp.Navigate(baseURL(t)+"/")); err != nil {
			t.Fatalf("load %s theme: %v", theme, err)
		}
		var background string
		if err := chromedp.Run(ctx, chromedp.Evaluate(`getComputedStyle(document.documentElement).getPropertyValue('--color-bg').trim()`, &background)); err != nil {
			t.Fatal(err)
		}
		if background == "" {
			t.Fatalf("%s theme has no computed background color", theme)
		}
		return background
	}
	dark := readBackground("dark")
	light := readBackground("light")
	if dark == light {
		t.Fatalf("dark and light themes compute the same background %q", dark)
	}
}

func runOwnerTokenBrowserFlow(t *testing.T, noJS bool) {
	t.Helper()
	proxy := secureCookieProxy(t)
	ctx := browserContext(t, noJS)
	navigateAt(t, ctx, proxy.URL, "/server/preferences/tokens")
	dismissCookieConsent(t, ctx)
	if err := chromedp.Run(ctx,
		chromedp.Click(`form[action="/server/preferences/token/create"] button[type="submit"]`, chromedp.ByQuery),
		chromedp.WaitVisible(`#primary .copy-value`, chromedp.ByQuery),
	); err != nil {
		t.Fatalf("issue a primary owner token through the browser form: %v", err)
	}
	var primary string
	if err := chromedp.Run(ctx, chromedp.Text(`#primary .copy-value`, &primary)); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(primary, "own_") || strings.Contains(primary, "*") {
		t.Fatalf("new primary token was not revealed on the redirect GET: %q", primary)
	}
	if err := chromedp.Run(ctx, chromedp.Reload(), chromedp.WaitReady("body")); err != nil {
		t.Fatalf("reload owner-token page: %v", err)
	}
	var masked string
	if err := chromedp.Run(ctx, chromedp.Text(`#primary .copy-value`, &masked)); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(masked, primary) || !strings.Contains(masked, "•") {
		t.Fatalf("primary token was not masked after reload: %q", masked)
	}
	if err := chromedp.Run(ctx,
		chromedp.Submit(`form[action="/server/preferences/token/reveal"]`, chromedp.ByQuery),
		chromedp.WaitReady("body"),
	); err != nil {
		t.Fatalf("reveal primary token through the browser form: %v", err)
	}
	var revealed string
	if err := chromedp.Run(ctx, chromedp.Text(`#primary .copy-value`, &revealed)); err != nil || revealed != primary {
		t.Fatalf("one-use primary reveal = %q, want original token (err %v)", revealed, err)
	}
	if err := chromedp.Run(ctx,
		chromedp.SendKeys(`#owner-token-name`, "E2E MCP client"),
		chromedp.Click(`form[action="/server/preferences/tokens"] button[type="submit"]`, chromedp.ByQuery),
		chromedp.WaitVisible(`.owner-linked-token .copy-value`, chromedp.ByQuery),
	); err != nil {
		t.Fatalf("create linked owner token through the browser form: %v", err)
	}
	var linked string
	if err := chromedp.Run(ctx, chromedp.Text(`.owner-linked-token .copy-value`, &linked)); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(linked, "own_") || strings.Contains(linked, "*") {
		t.Fatalf("new linked token was not revealed on the redirect GET: %q", linked)
	}
	if err := chromedp.Run(ctx, chromedp.Reload(), chromedp.WaitReady("body")); err != nil {
		t.Fatalf("reload linked-token page: %v", err)
	}
	if err := chromedp.Run(ctx, chromedp.Text(`.owner-linked-token .copy-value`, &masked)); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(masked, linked) || !strings.Contains(masked, "•") {
		t.Fatalf("linked token was not masked after reload: %q", masked)
	}
	if err := chromedp.Run(ctx,
		chromedp.Click(`button[popovertarget="owner-token-forget-confirm"]`, chromedp.ByQuery),
		chromedp.Click(`form[action="/server/preferences/token/forget"] button[type="submit"]`, chromedp.ByQuery),
		chromedp.WaitVisible(`form[action="/server/preferences/token/restore"]`, chromedp.ByQuery),
	); err != nil {
		t.Fatalf("forget the browser token before restoring it: %v", err)
	}
	dismissCookieConsent(t, ctx)
	if err := chromedp.Run(ctx,
		chromedp.SendKeys(`#owner-token-restore`, primary),
		chromedp.Click(`form[action="/server/preferences/token/restore"] button[type="submit"]`, chromedp.ByQuery),
		chromedp.WaitVisible(`#primary .copy-value`, chromedp.ByQuery),
	); err != nil {
		t.Fatalf("restore the primary token by pasting it into the browser form: %v", err)
	}
	if err := chromedp.Run(ctx, chromedp.Text(`#primary .copy-value`, &masked)); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(masked, primary) || !strings.Contains(masked, "•") {
		t.Fatalf("restored primary token was not masked by default: %q", masked)
	}
}

func TestNoJS_OwnerTokenIssueRevealMaskAndLinkedCreate(t *testing.T) {
	runOwnerTokenBrowserFlow(t, true)
}

func TestNoJS_Controlled500UsesThemedErrorTemplate(t *testing.T) {
	proxy := controlledThemed500Proxy(t)
	ctx := browserContext(t, true)
	page := navigateAt(t, ctx, proxy.URL, "/__e2e/controlled-500")
	if !strings.Contains(page, "500") || !strings.Contains(page, "Internal Server Error") || !strings.Contains(page, `class="theme-`) || strings.Contains(strings.ToLower(page), "panic:") {
		t.Fatal("no-JS controlled HTTP 500 did not render the themed error page")
	}
}

func TestJS_OwnerTokenIssueRevealMaskAndLinkedCreate(t *testing.T) {
	runOwnerTokenBrowserFlow(t, false)
}

func TestJS_Controlled500UsesThemedErrorTemplate(t *testing.T) {
	proxy := controlledThemed500Proxy(t)
	ctx := browserContext(t, false)
	page := navigateAt(t, ctx, proxy.URL, "/__e2e/controlled-500")
	if !strings.Contains(page, "500") || !strings.Contains(page, "Internal Server Error") || !strings.Contains(page, `class="theme-`) || strings.Contains(strings.ToLower(page), "panic:") {
		t.Fatal("JavaScript browser controlled HTTP 500 did not render the themed error page")
	}
}

func runConsentPreferenceSave(t *testing.T, noJS bool) {
	t.Helper()
	ctx := browserContext(t, noJS)
	proxy := secureCookieProxy(t)
	navigateAt(t, ctx, proxy.URL, "/server/privacy")
	var requestSeen bool
	var responseStatus int64
	var requestMu sync.Mutex
	chromedp.ListenTarget(ctx, func(event interface{}) {
		requestMu.Lock()
		defer requestMu.Unlock()
		switch event := event.(type) {
		case *network.EventRequestWillBeSent:
			if strings.HasSuffix(event.Request.URL, "/server/consent") {
				requestSeen = true
			}
		case *network.EventResponseReceived:
			if strings.HasSuffix(event.Response.URL, "/server/consent") {
				responseStatus = event.Response.Status
			}
		}
	})
	preferenceCtx, cancelPreference := context.WithTimeout(ctx, 10*time.Second)
	err := chromedp.Run(preferenceCtx,
		chromedp.WaitVisible(`#cookie-consent .cookie-preferences-form`, chromedp.ByQuery),
		chromedp.Evaluate(`document.querySelector('#cookie-consent input[name="preferences"]').checked = true; document.querySelector('#cookie-consent input[name="analytics"]').checked = false`, nil),
		chromedp.Submit(`#cookie-consent .cookie-preferences-form`, chromedp.ByQuery),
		chromedp.WaitNotPresent(`#cookie-consent`, chromedp.ByQuery),
	)
	cancelPreference()
	if err != nil {
		var diagnostic string
		_ = chromedp.Run(ctx, chromedp.Evaluate(`JSON.stringify({url: location.href, banner: !!document.querySelector('#cookie-consent'), formAction: document.querySelector('#cookie-consent .cookie-preferences-form')?.action || '', preferences: document.querySelector('#cookie-consent input[name="preferences"]')?.checked, analytics: document.querySelector('#cookie-consent input[name="analytics"]')?.checked, buttonDisabled: document.querySelector('#cookie-consent .cookie-preferences-form button[type="submit"]')?.disabled, body: document.body.innerText})`, &diagnostic))
		requestMu.Lock()
		seen, status := requestSeen, responseStatus
		requestMu.Unlock()
		t.Fatalf("save granular cookie preferences: %v (request_seen=%t response_status=%d page %s)", err, seen, status, diagnostic)
	}
	var raw string
	if err := chromedp.Run(ctx, chromedp.Evaluate(`(() => { const item = document.cookie.split('; ').find((entry) => entry.startsWith('cookie_consent=')); return item ? decodeURIComponent(item.slice('cookie_consent='.length)) : ''; })()`, &raw)); err != nil {
		t.Fatal(err)
	}
	var state struct {
		Essential   bool  `json:"essential"`
		Preferences bool  `json:"preferences"`
		Analytics   bool  `json:"analytics"`
		Timestamp   int64 `json:"timestamp"`
	}
	if err := json.Unmarshal([]byte(raw), &state); err != nil {
		t.Fatalf("parse saved cookie-consent state %q: %v", raw, err)
	}
	if !state.Essential || !state.Preferences || state.Analytics || state.Timestamp <= 0 {
		t.Fatalf("saved cookie-consent state = %+v; want essential/preferences true, analytics false, timestamp set", state)
	}
}

func TestNoJS_ConsentPreferencesSaveAndPersist(t *testing.T) {
	runConsentPreferenceSave(t, true)
}

func TestJS_ConsentPreferencesSaveAndPersist(t *testing.T) {
	runConsentPreferenceSave(t, false)
}

func runSuccessfulSecurityContact(t *testing.T, noJS bool) {
	t.Helper()
	proxy := secureCookieProxy(t)
	ctx := browserContext(t, noJS)
	navigateAt(t, ctx, proxy.URL, "/server/security")
	dismissCookieConsent(t, ctx)
	var reportURL string
	if err := chromedp.Run(ctx, chromedp.Evaluate(`document.querySelector('a[href*="/server/contact?"][href*="security_id="]')?.getAttribute('href') || ''`, &reportURL)); err != nil {
		t.Fatal(err)
	}
	if reportURL == "" {
		t.Fatal("security page did not provide its signed contact-report link")
	}
	parsedReportURL, err := url.Parse(reportURL)
	if err != nil {
		t.Fatalf("parse signed security-report URL %q: %v", reportURL, err)
	}
	query := parsedReportURL.Query()
	query.Set("type", "security")
	parsedReportURL.RawQuery = query.Encode()
	navigateAt(t, ctx, proxy.URL, parsedReportURL.RequestURI())
	var question string
	if err := chromedp.Run(ctx,
		chromedp.WaitVisible(`form[data-captcha="simple"]`, chromedp.ByQuery),
		chromedp.Text(`#contact-captcha-question`, &question),
	); err != nil {
		t.Fatalf("load simple CAPTCHA security-report form: %v", err)
	}
	parts := regexp.MustCompile(`(\d+)\s*([+-])\s*(\d+)`).FindStringSubmatch(question)
	if len(parts) != 4 {
		t.Fatalf("unrecognized simple CAPTCHA question %q", question)
	}
	left, _ := strconv.Atoi(parts[1])
	right, _ := strconv.Atoi(parts[3])
	answer := left + right
	if parts[2] == "-" {
		answer = left - right
	}
	if err := chromedp.Run(ctx,
		chromedp.SendKeys(`input[name="name"]`, "E2E Security Reporter"),
		chromedp.SendKeys(`input[name="email"]`, "e2e@example.invalid"),
		chromedp.SendKeys(`input[name="component"]`, "Airport search"),
		chromedp.Evaluate(`document.querySelector('select[name="severity"]').value = 'low'; document.querySelector('select[name="severity"]').dispatchEvent(new Event('change', {bubbles: true}))`, nil),
		chromedp.SendKeys(`input[name="summary"]`, "Controlled E2E security report"),
		chromedp.SendKeys(`textarea[name="steps_to_reproduce"]`, "Open the airport search page and enter the deterministic test query."),
		chromedp.SendKeys(`textarea[name="impact"]`, "No real vulnerability is reported; this is isolated E2E fixture data."),
		chromedp.Evaluate(`document.querySelector('select[name="credit_preference"]').value = 'no'; document.querySelector('select[name="credit_preference"]').dispatchEvent(new Event('change', {bubbles: true}))`, nil),
		chromedp.Evaluate(`document.querySelector('input[name="disclosure_agreement"]').checked = true`, nil),
		chromedp.SendKeys(`#contact-captcha`, strconv.Itoa(answer)),
	); err != nil {
		t.Fatalf("fill deterministic security contact fields: %v", err)
	}
	time.Sleep(3100 * time.Millisecond)
	if err := chromedp.Run(ctx,
		chromedp.Click(`form[data-captcha="simple"] button[type="submit"]`, chromedp.ByQuery),
		chromedp.WaitVisible(`#contact-review`, chromedp.ByQuery),
	); err != nil {
		t.Fatalf("submit security contact form for review: %v", err)
	}
	time.Sleep(2100 * time.Millisecond)
	if err := chromedp.Run(ctx,
		chromedp.Click(`input[name="confirmed"]`, chromedp.ByQuery),
		chromedp.Click(`section[aria-labelledby="contact-review"] form:first-of-type button[type="submit"]`, chromedp.ByQuery),
		chromedp.WaitVisible(`#contact-success`, chromedp.ByQuery),
	); err != nil {
		t.Fatalf("confirm successful security contact submission: %v", err)
	}
	var body string
	if err := chromedp.Run(ctx, chromedp.Text(`#contact-success`, &body)); err != nil || !strings.Contains(body, "received") {
		t.Fatalf("successful contact response lacks its confirmation (body=%q err=%v)", body, err)
	}
}

func TestNoJS_SecurityContactCompletesReviewAndSubmission(t *testing.T) {
	runSuccessfulSecurityContact(t, true)
}

func TestJS_SecurityContactCompletesReviewAndSubmission(t *testing.T) {
	runSuccessfulSecurityContact(t, false)
}

func e2eAPIBase(t *testing.T) string {
	t.Helper()
	status, _, body := getPageAccept(t, "/api/autodiscover", "application/json")
	if status != http.StatusOK {
		t.Fatalf("API autodiscover returned HTTP %d", status)
	}
	var response struct {
		Data struct {
			API struct {
				CurrentVersion string `json:"current_version"`
			} `json:"api"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(body), &response); err != nil || response.Data.API.CurrentVersion == "" {
		t.Fatalf("API autodiscover omitted data.api.current_version: %v", err)
	}
	return "/api/" + response.Data.API.CurrentVersion
}

func e2eAPIRequest(t *testing.T, method, path, bearer string, payload interface{}) (int, []byte) {
	t.Helper()
	var body io.Reader
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("encode API request payload: %v", err)
		}
		body = strings.NewReader(string(encoded))
	}
	request, err := http.NewRequest(method, baseURL(t)+path, body)
	if err != nil {
		t.Fatalf("create API request %s %s: %v", method, path, err)
	}
	request.Header.Set("Accept", "application/json")
	if payload != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	if bearer != "" {
		request.Header.Set("Authorization", "Bearer "+bearer)
	}
	response, err := (&http.Client{Timeout: 20 * time.Second}).Do(request)
	if err != nil {
		t.Fatalf("send API request %s %s: %v", method, path, err)
	}
	defer response.Body.Close()
	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("read API response %s %s: %v", method, path, err)
	}
	return response.StatusCode, responseBody
}

func apiOwnerToken(t *testing.T, body []byte) string {
	t.Helper()
	var response struct {
		Data struct {
			OwnerToken string `json:"owner_token"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &response); err != nil || !strings.HasPrefix(response.Data.OwnerToken, "own_") {
		t.Fatalf("API token response omitted a primary/linked token: %v", err)
	}
	return response.Data.OwnerToken
}

func mcpAirportCall(t *testing.T, apiBase, bearer string) (int, []byte) {
	t.Helper()
	payload := map[string]interface{}{
		"jsonrpc": "2.0", "id": 1, "method": "tools/call",
		"params": map[string]interface{}{
			"name": "getAirport", "arguments": map[string]interface{}{"ident": "KJFK"},
			"_meta": map[string]string{"io.modelcontextprotocol/protocolVersion": "2026-07-28"},
		},
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("encode MCP tool call: %v", err)
	}
	request, err := http.NewRequest(http.MethodPost, baseURL(t)+apiBase+"/server/mcp", strings.NewReader(string(encoded)))
	if err != nil {
		t.Fatalf("create MCP tool call: %v", err)
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("MCP-Protocol-Version", "2026-07-28")
	request.Header.Set("Mcp-Method", "tools/call")
	request.Header.Set("Mcp-Name", "getAirport")
	if bearer != "" {
		request.Header.Set("Authorization", "Bearer "+bearer)
	}
	response, err := (&http.Client{Timeout: 20 * time.Second}).Do(request)
	if err != nil {
		t.Fatalf("send MCP tool call: %v", err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("read MCP response: %v", err)
	}
	return response.StatusCode, body
}

func TestE2E_OwnerTokenAPIRotationLinkedManagementAndMCP(t *testing.T) {
	apiBase := e2eAPIBase(t)
	status, issueBody := e2eAPIRequest(t, http.MethodPost, apiBase+"/server/preferences/tokens/primary", "", nil)
	if status != http.StatusCreated {
		t.Fatalf("issue a primary token without authentication: HTTP %d, body %s", status, issueBody)
	}
	primary := apiOwnerToken(t, issueBody)
	if status, _ = e2eAPIRequest(t, http.MethodPost, apiBase+"/server/preferences/tokens/primary", primary, nil); status != http.StatusConflict {
		t.Fatalf("issuing a second primary with a valid token returned HTTP %d, want 409", status)
	}

	status, linkedBody := e2eAPIRequest(t, http.MethodPost, apiBase+"/server/preferences/tokens", primary, map[string]string{"name": "e2e-primary-linked", "expiry": "never"})
	if status != http.StatusCreated {
		t.Fatalf("create first linked token: HTTP %d", status)
	}
	firstLinked := apiOwnerToken(t, linkedBody)
	status, secondLinkedBody := e2eAPIRequest(t, http.MethodPost, apiBase+"/server/preferences/tokens", primary, map[string]string{"name": "e2e-secondary-linked", "expiry": "never"})
	if status != http.StatusCreated {
		t.Fatalf("create second linked token: HTTP %d", status)
	}
	secondLinked := apiOwnerToken(t, secondLinkedBody)

	status, listBody := e2eAPIRequest(t, http.MethodGet, apiBase+"/server/preferences/tokens", primary, nil)
	if status != http.StatusOK {
		t.Fatalf("list linked tokens: HTTP %d", status)
	}
	var tokenList struct {
		Data struct {
			Tokens []struct {
				ID        int64  `json:"id"`
				Name      string `json:"name"`
				Status    string `json:"status"`
				ExpiresAt *int64 `json:"expires_at"`
			} `json:"tokens"`
		} `json:"data"`
	}
	if err := json.Unmarshal(listBody, &tokenList); err != nil {
		t.Fatalf("decode owner token list: %v", err)
	}
	ids := make(map[string]int64)
	for _, item := range tokenList.Data.Tokens {
		ids[item.Name] = item.ID
	}
	firstID, secondID := ids["e2e-primary-linked"], ids["e2e-secondary-linked"]
	if firstID < 1 || secondID < 1 {
		t.Fatal("API token list did not return the IDs for both linked tokens")
	}

	for _, check := range []struct {
		method string
		path   string
		body   interface{}
	}{
		{http.MethodGet, apiBase + "/server/preferences/tokens", nil},
		{http.MethodPost, apiBase + "/server/preferences/tokens/primary/reveal", nil},
		{http.MethodPost, apiBase + "/server/preferences/tokens/primary/rotate", nil},
		{http.MethodDelete, apiBase + "/server/preferences/tokens/primary", map[string]string{"confirm": "0000"}},
		{http.MethodPost, apiBase + "/server/preferences/tokens", map[string]string{"name": "forbidden-linked-create", "expiry": "never"}},
		{http.MethodPost, fmt.Sprintf("%s/server/preferences/tokens/%d/reveal", apiBase, firstID), nil},
		{http.MethodPost, fmt.Sprintf("%s/server/preferences/tokens/%d/rotate", apiBase, firstID), nil},
		{http.MethodPatch, fmt.Sprintf("%s/server/preferences/tokens/%d", apiBase, firstID), map[string]string{"name": "forbidden-rename", "expiry": "never"}},
		{http.MethodDelete, fmt.Sprintf("%s/server/preferences/tokens/%d", apiBase, firstID), nil},
	} {
		status, _ := e2eAPIRequest(t, check.method, check.path, firstLinked, check.body)
		if status != http.StatusForbidden {
			t.Errorf("linked token %s %s returned HTTP %d, want 403", check.method, check.path, status)
		}
	}
	if status, _ := e2eAPIRequest(t, http.MethodPost, apiBase+"/server/preferences/tokens/primary", firstLinked, nil); status != http.StatusConflict {
		t.Errorf("issue-primary request with a valid linked token returned HTTP %d, want 409", status)
	}

	status, primaryRotateBody := e2eAPIRequest(t, http.MethodPost, apiBase+"/server/preferences/tokens/primary/rotate", primary, nil)
	if status != http.StatusOK {
		t.Fatalf("rotate primary token: HTTP %d", status)
	}
	rotatedPrimary := apiOwnerToken(t, primaryRotateBody)
	if status, _ = e2eAPIRequest(t, http.MethodGet, apiBase+"/server/preferences/tokens", primary, nil); status != http.StatusUnauthorized {
		t.Fatalf("old primary token returned HTTP %d after rotation, want 401", status)
	}
	if status, _ = e2eAPIRequest(t, http.MethodGet, apiBase+"/server/preferences/tokens", rotatedPrimary, nil); status != http.StatusOK {
		t.Fatalf("new primary token returned HTTP %d after rotation, want 200", status)
	}

	for label, token := range map[string]string{"primary": rotatedPrimary, "first linked": firstLinked, "second linked": secondLinked} {
		status, body := mcpAirportCall(t, apiBase, token)
		if status != http.StatusOK || !strings.Contains(string(body), "KJFK") {
			t.Errorf("MCP tools/call with %s token returned HTTP %d without KJFK: %s", label, status, body)
		}
	}
	if status, _ = mcpAirportCall(t, apiBase, ""); status != http.StatusUnauthorized {
		t.Errorf("MCP tools/call without a token returned HTTP %d, want 401", status)
	}

	status, linkedRotateBody := e2eAPIRequest(t, http.MethodPost, fmt.Sprintf("%s/server/preferences/tokens/%d/rotate", apiBase, firstID), rotatedPrimary, nil)
	if status != http.StatusOK {
		t.Fatalf("rotate linked token: HTTP %d", status)
	}
	rotatedLinked := apiOwnerToken(t, linkedRotateBody)
	if status, _ = mcpAirportCall(t, apiBase, firstLinked); status != http.StatusUnauthorized {
		t.Errorf("old linked token returned HTTP %d after rotation, want 401", status)
	}
	for label, token := range map[string]string{"primary": rotatedPrimary, "rotated linked": rotatedLinked, "other linked": secondLinked} {
		status, body := mcpAirportCall(t, apiBase, token)
		if status != http.StatusOK || !strings.Contains(string(body), "KJFK") {
			t.Errorf("MCP tools/call with %s token returned HTTP %d without KJFK after linked rotation: %s", label, status, body)
		}
	}

	status, _ = e2eAPIRequest(t, http.MethodPatch, fmt.Sprintf("%s/server/preferences/tokens/%d", apiBase, secondID), rotatedPrimary, map[string]string{"name": "e2e-renamed-linked", "expiry": "7d"})
	if status != http.StatusOK {
		t.Fatalf("rename linked token and change its expiry: HTTP %d", status)
	}
	status, listBody = e2eAPIRequest(t, http.MethodGet, apiBase+"/server/preferences/tokens", rotatedPrimary, nil)
	if status != http.StatusOK || json.Unmarshal(listBody, &tokenList) != nil {
		t.Fatalf("read renamed/expiry-updated token list: HTTP %d", status)
	}
	updated := false
	for _, item := range tokenList.Data.Tokens {
		if item.ID == secondID && item.Name == "e2e-renamed-linked" && item.ExpiresAt != nil && item.Status == "active" {
			updated = true
		}
	}
	if !updated {
		t.Fatal("linked token list did not reflect its renamed label and active expiry")
	}
	status, _ = e2eAPIRequest(t, http.MethodDelete, fmt.Sprintf("%s/server/preferences/tokens/%d", apiBase, secondID), rotatedPrimary, nil)
	if status != http.StatusOK {
		t.Fatalf("revoke renamed linked token: HTTP %d", status)
	}
	if status, _ = mcpAirportCall(t, apiBase, secondLinked); status != http.StatusUnauthorized {
		t.Errorf("revoked linked token returned HTTP %d to MCP, want 401", status)
	}
}
