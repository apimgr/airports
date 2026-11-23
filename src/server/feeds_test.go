package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/apimgr/airports/src/config"
)

func TestEmptyFeedsAreValidAndConditional(t *testing.T) {
	server := &Server{config: config.DefaultApplicationConfig()}
	for _, test := range []struct {
		name        string
		handler     http.HandlerFunc
		contentType string
		rootTag     string
	}{
		{name: "atom", handler: server.handleAtomFeed, contentType: "application/atom+xml; charset=utf-8", rootTag: "<feed"},
		{name: "rss", handler: server.handleRSSFeed, contentType: "application/rss+xml; charset=utf-8", rootTag: "<rss"},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "https://example.test/", nil)
			response := httptest.NewRecorder()
			test.handler(response, request)
			if response.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
			}
			if got := response.Header().Get("Content-Type"); got != test.contentType {
				t.Errorf("Content-Type = %q, want %q", got, test.contentType)
			}
			if !strings.Contains(response.Body.String(), test.rootTag) || !strings.Contains(response.Body.String(), "https://example.test/") {
				t.Errorf("feed body lacks expected root or absolute home link: %s", response.Body.String())
			}
			if strings.Contains(response.Body.String(), "<entry>") || strings.Contains(response.Body.String(), "<item>") {
				t.Errorf("empty project feed unexpectedly contains items: %s", response.Body.String())
			}
			conditional := httptest.NewRequest(http.MethodGet, "https://example.test/", nil)
			conditional.Header.Set("If-None-Match", response.Header().Get("ETag"))
			cached := httptest.NewRecorder()
			test.handler(cached, conditional)
			if cached.Code != http.StatusNotModified {
				t.Errorf("conditional status = %d, want %d", cached.Code, http.StatusNotModified)
			}
		})
	}
}

func TestFeedsSupportHead(t *testing.T) {
	server := &Server{config: config.DefaultApplicationConfig()}
	for _, handler := range []http.HandlerFunc{server.handleAtomFeed, server.handleRSSFeed} {
		request := httptest.NewRequest(http.MethodHead, "/", nil)
		response := httptest.NewRecorder()
		handler(response, request)
		if response.Code != http.StatusOK || response.Body.Len() != 0 {
			t.Errorf("HEAD response = status %d, body length %d; want 200 with no body", response.Code, response.Body.Len())
		}
	}
}
