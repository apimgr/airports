package server

import (
	"crypto/subtle"
	"database/sql"
	"net/http"
	"strings"
	"time"
)

func bearerCredential(r *http.Request) string {
	value := strings.TrimSpace(r.Header.Get("Authorization"))
	scheme, token, ok := strings.Cut(value, " ")
	if !ok || !strings.EqualFold(scheme, "Bearer") {
		return ""
	}
	return strings.TrimSpace(token)
}

func (s *Server) validAPIBearer(r *http.Request) bool {
	provided := bearerCredential(r)
	expected := strings.TrimSpace(s.config.Server.Token)
	if provided == "" {
		return false
	}
	if expected != "" && len(provided) == len(expected) && subtle.ConstantTimeCompare([]byte(provided), []byte(expected)) == 1 {
		return true
	}
	_, _, ok := s.validateOwnerToken(provided)
	return ok
}

func (s *Server) handleAPITokenCheck(w http.ResponseWriter, r *http.Request) {
	provided := bearerCredential(r)
	if provided == "" {
		s.writeErrorJSON(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "A valid Bearer token is required.")
		return
	}
	operator := strings.TrimSpace(s.config.Server.Token)
	if operator != "" && len(provided) == len(operator) && subtle.ConstantTimeCompare([]byte(provided), []byte(operator)) == 1 {
		s.respondItem(w, http.StatusOK, map[string]string{"kind": "operator"})
		return
	}
	if s.db != nil && len(provided) == 36 && strings.HasPrefix(provided, "own_") {
		var kind string
		var expires sql.NullInt64
		var revoked sql.NullInt64
		err := s.db.QueryRow(`SELECT kind, expires_at, revoked_at FROM api_tokens WHERE token_hash = ?`, tokenHash(provided)).Scan(&kind, &expires, &revoked)
		if err == nil && !revoked.Valid && (!expires.Valid || expires.Int64 > time.Now().Unix()) {
			s.respondItem(w, http.StatusOK, map[string]string{"kind": "owner"})
			return
		}
	}
	s.writeErrorJSON(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "A valid Bearer token is required.")
}
