package server

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/apimgr/airports/src/config"
)

func TestRenderOperatorContentSanitizesHTMLAndContactLinks(t *testing.T) {
	request := httptest.NewRequest("GET", "https://airports.example/server/about", nil)
	content := config.BrandingContent{
		Format: "html",
		Body:   `<p class="notice" onclick="alert(1)">Safe <strong>text</strong></p><script>alert(2)</script><a href="/server/contact">Contact</a><a href="https://example.com">External</a>`,
	}
	rendered := string(renderOperatorContent(request, "about", "en", content, "", "", false))
	if strings.Contains(rendered, "script") || strings.Contains(rendered, "onclick") || strings.Contains(rendered, "/server/contact") {
		t.Fatalf("operator HTML retained disallowed content: %s", rendered)
	}
	if !strings.Contains(rendered, `class="notice"`) || !strings.Contains(rendered, "<strong>text</strong>") || !strings.Contains(rendered, `rel="noopener noreferrer nofollow ugc"`) {
		t.Fatalf("operator HTML lost allowed markup: %s", rendered)
	}
}

func TestRenderOperatorContentResolvesLanguageAndMarkdown(t *testing.T) {
	request := httptest.NewRequest("GET", "https://airports.example/server/about", nil)
	content := config.BrandingContent{
		Format: "md",
		Body: map[string]interface{}{
			"default": "Default **body**",
			"fr":      "# Bonjour\n\n[Site](https://example.com)",
		},
	}
	rendered := string(renderOperatorContent(request, "about", "fr", content, "", "", false))
	if !strings.Contains(rendered, "<h1>Bonjour</h1>") || !strings.Contains(rendered, `<a href="https://example.com" rel="noopener noreferrer nofollow ugc">Site</a>`) {
		t.Fatalf("operator markdown was not language-resolved/rendered: %s", rendered)
	}
}

func TestOperatorContentRejectsOversizedAndUnsupportedEntries(t *testing.T) {
	request := httptest.NewRequest("GET", "https://airports.example/server/about", nil)
	if got := renderOperatorContent(request, "about", "en", config.BrandingContent{Format: "md", Body: strings.Repeat("x", maxOperatorContentBytes+1)}, "", "", false); got != "" {
		t.Fatal("oversized operator content rendered")
	}
	if got := renderOperatorContent(request, "footer", "en", config.BrandingContent{Format: "md", Body: "text"}, "", "", false); got != "" {
		t.Fatal("footer accepted non-HTML format")
	}
}

func TestRenderOperatorContentStripsExternalLinksOnTor(t *testing.T) {
	request := httptest.NewRequest("GET", "http://sampleonionaddress.onion/server/privacy", nil)
	content := config.BrandingContent{Format: "md", Body: "[Clearnet](https://example.com/policy) and [Onion](http://sampleonionaddress.onion/about)"}
	rendered := string(renderOperatorContent(request, "privacy", "en", content, "", "", true))
	if strings.Contains(rendered, `href="https://example.com/policy"`) || !strings.Contains(rendered, "Clearnet") {
		t.Fatalf("Tor render must preserve external-link text but remove its href: %s", rendered)
	}
	if !strings.Contains(rendered, `href="http://sampleonionaddress.onion/about"`) {
		t.Fatalf("Tor render removed a same-onion link: %s", rendered)
	}
}
