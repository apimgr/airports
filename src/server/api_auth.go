package server

import (
	"crypto/subtle"
	"net/http"
	"strings"
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
