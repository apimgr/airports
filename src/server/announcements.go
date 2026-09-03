package server

import (
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/apimgr/airports/src/config"
)

// dismissedAnnouncementsCookieName is the cookie used to remember which
// announcement ids a visitor has dismissed, per AI.md PART 16 "Site Banner
// > Dismissal": a comma-separated list of announcement ids. Changing an
// announcement's id in config reshows it for everyone.
const dismissedAnnouncementsCookieName = "dismissed_announcements"

// readDismissedAnnouncementIDs parses the dismissed_announcements cookie, if
// present, into a slice of dismissed announcement ids.
func readDismissedAnnouncementIDs(r *http.Request) []string {
	c, err := r.Cookie(dismissedAnnouncementsCookieName)
	if err != nil || c.Value == "" {
		return nil
	}
	raw, err := url.QueryUnescape(c.Value)
	if err != nil || raw == "" {
		return nil
	}
	return strings.Split(raw, ",")
}

// writeDismissedAnnouncementIDs persists the given ids into the
// dismissed_announcements cookie for one year (readable by app.js so
// dismissal can also be intercepted client-side without a reload).
func writeDismissedAnnouncementIDs(w http.ResponseWriter, ids []string) {
	http.SetCookie(w, &http.Cookie{
		Name:     dismissedAnnouncementsCookieName,
		Value:    url.QueryEscape(strings.Join(ids, ",")),
		Path:     "/",
		MaxAge:   365 * 24 * 60 * 60,
		SameSite: http.SameSiteLaxMode,
	})
}

// isAnnouncementActive reports whether msg's start/end window (ISO 8601 UTC)
// contains now. An empty or unparsable bound is treated as absent (no
// restriction on that side) per AI.md PART 16 "Announcement Structure".
func isAnnouncementActive(msg config.AnnouncementMessage, now time.Time) bool {
	if msg.Start != "" {
		if start, err := time.Parse(time.RFC3339, msg.Start); err == nil && now.Before(start) {
			return false
		}
	}
	if msg.End != "" {
		if end, err := time.Parse(time.RFC3339, msg.End); err == nil && now.After(end) {
			return false
		}
	}
	return true
}

// activeAnnouncements returns the announcement messages that are enabled,
// currently within their start/end window, and not already dismissed by
// this visitor, in config order (AI.md PART 16 "Site Banner > Stacking").
func (s *Server) activeAnnouncements(r *http.Request) []config.AnnouncementMessage {
	cfg := s.config.Web.Announcements
	if !cfg.Enabled || len(cfg.Messages) == 0 {
		return nil
	}
	dismissed := readDismissedAnnouncementIDs(r)
	now := time.Now().UTC()
	active := make([]config.AnnouncementMessage, 0, len(cfg.Messages))
	for _, msg := range cfg.Messages {
		if slices.Contains(dismissed, msg.ID) {
			continue
		}
		if !isAnnouncementActive(msg, now) {
			continue
		}
		active = append(active, msg)
	}
	return active
}

// announcementIcons maps announcement type to its display glyph, matching
// the toast notification icon set (AI.md PART 16 "Toast/Notification
// Requirements > Types").
var announcementIcons = map[string]string{
	"success": "✓",
	"error":   "✗",
	"warning": "⚠",
	"info":    "ℹ",
}

// AnnouncementView is the per-request, template-ready form of an active
// announcement: Role is precomputed per AI.md PART 16 "Site Banner > ARIA"
// (role="status" for info/success, role="alert" for warning/error) and Icon
// is the display glyph for Type, so template/component/site-banner.tmpl
// stays free of Go conditionals.
type AnnouncementView struct {
	ID          string
	Type        string
	Icon        string
	Title       string
	Message     string
	Dismissible bool
	Role        string
}

// announcementViews converts this request's active announcements into their
// template-ready form (see AnnouncementView).
func (s *Server) announcementViews(r *http.Request) []AnnouncementView {
	active := s.activeAnnouncements(r)
	if len(active) == 0 {
		return nil
	}
	views := make([]AnnouncementView, 0, len(active))
	for _, msg := range active {
		role := "status"
		if msg.Type == "warning" || msg.Type == "error" {
			role = "alert"
		}
		views = append(views, AnnouncementView{
			ID:          msg.ID,
			Type:        msg.Type,
			Icon:        announcementIcons[msg.Type],
			Title:       msg.Title,
			Message:     msg.Message,
			Dismissible: msg.Dismissible,
			Role:        role,
		})
	}
	return views
}

// handleAnnouncementDismiss implements POST /announcements/dismiss per
// AI.md PART 16 "Site Banner > Dismissal": appends the submitted
// announcement id to the dismissed_announcements cookie and redirects back
// to the referring page. Works with zero JavaScript; app.js intercepts the
// form submit to skip the reload.
func (s *Server) handleAnnouncementDismiss(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		s.respondError(w, r, http.StatusBadRequest, "INVALID_REQUEST", "errors.form_parse_failed")
		return
	}
	id := r.FormValue("id")
	if id == "" {
		s.respondError(w, r, http.StatusBadRequest, "INVALID_REQUEST", "errors.form_parse_failed")
		return
	}
	ids := readDismissedAnnouncementIDs(r)
	if !slices.Contains(ids, id) {
		ids = append(ids, id)
	}
	writeDismissedAnnouncementIDs(w, ids)
	redirectBack(w, r)
}
