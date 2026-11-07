//go:build e2e

package e2e

import (
	"context"
	"encoding/base64"
	"encoding/csv"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
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

func TestSSR_HomeHasRenderedSearchAndNavigation(t *testing.T) {
	assertSSRPage(t, "/", "Search", "/airports/search", "/airports/nearby")
}

func TestSSR_SearchReturnsAirportData(t *testing.T) {
	assertSSRPage(t, "/airports/search?q="+url.QueryEscape("JFK"), "JFK", "airport-card")
}

func TestSSR_AirportDetailContainsCoordinatesAndMapBaseline(t *testing.T) {
	assertSSRPage(t, "/airports/KJFK", "KJFK", "airport-map", "openstreetmap.org")
}

func TestSSR_BoundingBoxSearchRendersAirportData(t *testing.T) {
	assertSSRPage(t, "/airports/nearby?minLat=40&maxLat=41&minLon=-74&maxLon=-73", "results-grid", "airport-map")
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

func TestSSR_UnknownRouteUsesThemed404(t *testing.T) {
	status, headers, body := getPageAccept(t, "/this-route-does-not-exist", "text/html")
	if status != http.StatusNotFound || !strings.Contains(headers.Get("Content-Type"), "text/html") || !strings.Contains(body, "theme-dark") || len(strings.TrimSpace(body)) < 100 {
		t.Fatalf("unknown route is not a rendered themed 404 (status %d)", status)
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
		if visited[canonical] || strings.HasPrefix(canonical, "/static/") || strings.HasPrefix(canonical, "/locales/") {
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
	if err := chromedp.Run(ctx, chromedp.Click(`[data-action="detect-location"]`), chromedp.WaitVisible("#app-modal", chromedp.ByQuery)); err != nil {
		t.Fatalf("run positive GeoIP location flow: %v", err)
	}
	var modalText string
	if err := chromedp.Run(ctx, chromedp.Text("#app-modal", &modalText)); err != nil {
		t.Fatal(err)
	}
	select {
	case status := <-responseStatuses:
		if status != http.StatusOK {
			t.Fatalf("GeoIP fixture response status = %d, want %d", status, http.StatusOK)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("browser did not receive the positive GeoIP fixture response")
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

func TestNoJS_AirportDetailKeepsStaticMapLink(t *testing.T) {
	ctx := browserContext(t, true)
	page := navigate(t, ctx, "/airports/KJFK")
	if !strings.Contains(page, "KJFK") || !strings.Contains(page, "openstreetmap.org") {
		t.Fatal("no-JS airport detail lacks airport data or static OpenStreetMap link")
	}
}

func TestNoJS_NearbyCoordinatesRenderBaseline(t *testing.T) {
	ctx := browserContext(t, true)
	page := navigate(t, ctx, "/airports/nearby?lat=40.6413&lon=-73.7781&radius=25")
	if !strings.Contains(page, "Nearby") && !strings.Contains(page, "JFK") {
		t.Fatal("no-JS nearby page lacks rendered nearby-airport content")
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
	if !strings.Contains(page, `action="/theme"`) || !strings.Contains(page, `name="csrf_token"`) || !strings.Contains(page, `name="theme"`) {
		t.Fatal("no-JS theme fallback lacks its POST form or CSRF token")
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
	if err := chromedp.Run(ctx, chromedp.Click(".mobile-menu-toggle")); err != nil {
		t.Fatalf("open mobile navigation: %v", err)
	}
	var searchNavigationVisible bool
	if err := chromedp.Run(ctx, chromedp.Evaluate(`(() => { const link = document.querySelector('a[href="/airports/search"]'); return !!link && link.getBoundingClientRect().width > 0 && link.getBoundingClientRect().height > 0; })()`, &searchNavigationVisible)); err != nil {
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
		if err := chromedp.Run(ctx, network.SetCookie("theme", theme).WithURL(baseURL(t)).WithPath("/"), chromedp.Navigate(baseURL(t)+"/")); err != nil {
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
