package server

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/apimgr/airports/src/config"
	xhtml "golang.org/x/net/html"
)

var pageTestAddress atomic.Uint32

func TestRegisteredPageRoutesAndHubLinks(t *testing.T) {
	s := contactTestServer(t)
	registries := []struct {
		name  string
		path  string
		pages []helpPage
	}{
		{name: "help", path: "/server/help", pages: s.registeredHelpPages("en")},
		{name: "preferences", path: "/server/preferences", pages: s.registeredPreferencePages("en")},
	}
	for _, registry := range registries {
		t.Run(registry.name, func(t *testing.T) {
			hub := servePage(t, s, registry.path)
			if hub.Code != http.StatusOK {
				t.Fatalf("hub %s status = %d", registry.path, hub.Code)
			}
			for _, page := range registry.pages {
				response := servePage(t, s, page.Path)
				if response.Code != http.StatusOK || !strings.Contains(response.Header().Get("Content-Type"), "text/html") {
					t.Errorf("registered page %s status/content-type = %d/%q", page.Path, response.Code, response.Header().Get("Content-Type"))
				}
				if count := hubListLinkCount(t, hub.Body.String(), page.Path); count != 1 {
					t.Errorf("hub %s links %s %d times, want once", registry.path, page.Path, count)
				}
			}
		})
	}

	api := httptest.NewRecorder()
	s.router.ServeHTTP(api, httptest.NewRequest(http.MethodGet, "/api/v1/server/help", nil))
	if api.Code != http.StatusOK {
		t.Fatalf("help API status = %d, body=%s", api.Code, api.Body.String())
	}
	for _, page := range s.registeredHelpPages("en") {
		if count := strings.Count(api.Body.String(), `"path": "`+page.Path+`"`); count != 1 {
			t.Errorf("help API lists %s %d times, want once", page.Path, count)
		}
	}
}

func TestDocumentationUIThemeAssets(t *testing.T) {
	s := contactTestServer(t)
	for _, path := range []string{
		"/server/docs/swagger/theme.css",
		"/server/docs/graphql/theme.css",
		"/static/vendor/swagger-ui/swagger-ui.css",
		"/static/vendor/swagger-ui/swagger-ui-bundle.js",
		"/static/vendor/graphiql/graphiql.min.css",
		"/static/vendor/graphiql/react.production.min.js",
		"/static/js/docs-swagger.js",
		"/static/js/docs-graphql.js",
		"/static/css/docs.css",
	} {
		request := httptest.NewRequest(http.MethodGet, path, nil)
		request.RemoteAddr = nextPageTestAddress()
		response := httptest.NewRecorder()
		s.router.ServeHTTP(response, request)
		if response.Code != http.StatusOK {
			t.Errorf("asset %s status = %d", path, response.Code)
		}
		if strings.HasSuffix(path, ".css") && !strings.Contains(response.Header().Get("Content-Type"), "text/css") {
			t.Errorf("stylesheet %s content-type = %q", path, response.Header().Get("Content-Type"))
		}
		if strings.Contains(path, "theme.css") && !strings.Contains(response.Body.String(), "var(--color-bg)") {
			t.Errorf("theme stylesheet %s does not use the shared palette", path)
		}
	}
}

