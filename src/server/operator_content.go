package server

import (
	"bytes"
	"html"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/apimgr/airports/src/common/i18n"
	"github.com/apimgr/airports/src/config"
	"github.com/microcosm-cc/bluemonday"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	xhtml "golang.org/x/net/html"
)

const maxOperatorContentBytes = 256 * 1024

var invalidOperatorContentWarnings sync.Map

func (s *Server) operatorHTML(r *http.Request, name string) template.HTML {
	s.configMu.RLock()
	cfg := s.config
	s.configMu.RUnlock()
	if cfg == nil {
		return ""
	}
	onionAddress := ""
	if cfg.Server.Tor.OnionAddress != "" {
		onionAddress = cfg.Server.Tor.OnionAddress
	}
	i2pAddress := ""
	if s.i2p != nil {
		i2pAddress = s.i2p.EepsiteAddress()
	}
	return renderOperatorContent(r, name, i18n.FromContext(r.Context()), cfg.Server.Branding.Content[name], onionAddress, i2pAddress)
}

func (s *Server) operatorContentSource(r *http.Request, name string) (string, string, bool) {
	s.configMu.RLock()
	cfg := s.config
	s.configMu.RUnlock()
	if cfg == nil || !validOperatorContentName(name) {
		return "", "", false
	}
	content := cfg.Server.Branding.Content[name]
	format := strings.ToLower(strings.TrimSpace(content.Format))
	if format == "" {
		format = "md"
	}
	if (name == "header" || name == "footer") && format != "html" {
		return "", "", false
	}
	if format != "md" && format != "html" && format != "txt" {
		return "", "", false
	}
	body, ok := operatorContentBody(content.Body, i18n.FromContext(r.Context()))
	if !ok || len([]byte(body)) > maxOperatorContentBytes {
		return "", "", false
	}
	onionAddress := ""
	if cfg.Server.Tor.OnionAddress != "" {
		onionAddress = cfg.Server.Tor.OnionAddress
	}
	i2pAddress := ""
	if s.i2p != nil {
		i2pAddress = s.i2p.EepsiteAddress()
	}
	return mapOperatorContentFormat(format), expandOperatorVariables(body, onionAddress, i2pAddress), true
}

func mapOperatorContentFormat(format string) string {
	switch format {
	case "md":
		return "markdown"
	case "txt":
		return "text"
	default:
		return format
	}
}

func renderOperatorContent(r *http.Request, name, language string, content config.BrandingContent, onionAddress, i2pAddress string) template.HTML {
	if !validOperatorContentName(name) {
		return ""
	}
	format := strings.ToLower(strings.TrimSpace(content.Format))
	if format == "" {
		format = "md"
	}
	if (name == "header" || name == "footer") && format != "html" {
		warnInvalidOperatorContent(name)
		return ""
	}
	if format != "md" && format != "html" && format != "txt" {
		warnInvalidOperatorContent(name)
		return ""
	}
	body, ok := operatorContentBody(content.Body, language)
	if !ok || len([]byte(body)) > maxOperatorContentBytes {
		warnInvalidOperatorContent(name)
		return ""
	}
	body = expandOperatorVariables(body, onionAddress, i2pAddress)
	switch format {
	case "md":
		body = renderOperatorMarkdown(body)
	case "txt":
		body = renderOperatorText(body)
	}
	return template.HTML(sanitizeOperatorHTML(body, name == "header" || name == "footer"))
}

func warnInvalidOperatorContent(name string) {
	if _, loaded := invalidOperatorContentWarnings.LoadOrStore(name, struct{}{}); !loaded {
		log.Printf("Warning: ignoring unusable server.branding.content.%s configuration", name)
	}
}

func validOperatorContentName(name string) bool {
	switch name {
	case "header", "about", "privacy", "terms", "footer":
		return true
	default:
		return false
	}
}

