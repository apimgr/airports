package server

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/apimgr/airports/src/config"
)

func TestIsAnnouncementActive(t *testing.T) {
	now := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name string
		msg  config.AnnouncementMessage
		want bool
	}{
		{"no window", config.AnnouncementMessage{}, true},
		{"within window", config.AnnouncementMessage{Start: "2026-01-01T00:00:00Z", End: "2026-01-31T00:00:00Z"}, true},
		{"before start", config.AnnouncementMessage{Start: "2026-02-01T00:00:00Z"}, false},
		{"after end", config.AnnouncementMessage{End: "2026-01-01T00:00:00Z"}, false},
		{"unparsable bounds ignored", config.AnnouncementMessage{Start: "not-a-date", End: "also-not-a-date"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isAnnouncementActive(tt.msg, now); got != tt.want {
				t.Errorf("isAnnouncementActive(%+v) = %v, want %v", tt.msg, got, tt.want)
			}
		})
	}
}

func TestActiveAnnouncementsDisabledOrEmpty(t *testing.T) {
	s := newTestServer(t)
	s.config.Web.Announcements = config.AnnouncementsConfig{Enabled: false, Messages: []config.AnnouncementMessage{
		{ID: "a", Type: "info"},
	}}
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	if got := s.activeAnnouncements(r); got != nil {
		t.Errorf("activeAnnouncements() with Enabled=false = %v, want nil", got)
	}

	s.config.Web.Announcements = config.AnnouncementsConfig{Enabled: true, Messages: nil}
	if got := s.activeAnnouncements(r); got != nil {
		t.Errorf("activeAnnouncements() with empty Messages = %v, want nil", got)
	}
}

func TestActiveAnnouncementsFiltersDismissedAndExpired(t *testing.T) {
	s := newTestServer(t)
	s.config.Web.Announcements = config.AnnouncementsConfig{
		Enabled: true,
		Messages: []config.AnnouncementMessage{
			{ID: "keep", Type: "info"},
			{ID: "dismissed", Type: "warning"},
			{ID: "expired", Type: "error", End: "2000-01-01T00:00:00Z"},
		},
	}

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.AddCookie(&http.Cookie{Name: dismissedAnnouncementsCookieName, Value: url.QueryEscape("dismissed")})

	active := s.activeAnnouncements(r)
	if len(active) != 1 || active[0].ID != "keep" {
		t.Fatalf("activeAnnouncements() = %+v, want only %q", active, "keep")
	}
}

func TestAnnouncementViewsRoleAndIcon(t *testing.T) {
	s := newTestServer(t)
	s.config.Web.Announcements = config.AnnouncementsConfig{
		Enabled: true,
		Messages: []config.AnnouncementMessage{
			{ID: "i", Type: "info", Title: "Heads up", Message: "hello", Dismissible: true},
			{ID: "w", Type: "warning", Message: "careful"},
		},
	}
	r := httptest.NewRequest(http.MethodGet, "/", nil)

	views := s.announcementViews(r)
	if len(views) != 2 {
		t.Fatalf("announcementViews() returned %d views, want 2", len(views))
	}
	if views[0].Role != "status" || views[0].Icon != "ℹ" || !views[0].Dismissible {
		t.Errorf("info view = %+v, want role=status icon=ℹ dismissible=true", views[0])
	}
	if views[1].Role != "alert" || views[1].Icon != "⚠" {
		t.Errorf("warning view = %+v, want role=alert icon=⚠", views[1])
	}
}

func TestAnnouncementViewsEmptyReturnsNil(t *testing.T) {
	s := newTestServer(t)
	s.config.Web.Announcements = config.AnnouncementsConfig{Enabled: false}
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	if got := s.announcementViews(r); got != nil {
		t.Errorf("announcementViews() = %v, want nil", got)
	}
}

func TestHandleAnnouncementDismissSetsCookieAndRedirects(t *testing.T) {
	s := newTestServer(t)

	form := url.Values{"id": {"maintenance-1"}}
	r := httptest.NewRequest(http.MethodPost, "/announcements/dismiss", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.Header.Set("Referer", "/search")
	w := httptest.NewRecorder()

	s.handleAnnouncementDismiss(w, r)

	resp := w.Result()
	if resp.StatusCode != http.StatusFound && resp.StatusCode != http.StatusSeeOther {
		t.Errorf("status = %d, want a redirect", resp.StatusCode)
	}

	var cookieValue string
	for _, c := range resp.Cookies() {
		if c.Name == dismissedAnnouncementsCookieName {
			cookieValue = c.Value
		}
	}
	unescaped, err := url.QueryUnescape(cookieValue)
	if err != nil {
		t.Fatalf("QueryUnescape(%q): %v", cookieValue, err)
	}
	if !strings.Contains(unescaped, "maintenance-1") {
		t.Errorf("dismissed_announcements cookie = %q, want it to contain %q", unescaped, "maintenance-1")
	}
}

func TestHandleAnnouncementDismissRequiresID(t *testing.T) {
	s := newTestServer(t)

	r := httptest.NewRequest(http.MethodPost, "/announcements/dismiss", strings.NewReader(""))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	s.handleAnnouncementDismiss(w, r)

	if w.Result().StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Result().StatusCode, http.StatusBadRequest)
	}
}