func TestServerPageHTMLConformance(t *testing.T) {
	s := contactTestServer(t)
	s.config.Server.Healthz.Root.Enabled = true
	s.setupRouter()
	pages := []string{
		"/", "/healthz", "/airports", "/airports/search?q=JFK", "/airports/nearby?lat=40.6398&lon=-73.7789&radius=50",
		"/airports/KJFK", "/stats", "/geoip", "/server/about", "/server/help", "/server/help/mcp", "/server/healthz",
		"/server/privacy", "/server/preferences", "/server/preferences/sync", "/server/preferences/tokens",
		"/server/dpo", "/server/terms", "/server/contact", "/server/security",
		"/server/security/policy", "/server/security/thanks", "/server/security/report/not-found",
		"/server/docs/swagger", "/server/docs/graphql",
	}
	for _, path := range pages {
		t.Run(path, func(t *testing.T) {
			response := servePage(t, s, path)
			if response.Code >= http.StatusInternalServerError && path != "/server/healthz" {
				t.Fatalf("page status = %d, body=%q", response.Code, response.Body.String())
			}
			if !strings.Contains(response.Header().Get("Content-Type"), "text/html") {
				t.Fatalf("page content type = %q", response.Header().Get("Content-Type"))
			}
			assertPageHTML(t, response.Body.String())
			assertPageBreadcrumb(t, response.Body.String(), path)
			assertInternalPageLinks(t, s, response.Body.String())
			if path == "/server/docs/swagger" {
				assertInteractiveDocsPage(t, response.Body.String(), "/api/v1/server/swagger")
			}
			if path == "/server/docs/graphql" {
				assertInteractiveDocsPage(t, response.Body.String(), "/api/v1/server/graphql")
			}
		})
	}
	for _, testCase := range []struct {
		name   string
		method string
		path   string
		status int
	}{
		{name: "not found", method: http.MethodGet, path: "/missing-page", status: http.StatusNotFound},
	} {
		t.Run("error/"+testCase.name, func(t *testing.T) {
			request := httptest.NewRequest(testCase.method, testCase.path, nil)
			request.RemoteAddr = nextPageTestAddress()
			request.Header.Set("Accept", "text/html")
			response := httptest.NewRecorder()
			s.router.ServeHTTP(response, request)
			if response.Code != testCase.status {
				t.Fatalf("error page status = %d, want %d", response.Code, testCase.status)
			}
			assertPageHTML(t, response.Body.String())
			assertInternalPageLinks(t, s, response.Body.String())
		})
	}
	t.Run("error/method not allowed", func(t *testing.T) {
		request := httptest.NewRequest(http.MethodPut, "/server/about", nil)
		request.RemoteAddr = nextPageTestAddress()
		request.Header.Set("Accept", "text/html")
		response := httptest.NewRecorder()
		s.renderErrorPage(response, request, http.StatusMethodNotAllowed)
		if response.Code != http.StatusMethodNotAllowed {
			t.Fatalf("error page status = %d, want %d", response.Code, http.StatusMethodNotAllowed)
		}
		assertPageHTML(t, response.Body.String())
		assertInternalPageLinks(t, s, response.Body.String())
	})
	t.Run("operator header fragment", func(t *testing.T) {
		originalContent := s.config.Server.Branding.Content
		content := make(map[string]config.BrandingContent, len(originalContent)+1)
		for name, value := range originalContent {
			content[name] = value
		}
		content["header"] = config.BrandingContent{
			Format: "html",
			Body:   `<span id="operator-header-fragment">Alignment header notice</span>`,
		}
		s.config.Server.Branding.Content = content
		defer func() { s.config.Server.Branding.Content = originalContent }()
		response := servePage(t, s, "/server/about")
		if response.Code != http.StatusOK {
			t.Fatalf("header-fragment page status = %d", response.Code)
		}
		assertPageHTML(t, response.Body.String())
		document, err := xhtml.Parse(strings.NewReader(response.Body.String()))
		if err != nil {
			t.Fatalf("parse header-fragment page: %v", err)
		}
		var header *xhtml.Node
		var findHeader func(*xhtml.Node)
		findHeader = func(node *xhtml.Node) {
			if node.Type == xhtml.ElementNode && node.Data == "header" && attrValue(node, "id") == "main-header" {
				header = node
				return
			}
			for child := node.FirstChild; child != nil && header == nil; child = child.NextSibling {
				findHeader(child)
			}
		}
		findHeader(document)
		if header == nil {
			t.Fatal("page has no shared header")
		}
		var elementChildren []*xhtml.Node
		for child := header.FirstChild; child != nil; child = child.NextSibling {
			if child.Type == xhtml.ElementNode {
				elementChildren = append(elementChildren, child)
			}
		}
		if len(elementChildren) != 2 || attrValue(elementChildren[0], "class") != "header-container" || attrValue(elementChildren[1], "id") != "operator-header-fragment" {
			t.Errorf("header element children = %d; want unchanged container followed by operator fragment", len(elementChildren))
		}
	})
	s.config.Server.Privacy.Data.Sold = true
	privacy := servePage(t, s, "/server/privacy")
	assertPageHTML(t, privacy.Body.String())
	for _, expected := range []string{`href="#ccpa-opt-out"`, `id="ccpa-opt-out"`, `action="/server/ccpa"`, `name="choice" value="opt-out"`, `name="csrf_token"`} {
		if !strings.Contains(privacy.Body.String(), expected) {
			t.Errorf("CCPA privacy form is missing %q", expected)
		}
	}
}

