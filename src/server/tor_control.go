package server

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/apimgr/airports/src/tor"
)

// torControlBodyLimit bounds the request body accepted by the Tor internal
// control channel (imported key material is small; this is generous headroom
// while still refusing an unbounded read from a loopback caller).
const torControlBodyLimit = 1 << 20 // 1 MiB

// torLoopbackOnly wraps next so it only ever runs for requests whose
// immediate TCP peer is 127.0.0.1/::1, per AI.md PART 31 "CLI-to-running-
// server control channel": /server/tor/* has no legitimate remote caller,
// so it is loopback-gated rather than token-gated like /server/metrics. A
// non-loopback peer gets 404, never 403 — the endpoint must not be
// discoverable.
func torLoopbackOnly(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		host, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			host = r.RemoteAddr
		}
		ip := net.ParseIP(host)
		if ip == nil || !ip.IsLoopback() {
			http.NotFound(w, r)
			return
		}
		next(w, r)
	}
}

// torControlError maps a Manager error to the canonical JSON error envelope,
// per AI.md PART 14. tor.ErrNotEnabled always maps to 409 CONFLICT (the
// operation is well-formed but Tor is not currently configured to allow
// it); anything else is a 500 SERVER_ERROR with the detail logged, never
// echoed to the caller (AI.md PART 9/11 Tier 1 rule).
func (s *Server) torControlError(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, tor.ErrNotEnabled) {
		s.respondError(w, r, http.StatusConflict, "CONFLICT", "errors.tor_not_enabled")
		return
	}
	s.respondError(w, r, http.StatusInternalServerError, "SERVER_ERROR", "errors.internal")
}

// handleTorStatus serves GET /server/tor/status — INTERNAL, loopback-only,
// backing the `{project_name} tor status` CLI subcommand.
func (s *Server) handleTorStatus(w http.ResponseWriter, r *http.Request) {
	if s.tor == nil {
		s.respondItem(w, http.StatusOK, tor.Status{})
		return
	}
	s.respondItem(w, http.StatusOK, s.tor.Status())
}

// handleTorValidate serves POST /server/tor/validate — INTERNAL,
// loopback-only, backing `{project_name} tor validate`.
func (s *Server) handleTorValidate(w http.ResponseWriter, r *http.Request) {
	if s.tor == nil {
		s.respondError(w, r, http.StatusConflict, "CONFLICT", "errors.tor_not_enabled")
		return
	}
	if err := s.tor.Validate(); err != nil {
		s.torControlError(w, r, err)
		return
	}
	s.respondItem(w, http.StatusOK, map[string]bool{"valid": true})
}

// handleTorRestart serves POST /server/tor/restart — INTERNAL,
// loopback-only, backing `{project_name} tor restart`.
func (s *Server) handleTorRestart(w http.ResponseWriter, r *http.Request) {
	if s.tor == nil {
		s.respondError(w, r, http.StatusConflict, "CONFLICT", "errors.tor_not_enabled")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()
	if err := s.tor.Restart(ctx); err != nil {
		s.torControlError(w, r, err)
		return
	}
	s.respondItem(w, http.StatusOK, s.tor.Status())
}

// handleTorRegenerate serves POST /server/tor/regenerate — INTERNAL,
// loopback-only, backing `{project_name} tor regenerate`.
func (s *Server) handleTorRegenerate(w http.ResponseWriter, r *http.Request) {
	if s.tor == nil {
		s.respondError(w, r, http.StatusConflict, "CONFLICT", "errors.tor_not_enabled")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()
	if err := s.tor.Regenerate(ctx); err != nil {
		s.torControlError(w, r, err)
		return
	}
	s.respondItem(w, http.StatusOK, s.tor.Status())
}

// handleTorVanityStart serves POST /server/tor/vanity/start — INTERNAL,
// loopback-only, backing `{project_name} tor vanity start`. The prefix is
// read from the "prefix" query parameter or form value.
func (s *Server) handleTorVanityStart(w http.ResponseWriter, r *http.Request) {
	if s.tor == nil {
		s.respondError(w, r, http.StatusConflict, "CONFLICT", "errors.tor_not_enabled")
		return
	}
	prefix := r.URL.Query().Get("prefix")
	if prefix == "" {
		_ = r.ParseForm()
		prefix = r.FormValue("prefix")
	}
	if prefix == "" {
		s.respondError(w, r, http.StatusBadRequest, "BAD_REQUEST", "errors.tor_missing_prefix")
		return
	}
	if err := s.tor.StartVanitySearch(prefix); err != nil {
		if errors.Is(err, tor.ErrNotEnabled) {
			s.torControlError(w, r, err)
			return
		}
		s.respondError(w, r, http.StatusBadRequest, "BAD_REQUEST", "errors.validation_failed")
		return
	}
	res, _ := s.tor.VanityStatus()
	s.respondItem(w, http.StatusAccepted, res)
}

// handleTorVanityApply serves POST /server/tor/vanity/apply — INTERNAL,
// loopback-only, backing `{project_name} tor vanity apply`.
func (s *Server) handleTorVanityApply(w http.ResponseWriter, r *http.Request) {
	if s.tor == nil {
		s.respondError(w, r, http.StatusConflict, "CONFLICT", "errors.tor_not_enabled")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()
	if err := s.tor.ApplyVanity(ctx); err != nil {
		if errors.Is(err, tor.ErrNotEnabled) {
			s.torControlError(w, r, err)
			return
		}
		s.respondError(w, r, http.StatusConflict, "CONFLICT", "errors.tor_no_vanity_result")
		return
	}
	s.respondItem(w, http.StatusOK, s.tor.Status())
}

// handleTorImportKeys serves POST /server/tor/import-keys — INTERNAL,
// loopback-only, backing `{project_name} tor import-keys <path>`. The CLI
// reads the key file locally and streams its raw bytes as the request body.
func (s *Server) handleTorImportKeys(w http.ResponseWriter, r *http.Request) {
	if s.tor == nil {
		s.respondError(w, r, http.StatusConflict, "CONFLICT", "errors.tor_not_enabled")
		return
	}
	data, err := io.ReadAll(io.LimitReader(r.Body, torControlBodyLimit))
	if err != nil {
		s.respondError(w, r, http.StatusBadRequest, "BAD_REQUEST", "errors.bad_request")
		return
	}
	if len(data) == 0 {
		s.respondError(w, r, http.StatusBadRequest, "BAD_REQUEST", "errors.bad_request")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()
	if err := s.tor.ImportKeys(ctx, data); err != nil {
		if errors.Is(err, tor.ErrNotEnabled) {
			s.torControlError(w, r, err)
			return
		}
		s.respondError(w, r, http.StatusBadRequest, "BAD_REQUEST", "errors.validation_failed")
		return
	}
	s.respondItem(w, http.StatusOK, s.tor.Status())
}
