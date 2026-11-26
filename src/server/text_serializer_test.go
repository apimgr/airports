package server

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSerializeRouteResponseShapes(t *testing.T) {
	tests := []struct {
		name    string
		route   Route
		method  string
		payload any
		want    string
	}{
		{name: "primary scalar", route: Route{Primary: "text"}, method: http.MethodGet, payload: map[string]any{"ok": true, "data": map[string]any{"text": "hello"}}, want: "hello"},
		{name: "object nested", route: Route{Response: map[string]any{"200": map[string]any{"content": map[string]any{"application/json": map[string]any{"schema": map[string]any{"properties": map[string]any{"name": nil, "stats": nil}}}}}}}, method: http.MethodGet, payload: map[string]any{"ok": true, "data": map[string]any{"stats": map[string]any{"requests_24h": 45678}, "name": "test"}}, want: "OK: Retrieved successfully\nname: test\nstats.requests_24h: 45678"},
		{name: "scalar list", route: Route{}, method: http.MethodGet, payload: map[string]any{"ok": true, "data": []any{"one", "two"}}, want: "OK: 2 items found\none\ntwo"},
		{name: "object list", route: Route{}, method: http.MethodGet, payload: map[string]any{"ok": true, "data": []any{map[string]any{"name": "A"}, map[string]any{"name": "B"}}}, want: "OK: 2 items found\nname: A\n\nname: B"},
		{name: "error", route: Route{}, method: http.MethodGet, payload: map[string]any{"ok": false, "error": "NOT_FOUND", "message": "missing"}, want: "ERROR: NOT_FOUND: missing"},
		{name: "empty operation", route: Route{Message: "Saved"}, method: http.MethodPost, payload: map[string]any{"ok": true}, want: "OK: Saved"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, ok := serializeRouteResponse(test.route, test.method, test.payload)
			if !ok || got != test.want {
				t.Fatalf("serializeRouteResponse() = %q, %v; want %q, true", got, ok, test.want)
			}
		})
	}
}

func TestBinaryRouteSkipsTextNegotiation(t *testing.T) {
	handler := textNegotiatedHandler(Route{Binary: "application/octet-stream"}, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write([]byte{0, 1, 2})
	})
	request := httptest.NewRequest(http.MethodGet, "/api/v1/file", nil)
	request.Header.Set("Accept", "text/plain")
	response := httptest.NewRecorder()
	handler(response, request)
	if response.Header().Get("Content-Type") != "application/octet-stream" || !bytes.Equal(response.Body.Bytes(), []byte{0, 1, 2}) {
		t.Fatalf("binary response changed during text negotiation: content-type=%q body=%v", response.Header().Get("Content-Type"), response.Body.Bytes())
	}
}