func operatorContentUsable(name string, content config.BrandingContent) bool {
	if !validOperatorContentName(name) {
		return false
	}
	format := strings.ToLower(strings.TrimSpace(content.Format))
	if format == "" {
		format = "md"
	}
	if format != "md" && format != "html" && format != "txt" {
		return false
	}
	if (name == "header" || name == "footer") && format != "html" {
		return false
	}
	switch body := content.Body.(type) {
	case string:
		return len([]byte(body)) <= maxOperatorContentBytes
	case map[string]interface{}:
		for _, value := range body {
			if text, ok := value.(string); ok && len([]byte(text)) <= maxOperatorContentBytes {
				return true
			}
		}
	case map[interface{}]interface{}:
		for _, value := range body {
			if text, ok := value.(string); ok && len([]byte(text)) <= maxOperatorContentBytes {
				return true
			}
		}
	}
	return false
}

func operatorContentBody(value interface{}, language string) (string, bool) {
	switch body := value.(type) {
	case string:
		return body, true
	case map[string]interface{}:
		if text, ok := body[language].(string); ok {
			return text, true
		}
		text, ok := body["default"].(string)
		return text, ok
	case map[interface{}]interface{}:
		if text, ok := body[language].(string); ok {
			return text, true
		}
		text, ok := body["default"].(string)
		return text, ok
	default:
		return "", false
	}
}

func expandOperatorVariables(body, onionAddress, i2pAddress string) string {
	projectName := "airports"
	projectOrg := "apimgr"
	version := Version
	replacer := strings.NewReplacer(
		"{current_year}", time.Now().Format("2006"),
		"{project_name}", projectName,
		"{project_org}", projectOrg,
		"{project_version}", version,
		"{build_datetime}", time.Now().UTC().Format("January 02, 2006 at 15:04:05 UTC"),
		"{onion_address}", onionAddress,
		"{i2p_address}", i2pAddress,
	)
	return replacer.Replace(body)
}

func renderOperatorText(body string) string {
	blocks := strings.Split(strings.TrimSpace(body), "\n\n")
	var rendered strings.Builder
	for _, block := range blocks {
		if strings.TrimSpace(block) == "" {
			continue
		}
		rendered.WriteString("<p>")
		rendered.WriteString(strings.ReplaceAll(html.EscapeString(strings.TrimSpace(block)), "\n", " "))
		rendered.WriteString("</p>")
	}
	return rendered.String()
}

func renderOperatorMarkdown(body string) string {
	var rendered bytes.Buffer
	markdown := goldmark.New(goldmark.WithExtensions(extension.GFM))
	if err := markdown.Convert([]byte(body), &rendered); err != nil {
		return "<p>" + html.EscapeString(body) + "</p>"
	}
	return rendered.String()
}

func sanitizeOperatorHTML(source string, strict bool) string {
	policy := bluemonday.NewPolicy()
	policy.AllowElements("p", "br", "span", "div", "strong", "b", "em", "i", "u", "s", "small", "h1", "h2", "h3", "h4", "h5", "h6", "ul", "ol", "li", "a", "img")
	if !strict {
		policy.AllowElements("table", "thead", "tbody", "tr", "th", "td", "pre", "code", "blockquote", "hr", "dl", "dt", "dd", "sup", "sub")
	}
	policy.AllowAttrs("class", "id").Globally()
	policy.AllowAttrs("href", "title", "target", "rel").OnElements("a")
	policy.AllowAttrs("src", "alt", "title", "width", "height").OnElements("img")
	policy.AllowRelativeURLs(true)
	policy.AllowURLSchemes("http", "https", "mailto", "data")
	policy.RequireNoReferrerOnLinks(true)
	policy.RequireNoFollowOnLinks(true)
	return policy.Sanitize(sanitizeOperatorHTMLLegacy(source, strict))
}