func assertInteractiveDocsPage(t *testing.T, source, fallbackPath string) {
	t.Helper()
	document, err := xhtml.Parse(strings.NewReader(source))
	if err != nil {
		t.Fatalf("parse interactive docs page: %v", err)
	}
	sections, pageLead := 0, false
	var walk func(*xhtml.Node)
	walk = func(node *xhtml.Node) {
		if node.Type == xhtml.ElementNode {
			if node.Data == "section" && hasClass(node, "section-card") {
				sections++
				if firstElement(node, "h2") == nil {
					t.Error("interactive docs card is missing its heading")
				}
			}
			if node.Data == "p" && hasClass(node, "page-lead") {
				pageLead = true
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(document)
	noscriptFallback := strings.Contains(source, "<noscript>") && strings.Contains(source, "JavaScript required") && strings.Contains(source, fallbackPath)
	if sections < 1 || !pageLead || !noscriptFallback {
		t.Errorf("interactive docs page has %d parsed cards, page lead=%t, noscript fallback=%t; want a headed tool card, a page lead, and a plain endpoint link in noscript", sections, pageLead, noscriptFallback)
	}
}

func servePage(t *testing.T, s *Server, path string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, path, nil)
	request.RemoteAddr = nextPageTestAddress()
	request.Header.Set("Accept", "text/html")
	response := httptest.NewRecorder()
	s.router.ServeHTTP(response, request)
	return response
}

func assertPageHTML(t *testing.T, source string) {
	t.Helper()
	document, err := xhtml.Parse(strings.NewReader(source))
	if err != nil {
		t.Fatalf("parse page HTML: %v", err)
	}
	mainCount, h1Count := 0, 0
	var body, pageHeader *xhtml.Node
	var walk func(*xhtml.Node)
	walk = func(node *xhtml.Node) {
		if node.Type == xhtml.ElementNode {
			switch node.Data {
			case "body":
				body = node
			case "header":
				if attrValue(node, "id") == "main-header" {
					pageHeader = node
				}
			case "main":
				mainCount++
			case "h1":
				h1Count++
				if !hasClass(node, "page-title") {
					t.Error("page title h1 is missing .page-title")
				}
			case "style":
				t.Error("page contains inline style element")
			case "script":
				if strings.TrimSpace(nodeText(node)) != "" && attrValue(node, "type") != "application/ld+json" {
					t.Error("page contains inline script body")
				}
			case "img":
				if _, ok := nodeAttr(node, "alt"); !ok {
					t.Error("image is missing alt attribute")
				}
			case "table":
				if firstElement(node, "thead") == nil {
					t.Error("table is missing thead")
				}
				if !hasAncestorClass(node, "table-wrapper") {
					t.Error("table is outside .table-wrapper")
				}
			case "form":
				if strings.EqualFold(attrValue(node, "method"), "post") && !formHasCSRF(node) {
					t.Error("POST form is missing csrf_token")
				}
			case "input", "select", "textarea":
				if attrValue(node, "type") != "hidden" && !hasControlLabel(node) {
					t.Errorf("%s control id=%q type=%q name=%q is missing a label", node.Data, attrValue(node, "id"), attrValue(node, "type"), attrValue(node, "name"))
				}
			}
			for _, attr := range node.Attr {
				if strings.EqualFold(attr.Key, "style") || strings.HasPrefix(strings.ToLower(attr.Key), "on") {
					t.Errorf("inline style or event attribute %q is forbidden", attr.Key)
				}
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(document)
	if mainCount != 1 || h1Count != 1 {
		t.Errorf("page has %d main elements and %d h1 elements, want exactly one of each", mainCount, h1Count)
	}
	if body == nil {
		t.Fatal("page has no body")
	}
	assertHeaderChildren(t, pageHeader)
	var first []string
	for child := body.FirstChild; child != nil && len(first) < 2; child = child.NextSibling {
		if child.Type == xhtml.ElementNode {
			first = append(first, child.Data+"."+attrValue(child, "class"))
		}
	}
	if len(first) != 2 || first[0] != "a.skip-link" || first[1] != "a.skip-link" {
		t.Errorf("first body elements = %v, want the two skip links", first)
	}
}

func assertHeaderChildren(t *testing.T, header *xhtml.Node) {
	t.Helper()
	if header == nil {
		t.Error("page has no shared header")
		return
	}
	var container *xhtml.Node
	for child := header.FirstChild; child != nil; child = child.NextSibling {
		if child.Type == xhtml.ElementNode && hasClass(child, "header-container") {
			container = child
			break
		}
	}
	if container == nil {
		t.Error("shared header has no .header-container")
		return
	}
	var actual []string
	for child := container.FirstChild; child != nil; child = child.NextSibling {
		if child.Type == xhtml.ElementNode {
			actual = append(actual, child.Data+"."+firstClass(child))
		}
	}
	want := []string{"a.logo", "nav.nav-links", "label.nav-toggle", "div.header-actions"}
	if strings.Join(actual, ",") != strings.Join(want, ",") {
		t.Errorf("header elements = %v, want %v", actual, want)
	}
	brand := firstElement(container, "a")
	if brand == nil || attrValue(brand, "class") != "logo" || attrValue(brand, "href") != "/" {
		t.Errorf("brand link = %#v, want one .logo link to /", brand)
	}
	var nav, actions *xhtml.Node
	for child := container.FirstChild; child != nil; child = child.NextSibling {
		if child.Type != xhtml.ElementNode {
			continue
		}
		if hasClass(child, "nav-links") {
			nav = child
		}
		if hasClass(child, "header-actions") {
			actions = child
		}
	}
	if nav == nil || actions == nil {
		t.Fatal("header is missing its navigation or actions container")
	}
	preferences, themeForm := 0, 0
	for child := actions.FirstChild; child != nil; child = child.NextSibling {
		if child.Type != xhtml.ElementNode {
			continue
		}
		switch {
		case child.Data == "a" && hasClass(child, "preferences-link"):
			preferences++
		case child.Data == "form" && hasClass(child, "theme-toggle-form"):
			themeForm++
			if strings.ToLower(attrValue(child, "method")) != "post" || attrValue(child, "action") != "/server/preferences" {
				t.Errorf("theme toggle form method/action = %q/%q", attrValue(child, "method"), attrValue(child, "action"))
			}
		default:
			t.Errorf("unexpected header action %s.%s", child.Data, attrValue(child, "class"))
		}
	}
	if preferences != 1 || themeForm != 1 {
		t.Errorf("header actions have %d preferences links and %d theme forms, want one of each", preferences, themeForm)
	}
}

func assertPageBreadcrumb(t *testing.T, source, path string) {
	t.Helper()
	parent := ""
	switch {
	case strings.HasPrefix(path, "/server/help/"):
		parent = "/server/help"
	case strings.HasPrefix(path, "/server/preferences/"):
		parent = "/server/preferences"
	}
	document, err := xhtml.Parse(strings.NewReader(source))
	if err != nil {
		t.Fatalf("parse page HTML: %v", err)
	}
	count := 0
	var walk func(*xhtml.Node)
	walk = func(node *xhtml.Node) {
		if node.Type == xhtml.ElementNode && node.Data == "nav" && hasClass(node, "breadcrumb") {
			for link := node.FirstChild; link != nil; link = link.NextSibling {
				if link.Type == xhtml.ElementNode && link.Data == "a" && attrValue(link, "href") == parent {
					count++
				}
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(document)
	if parent == "" && count != 0 {
		t.Errorf("hub or non-sub-page %s has %d breadcrumbs", path, count)
	}
	if parent != "" && count != 1 {
		t.Errorf("sub-page %s has %d breadcrumb links to %s, want one", path, count, parent)
	}
}

func assertInternalPageLinks(t *testing.T, s *Server, source string) {
	t.Helper()
	document, err := xhtml.Parse(strings.NewReader(source))
	if err != nil {
		t.Fatalf("parse page HTML: %v", err)
	}
	seen := map[string]bool{}
	var walk func(*xhtml.Node)
	walk = func(node *xhtml.Node) {
		if node.Type == xhtml.ElementNode && node.Data == "a" {
			href := strings.TrimSpace(attrValue(node, "href"))
			if strings.HasPrefix(href, "/") && !strings.HasPrefix(href, "//") {
				if parsed, parseErr := url.Parse(href); parseErr == nil && parsed.RawQuery == "" && parsed.Path != "" && !seen[parsed.Path] {
					seen[parsed.Path] = true
					request := httptest.NewRequest(http.MethodGet, parsed.Path, nil)
					request.RemoteAddr = nextPageTestAddress()
					request.Header.Set("Accept", "text/html")
					response := httptest.NewRecorder()
					s.router.ServeHTTP(response, request)
					if response.Code == http.StatusNotFound || response.Code == http.StatusMethodNotAllowed {
						t.Errorf("internal link %q resolves with status %d", href, response.Code)
					}
				}
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(document)
}

func nextPageTestAddress() string {
	return fmt.Sprintf("192.0.2.%d:1234", pageTestAddress.Add(1)%250+1)
}

func hasClass(node *xhtml.Node, class string) bool {
	for _, value := range strings.Fields(attrValue(node, "class")) {
		if value == class {
			return true
		}
	}
	return false
}

func firstClass(node *xhtml.Node) string {
	classes := strings.Fields(attrValue(node, "class"))
	if len(classes) == 0 {
		return ""
	}
	return classes[0]
}

func hubListLinkCount(t *testing.T, source, path string) int {
	t.Helper()
	document, err := xhtml.Parse(strings.NewReader(source))
	if err != nil {
		t.Fatalf("parse hub HTML: %v", err)
	}
	count := 0
	var walk func(*xhtml.Node, bool)
	walk = func(node *xhtml.Node, inHubList bool) {
		if node.Type == xhtml.ElementNode {
			for _, value := range strings.Fields(attrValue(node, "class")) {
				if value == "hub-list" {
					inHubList = true
				}
			}
			if inHubList && node.Data == "a" && strings.TrimSuffix(attrValue(node, "href"), "/") == path {
				count++
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child, inHubList)
		}
	}
	walk(document, false)
	return count
}

func nodeText(node *xhtml.Node) string {
	var value strings.Builder
	var walk func(*xhtml.Node)
	walk = func(current *xhtml.Node) {
		if current.Type == xhtml.TextNode {
			value.WriteString(current.Data)
		}
		for child := current.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(node)
	return value.String()
}

func nodeAttr(node *xhtml.Node, name string) (string, bool) {
	for _, attr := range node.Attr {
		if strings.EqualFold(attr.Key, name) {
			return attr.Val, true
		}
	}
	return "", false
}

func attrValue(node *xhtml.Node, name string) string {
	value, _ := nodeAttr(node, name)
	return value
}

func firstElement(node *xhtml.Node, name string) *xhtml.Node {
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if child.Type == xhtml.ElementNode && child.Data == name {
			return child
		}
		if found := firstElement(child, name); found != nil {
			return found
		}
	}
	return nil
}

func hasAncestorClass(node *xhtml.Node, class string) bool {
	for parent := node.Parent; parent != nil; parent = parent.Parent {
		for _, value := range strings.Fields(attrValue(parent, "class")) {
			if value == class {
				return true
			}
		}
	}
	return false
}

func formHasCSRF(form *xhtml.Node) bool {
	var found bool
	var walk func(*xhtml.Node)
	walk = func(node *xhtml.Node) {
		if node.Type == xhtml.ElementNode && node.Data == "input" && attrValue(node, "name") == "csrf_token" {
			found = true
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(form)
	return found
}

func hasControlLabel(control *xhtml.Node) bool {
	if strings.TrimSpace(attrValue(control, "aria-label")) != "" || strings.TrimSpace(attrValue(control, "aria-labelledby")) != "" {
		return true
	}
	for parent := control.Parent; parent != nil; parent = parent.Parent {
		if parent.Type == xhtml.ElementNode && parent.Data == "label" {
			return true
		}
	}
	id := attrValue(control, "id")
	if id == "" {
		return false
	}
	var document *xhtml.Node
	for node := control; node != nil; node = node.Parent {
		if node.Type == xhtml.DocumentNode {
			document = node
			break
		}
	}
	var labelled bool
	var walk func(*xhtml.Node)
	walk = func(node *xhtml.Node) {
		if node.Type == xhtml.ElementNode && node.Data == "label" && attrValue(node, "for") == id {
			labelled = true
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(document)
	return labelled
}
