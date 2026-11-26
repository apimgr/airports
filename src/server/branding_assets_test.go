package server

import (
	"bytes"
	"context"
	"encoding/base64"
	"image"
	"image/png"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseBrandingDataURI(t *testing.T) {
	pngData := onePixelPNG(t)
	data, contentType, err := parseBrandingDataURI("data:image/png;base64," + pngData)
	if err != nil {
		t.Fatalf("parseBrandingDataURI() error = %v", err)
	}
	if contentType != "image/png" || len(data) == 0 {
		t.Fatalf("parsed data URI = (%d bytes, %q), want non-empty PNG", len(data), contentType)
	}
	if _, _, err := parseBrandingDataURI("data:text/html;base64," + base64.StdEncoding.EncodeToString([]byte("<script>"))); err == nil {
		t.Fatal("parseBrandingDataURI() accepted non-image media type")
	}
}

func TestValidatePublicHTTPSURL(t *testing.T) {
	for _, raw := range []string{
		"http://example.com/image.png",
		"https://user@example.com/image.png",
		"https://localhost/image.png",
		"https://127.0.0.1/image.png",
		"https://192.168.1.10/image.png",
	} {
		parsed, err := url.Parse(raw)
		if err != nil {
			t.Fatalf("url.Parse(%q) error = %v", raw, err)
		}
		if err := validatePublicHTTPSURL(parsed); err == nil {
			t.Errorf("validatePublicHTTPSURL(%q) succeeded; want rejection", raw)
		}
	}
	parsed, err := url.Parse("https://example.com/image.png")
	if err != nil {
		t.Fatal(err)
	}
	if err := validatePublicHTTPSURL(parsed); err != nil {
		t.Errorf("validatePublicHTTPSURL(public URL) error = %v", err)
	}
}

func TestPublicBrandingIPBlocksReservedRanges(t *testing.T) {
	for _, raw := range []string{
		"0.0.0.0", "10.0.0.1", "100.64.0.1", "127.0.0.1", "169.254.1.1",
		"172.16.0.1", "192.0.2.1", "192.168.1.1", "198.18.0.1", "198.51.100.1",
		"203.0.113.1", "224.0.0.1", "240.0.0.1", "::1", "fc00::1", "fe80::1",
	} {
		if publicBrandingIP(net.ParseIP(raw)) {
			t.Errorf("publicBrandingIP(%q) = true, want reserved address rejected", raw)
		}
	}
	for _, raw := range []string{"8.8.8.8", "1.1.1.1", "2606:4700:4700::1111"} {
		if !publicBrandingIP(net.ParseIP(raw)) {
			t.Errorf("publicBrandingIP(%q) = false, want public address accepted", raw)
		}
	}
}

func TestSafeBrandingDialRejectsLoopbackResolution(t *testing.T) {
	if _, err := safeBrandingDialContext(context.Background(), "tcp", "localhost:443"); err == nil || !strings.Contains(err.Error(), "non-public IP") {
		t.Fatalf("safeBrandingDialContext(loopback) error = %v, want non-public IP rejection", err)
	}
}

func TestBrandingAssetResolverLocalAssetAndOpaqueHandler(t *testing.T) {
	configDir := t.TempDir()
	assetsDir := filepath.Join(configDir, "assets")
	if err := os.Mkdir(assetsDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(assetsDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(assetsDir, "logo.png"), mustDecodeBase64(t, onePixelPNG(t)), 0o600); err != nil {
		t.Fatal(err)
	}
	resolver, err := NewBrandingAssetResolver(configDir, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	path, err := resolver.Resolve("logo.png")
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if !strings.HasPrefix(path, "/branding-assets/") {
		t.Fatalf("Resolve() path = %q, want opaque branding asset path", path)
	}
	request := httptest.NewRequest("GET", path, nil)
	response := httptest.NewRecorder()
	resolver.Handler().ServeHTTP(response, request)
	if response.Code != 200 || response.Header().Get("Content-Type") != "image/png" || response.Body.Len() == 0 {
		t.Fatalf("asset response = (%d, %q, %d bytes), want 200 PNG", response.Code, response.Header().Get("Content-Type"), response.Body.Len())
	}
	scaledRequest := httptest.NewRequest("GET", path+"?width=32&height=32", nil)
	scaledResponse := httptest.NewRecorder()
	resolver.Handler().ServeHTTP(scaledResponse, scaledRequest)
	if scaledResponse.Code != http.StatusOK || scaledResponse.Header().Get("ETag") == "" {
		t.Fatalf("scaled asset response = (%d, ETag %q), want 200 with ETag", scaledResponse.Code, scaledResponse.Header().Get("ETag"))
	}
	decoded, err := png.DecodeConfig(bytes.NewReader(scaledResponse.Body.Bytes()))
	if err != nil || decoded.Width != 32 || decoded.Height != 32 {
		t.Fatalf("scaled asset size = %+v, decode error = %v; want 32x32", decoded, err)
	}
	cachedResponse := httptest.NewRecorder()
	resolver.Handler().ServeHTTP(cachedResponse, scaledRequest)
	if cachedResponse.Code != http.StatusOK || !bytes.Equal(cachedResponse.Body.Bytes(), scaledResponse.Body.Bytes()) {
		t.Fatalf("cached scaled asset response = (%d, %d bytes), want matching 200 response", cachedResponse.Code, cachedResponse.Body.Len())
	}
	conditionalRequest := httptest.NewRequest("GET", path, nil)
	conditionalRequest.Header.Set("If-None-Match", response.Header().Get("ETag"))
	conditionalResponse := httptest.NewRecorder()
	resolver.Handler().ServeHTTP(conditionalResponse, conditionalRequest)
	if conditionalResponse.Code != http.StatusNotModified {
		t.Errorf("conditional asset status = %d, want 304", conditionalResponse.Code)
	}
	headResponse := httptest.NewRecorder()
	resolver.Handler().ServeHTTP(headResponse, httptest.NewRequest("HEAD", path, nil))
	if headResponse.Code != http.StatusOK || headResponse.Body.Len() != 0 {
		t.Errorf("HEAD asset response = (%d, %d bytes), want 200 with no body", headResponse.Code, headResponse.Body.Len())
	}
	methodResponse := httptest.NewRecorder()
	resolver.Handler().ServeHTTP(methodResponse, httptest.NewRequest("POST", path, nil))
	if methodResponse.Code != http.StatusMethodNotAllowed || methodResponse.Header().Get("Allow") != "GET, HEAD" {
		t.Errorf("POST asset response = (%d, Allow %q), want 405 GET, HEAD", methodResponse.Code, methodResponse.Header().Get("Allow"))
	}
	for _, invalid := range []string{"../logo.png", "nested/logo.png", "missing.png", "logo.gif"} {
		if _, err := resolver.Resolve(invalid); err == nil {
			t.Errorf("Resolve(%q) succeeded; want rejection", invalid)
		}
	}
}

func TestValidBrandingImageSizes(t *testing.T) {
	for _, size := range [][2]int{{16, 16}, {32, 32}, {48, 48}, {180, 180}, {192, 192}, {512, 512}, {1200, 630}, {200, 0}, {50, 0}} {
		if !validBrandingImageSize(size[0], size[1]) {
			t.Errorf("validBrandingImageSize(%d, %d) = false, want true", size[0], size[1])
		}
	}
	for _, size := range [][2]int{{17, 17}, {32, 16}, {1199, 630}, {0, 0}} {
		if validBrandingImageSize(size[0], size[1]) {
			t.Errorf("validBrandingImageSize(%d, %d) = true, want false", size[0], size[1])
		}
	}
}

func TestBrandingAssetHandlerDoesNotServeUnregisteredAssets(t *testing.T) {
	resolver, err := NewBrandingAssetResolver("", "")
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest("GET", "/branding-assets/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", nil)
	response := httptest.NewRecorder()
	resolver.Handler().ServeHTTP(response, request)
	if response.Code != 404 {
		t.Fatalf("unregistered asset status = %d, want 404", response.Code)
	}
}

func TestBrandingImageScalingPreservesAspectRatio(t *testing.T) {
	data := mustDecodeBase64(t, onePixelPNG(t))
	asset := brandingAsset{data: data, contentType: "image/png"}
	resized, err := resizeBrandingImage(asset, 32)
	if err != nil {
		t.Fatalf("resizeBrandingImage() error = %v", err)
	}
	if resized.contentType != "image/png" {
		t.Fatalf("resized content type = %q, want image/png", resized.contentType)
	}
	decoded, err := png.DecodeConfig(bytes.NewReader(resized.data))
	if err != nil {
		t.Fatalf("decode resized PNG = %v", err)
	}
	if decoded.Width != 32 || decoded.Height != 32 {
		t.Fatalf("resized dimensions = %dx%d, want 32x32", decoded.Width, decoded.Height)
	}
}

func mustDecodeBase64(t *testing.T, value string) []byte {
	t.Helper()
	data, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func onePixelPNG(t *testing.T) string {
	t.Helper()
	var buffer bytes.Buffer
	if err := png.Encode(&buffer, image.NewRGBA(image.Rect(0, 0, 1, 1))); err != nil {
		t.Fatal(err)
	}
	return base64.StdEncoding.EncodeToString(buffer.Bytes())
}
