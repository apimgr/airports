package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAPITokenCheckOperator(t *testing.T) {
	server := contactTestServer(t)
	server.config.Server.Token = "adm_test-token"
	request := httptest.NewRequest(http.MethodGet, "/api/v1/server/auth/check", nil)
	request.Header.Set("Authorization", "Bearer adm_test-token")
	recorder := httptest.NewRecorder()
	server.handleAPITokenCheck(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if !strings.Contains(recorder.Body.String(), `"kind": "operator"`) {
		t.Fatalf("response = %s, want operator kind", recorder.Body.String())
	}
	if strings.Contains(recorder.Body.String(), "adm_test-token") {
		t.Fatal("response echoed the bearer token")
	}
}

func TestAPITokenCheckRejectsInvalidToken(t *testing.T) {
	server := contactTestServer(t)
	server.config.Server.Token = "adm_test-token"
	request := httptest.NewRequest(http.MethodGet, "/api/v1/server/auth/check", nil)
	request.Header.Set("Authorization", "Bearer invalid")
	recorder := httptest.NewRecorder()
	server.handleAPITokenCheck(recorder, request)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusUnauthorized)
	}
	if strings.Contains(recorder.Body.String(), "invalid") {
		t.Fatal("response echoed the invalid bearer token")
	}
}