func sanitizeOperatorHTMLLegacy(source string, strict bool) string {
	allowed := map[string]bool{
		"p": true, "br": true, "span": true, "div": true, "strong": true, "b": true,
		"em": true, "i": true, "u": true, "s": true, "small": true,
		"h1": true, "h2": true, "h3": true, "h4": true, "h5": true, "h6": true,
		"ul": true, "ol": true, "li": true, "a": true, "img": true,
	}
	if !strict {
		for _, tag := range []string{"table", "thead", "tbody", "tr", "th", "td", "pre", "code", "blockquote", "hr", "dl", "dt", "dd", "sup", "sub"} {
			allowed[tag] = true
		}
	}
	var result strings.Builder
	tokenizer := xhtml.NewTokenizer(strings.NewReader(source))
	blockedDepth := 0
	for {
		typeOfToken := tokenizer.Next()
		token := tokenizer.Token()
		switch typeOfToken {
		case xhtml.ErrorToken:
			return result.String()
		case xhtml.TextToken:
			if blockedDepth == 0 {
				result.WriteString(html.EscapeString(token.Data))
			}
		case xhtml.StartTagToken:
			name := strings.ToLower(token.Data)
			if blockedDepth > 0 {
				if name == "script" || name == "style" || name == "iframe" || name == "form" || name == "object" || name == "svg" {
					blockedDepth++
				}
				continue
			}
			if name == "script" || name == "style" || name == "iframe" || name == "form" || name == "object" || name == "svg" {
				blockedDepth = 1
				continue
			}
			if !allowed[name] {
				continue
			}
			result.WriteByte('<')
			result.WriteString(name)
			for _, attr := range token.Attr {
				key := strings.ToLower(attr.Key)
				value := strings.TrimSpace(attr.Val)
				switch {
				case key == "class" || key == "id" || key == "title":
				case name == "a" && (key == "href" || key == "target"):
					if key == "href" && (!safeOperatorURL(value, false) || operatorContactLink(value)) {
						continue
					}
					if key == "target" && value != "_blank" && value != "_self" {
						continue
					}
				case name == "img" && (key == "src" || key == "alt" || key == "width" || key == "height"):
					if key == "src" && !safeOperatorURL(value, true) {
						continue
					}
				default:
					continue
				}
				result.WriteByte(' ')
				result.WriteString(key)
				result.WriteString(`="`)
				result.WriteString(html.EscapeString(value))
				result.WriteByte('"')
			}
			if name == "a" {
				result.WriteString(` rel="noopener noreferrer nofollow ugc"`)
			}
			result.WriteByte('>')
		case xhtml.EndTagToken:
			name := strings.ToLower(token.Data)
			if blockedDepth > 0 {
				if name == "script" || name == "style" || name == "iframe" || name == "form" || name == "object" || name == "svg" {
					blockedDepth--
				}
				continue
			}
			if allowed[name] && name != "br" && name != "img" && name != "hr" {
				result.WriteString("</")
				result.WriteString(name)
				result.WriteByte('>')
			}
		case xhtml.SelfClosingTagToken:
			name := strings.ToLower(token.Data)
			if allowed[name] && (name == "br" || name == "img" || name == "hr") {
				result.WriteString("<")
				result.WriteString(name)
				result.WriteString(" />")
			}
		}
	}
}

func safeOperatorURL(value string, image bool) bool {
	if strings.HasPrefix(value, "/") && !strings.HasPrefix(value, "//") {
		return true
	}
	parsed, err := url.Parse(value)
	if err != nil {
		return false
	}
	if image {
		return parsed.Scheme == "https" || parsed.Scheme == "data"
	}
	return parsed.Scheme == "https" || parsed.Scheme == "http" || parsed.Scheme == "mailto"
}

func operatorContactLink(value string) bool {
	parsed, err := url.Parse(value)
	if err != nil {
		return true
	}
	return strings.TrimSuffix(parsed.Path, "/") == "/server/contact"
}
