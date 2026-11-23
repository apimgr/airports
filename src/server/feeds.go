package server

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/apimgr/airports/src/common/i18n"
)

type atomFeed struct {
	XMLName xml.Name    `xml:"http://www.w3.org/2005/Atom feed"`
	Title   string      `xml:"title"`
	ID      string      `xml:"id"`
	Updated string      `xml:"updated"`
	Links   []atomLink  `xml:"link"`
	Entries []atomEntry `xml:"entry"`
}

type atomLink struct {
	Href string `xml:"href,attr"`
	Rel  string `xml:"rel,attr,omitempty"`
}

type atomEntry struct {
	Title   string     `xml:"title"`
	ID      string     `xml:"id"`
	Updated string     `xml:"updated"`
	Links   []atomLink `xml:"link"`
}

type rssFeed struct {
	XMLName xml.Name   `xml:"rss"`
	Version string     `xml:"version,attr"`
	Channel rssChannel `xml:"channel"`
}

type rssChannel struct {
	Title         string    `xml:"title"`
	Link          string    `xml:"link"`
	Description   string    `xml:"description"`
	LastBuildDate string    `xml:"lastBuildDate"`
	Items         []rssItem `xml:"item"`
}

type rssItem struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	GUID        string `xml:"guid"`
	PubDate     string `xml:"pubDate"`
	Description string `xml:"description"`
}

func (s *Server) handleAtomFeed(w http.ResponseWriter, r *http.Request) {
	base := strings.TrimSuffix(s.publicBaseURL(r), "/")
	title := s.feedTitle()
	updated := s.feedUpdated().Format(time.RFC3339)
	body, err := xml.MarshalIndent(atomFeed{
		Title:   title,
		ID:      base + "/",
		Updated: updated,
		Links:   []atomLink{{Href: base + "/", Rel: "alternate"}, {Href: base + "/feed.xml", Rel: "self"}},
		Entries: []atomEntry{},
	}, "", "  ")
	if err != nil {
		http.Error(w, i18n.TranslateText(i18n.FromContext(r.Context()), "errors.internal"), http.StatusInternalServerError)
		return
	}
	s.serveFeed(w, r, "application/atom+xml; charset=utf-8", append([]byte(xml.Header), append(body, '\n')...))
}

func (s *Server) handleRSSFeed(w http.ResponseWriter, r *http.Request) {
	base := strings.TrimSuffix(s.publicBaseURL(r), "/")
	title := s.feedTitle()
	body, err := xml.MarshalIndent(rssFeed{
		Version: "2.0",
		Channel: rssChannel{Title: title, Link: base + "/", Description: title, LastBuildDate: s.feedUpdated().Format(time.RFC1123Z), Items: []rssItem{}},
	}, "", "  ")
	if err != nil {
		http.Error(w, i18n.TranslateText(i18n.FromContext(r.Context()), "errors.internal"), http.StatusInternalServerError)
		return
	}
	s.serveFeed(w, r, "application/rss+xml; charset=utf-8", append([]byte(xml.Header), append(body, '\n')...))
}

func (s *Server) feedTitle() string {
	s.configMu.RLock()
	defer s.configMu.RUnlock()
	if s.config != nil {
		if title := strings.TrimSpace(s.config.Server.SEO.Feed.Title); title != "" {
			return title
		}
		if title := strings.TrimSpace(s.config.Server.Branding.Title); title != "" {
			return title
		}
	}
	return "airports"
}

func (s *Server) feedUpdated() time.Time {
	if built, err := time.Parse(time.RFC3339, BuildDate); err == nil && !built.IsZero() {
		return built.UTC()
	}
	return time.Unix(0, 0).UTC()
}

func (s *Server) serveFeed(w http.ResponseWriter, r *http.Request, contentType string, body []byte) {
	sum := sha256.Sum256(body)
	etag := `"` + hex.EncodeToString(sum[:]) + `"`
	lastModified := s.feedUpdated().Format(http.TimeFormat)
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "public, max-age=300")
	w.Header().Set("ETag", etag)
	w.Header().Set("Last-Modified", lastModified)
	if r.Header.Get("If-None-Match") == etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	if since, err := http.ParseTime(r.Header.Get("If-Modified-Since")); err == nil {
		if modified, err := http.ParseTime(lastModified); err == nil && !modified.After(since) {
			w.WriteHeader(http.StatusNotModified)
			return
		}
	}
	w.Header().Set("Content-Length", fmt.Sprint(len(body)))
	if r.Method == http.MethodHead {
		w.WriteHeader(http.StatusOK)
		return
	}
	_, _ = w.Write(body)
}
