package server

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	"image/png"
	"io"
	"mime"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/apimgr/airports/src/common/i18n"

	_ "golang.org/x/image/webp"
)

const (
	maxBrandingAssetBytes     = 10 << 20
	maxBrandingDataURIBytes   = 256 << 10
	maxBrandingLocalFileBytes = 2 << 20
	brandingAssetCacheAge     = 24 * time.Hour
)

var brandingImageTypes = map[string]string{
	".svg":  "image/svg+xml",
	".png":  "image/png",
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".webp": "image/webp",
	".ico":  "image/x-icon",
}

type brandingAsset struct {
	data        []byte
	contentType string
	updated     time.Time
}

// BrandingAssetResolver validates configured image sources and serves
// non-data assets from an opaque, content-addressed local route.
type BrandingAssetResolver struct {
	configDir string
	cacheDir  string
	client    *http.Client
	mu        sync.RWMutex
	assets    map[string]brandingAsset
}

// NewBrandingAssetResolver creates a resolver using a private derived-data
// cache below dataDir. Empty configDir disables file and URL sources.
func NewBrandingAssetResolver(configDir, dataDir string) (*BrandingAssetResolver, error) {
	if strings.TrimSpace(configDir) == "" || strings.TrimSpace(dataDir) == "" {
		return &BrandingAssetResolver{assets: make(map[string]brandingAsset)}, nil
	}
	absolute, err := filepath.Abs(configDir)
	if err != nil {
		return nil, fmt.Errorf("resolve branding config directory: %w", err)
	}
	absoluteData, err := filepath.Abs(dataDir)
	if err != nil {
		return nil, fmt.Errorf("resolve branding cache directory: %w", err)
	}
	cacheDir := filepath.Join(absoluteData, "cache", "branding")
	if err := os.MkdirAll(cacheDir, 0o700); err != nil {
		return nil, fmt.Errorf("create branding asset cache: %w", err)
	}
	if err := os.Chmod(cacheDir, 0o700); err != nil {
		return nil, fmt.Errorf("secure branding asset cache: %w", err)
	}
	resolver := &BrandingAssetResolver{
		configDir: absolute,
		cacheDir:  cacheDir,
		assets:    make(map[string]brandingAsset),
	}
	resolver.client = &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			Proxy:       nil,
			DialContext: safeBrandingDialContext,
		},
		CheckRedirect: func(request *http.Request, via []*http.Request) error {
			if len(via) >= 3 {
				return errors.New("branding image redirect limit exceeded")
			}
			if err := validatePublicHTTPSURL(request.URL); err != nil {
				return fmt.Errorf("unsafe branding image redirect: %w", err)
			}
			return nil
		},
	}
	return resolver, nil
}

// Resolve validates a configured source and returns either its data URI or an
// opaque same-origin path to be served by this resolver's Handler.
func (resolver *BrandingAssetResolver) Resolve(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	if strings.HasPrefix(strings.ToLower(value), "data:") {
		if _, _, err := parseBrandingDataURI(value); err != nil {
			return "", err
		}
		return value, nil
	}
	if resolver == nil || resolver.configDir == "" {
		return "", errors.New("branding asset directory is unavailable")
	}
	var asset brandingAsset
	var err error
	if strings.HasPrefix(strings.ToLower(value), "https://") {
		asset, err = resolver.remoteAsset(value)
	} else {
		asset, err = resolver.localAsset(value)
	}
	if err != nil {
		return "", err
	}
	identifier := sha256.Sum256(asset.data)
	key := hex.EncodeToString(identifier[:])
	resolver.mu.Lock()
	resolver.assets[key] = asset
	resolver.mu.Unlock()
	return "/branding-assets/" + key, nil
}

// Handler serves assets previously registered by Resolve.
func (resolver *BrandingAssetResolver) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, i18n.TranslateText(i18n.FromContext(r.Context()), "errors.method_not_allowed"), http.StatusMethodNotAllowed)
			return
		}
		key := strings.TrimPrefix(r.URL.Path, "/branding-assets/")
		if len(key) != sha256.Size*2 {
			http.NotFound(w, r)
			return
		}
		if _, err := hex.DecodeString(key); err != nil {
			http.NotFound(w, r)
			return
		}
		resolver.mu.RLock()
		asset, ok := resolver.assets[key]
		resolver.mu.RUnlock()
		if !ok {
			http.NotFound(w, r)
			return
		}
		if width, err := strconv.Atoi(r.URL.Query().Get("width")); err == nil {
			height, _ := strconv.Atoi(r.URL.Query().Get("height"))
			if validBrandingImageSize(width, height) {
				if scaled, scaleErr := resolver.scaledAsset(key, asset, width, height); scaleErr == nil {
					asset = scaled
				}
			}
		}
		servedDigest := sha256.Sum256(asset.data)
		etag := `"` + hex.EncodeToString(servedDigest[:]) + `"`
		w.Header().Set("Content-Type", asset.contentType)
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Cache-Control", "public, max-age=300")
		w.Header().Set("ETag", etag)
		if r.Header.Get("If-None-Match") == etag {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("Content-Length", fmt.Sprint(len(asset.data)))
		if r.Method == http.MethodHead {
			w.WriteHeader(http.StatusOK)
			return
		}
		_, _ = w.Write(asset.data)
	})
}

func (resolver *BrandingAssetResolver) scaledAsset(key string, source brandingAsset, width, height int) (brandingAsset, error) {
	cacheKey := sha256.Sum256([]byte(key + ":" + strconv.Itoa(width) + "x" + strconv.Itoa(height)))
	cacheBase := filepath.Join(resolver.cacheDir, "scaled-"+hex.EncodeToString(cacheKey[:]))
	if resolver.cacheDir != "" {
		if cached, ok := readBrandingCache(cacheBase); ok {
			return cached, nil
		}
	}
	scaled, err := resizeBrandingImageToSize(source, width, height)
	if err != nil {
		return brandingAsset{}, err
	}
	if resolver.cacheDir != "" {
		scaled.updated = time.Now().UTC()
		if err := writeBrandingCache(cacheBase, scaled); err != nil {
			return brandingAsset{}, err
		}
	}
	return scaled, nil
}

func validBrandingImageSize(width, height int) bool {
	if height == 0 {
		return validBrandingImageWidth(width)
	}
	if width == height {
		return width == 16 || width == 32 || width == 48 || width == 180 || width == 192 || width == 512
	}
	return width == 1200 && height == 630
}

func validBrandingImageWidth(width int) bool {
	switch width {
	case 16, 32, 48, 50, 72, 96, 128, 144, 152, 180, 192, 200, 384, 512, 1200:
		return true
	default:
		return false
	}
}

func resizeBrandingImage(asset brandingAsset, width int) (brandingAsset, error) {
	return resizeBrandingImageToSize(asset, width, 0)
}

func resizeBrandingImageToSize(asset brandingAsset, width, height int) (brandingAsset, error) {
	if asset.contentType == "image/svg+xml" || asset.contentType == "image/x-icon" {
		return asset, nil
	}
	source, _, err := image.Decode(bytes.NewReader(asset.data))
	if err != nil {
		return asset, err
	}
	sourceBounds := source.Bounds()
	if sourceBounds.Dx() < 1 || sourceBounds.Dy() < 1 {
		return asset, errors.New("invalid branding image dimensions")
	}
	if height <= 0 {
		height = max(1, sourceBounds.Dy()*width/sourceBounds.Dx())
	}
	resized := image.NewRGBA(image.Rect(0, 0, width, height))
	scale := max(float64(width)/float64(sourceBounds.Dx()), float64(height)/float64(sourceBounds.Dy()))
	scaledWidth := int(float64(sourceBounds.Dx())*scale + 0.5)
	scaledHeight := int(float64(sourceBounds.Dy())*scale + 0.5)
	cropX := max(0, (scaledWidth-width)/2)
	cropY := max(0, (scaledHeight-height)/2)
	for y := 0; y < height; y++ {
		sourceY := sourceBounds.Min.Y + min(sourceBounds.Dy()-1, int(float64(cropY+y)/scale))
		for x := 0; x < width; x++ {
			sourceX := sourceBounds.Min.X + min(sourceBounds.Dx()-1, int(float64(cropX+x)/scale))
			resized.Set(x, y, source.At(sourceX, sourceY))
		}
	}
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, resized); err != nil {
		return asset, err
	}
	asset.data = encoded.Bytes()
	asset.contentType = "image/png"
	return asset, nil
}

func (resolver *BrandingAssetResolver) localAsset(name string) (brandingAsset, error) {
	if filepath.Base(name) != name || name == "." || name == ".." || strings.ContainsAny(name, `/\\`) {
		return brandingAsset{}, errors.New("branding asset must be a file name without a path")
	}
	extension := strings.ToLower(filepath.Ext(name))
	contentType, ok := brandingImageTypes[extension]
	if !ok {
		return brandingAsset{}, errors.New("unsupported branding image type")
	}
	path := filepath.Join(resolver.configDir, "assets", name)
	assetsInfo, err := os.Lstat(filepath.Dir(path))
	if err != nil || !assetsInfo.IsDir() || assetsInfo.Mode().Perm() != 0o700 {
		return brandingAsset{}, errors.New("branding assets directory must exist with mode 0700")
	}
	info, err := os.Lstat(path)
	if err != nil {
		return brandingAsset{}, fmt.Errorf("read branding asset: %w", err)
	}
	if !info.Mode().IsRegular() || info.Mode().Perm() != 0o600 || info.Size() > maxBrandingLocalFileBytes {
		return brandingAsset{}, errors.New("branding asset must be a regular file no larger than 2 MiB")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return brandingAsset{}, fmt.Errorf("read branding asset: %w", err)
	}
	if err := validateImageData(data, contentType); err != nil {
		return brandingAsset{}, err
	}
	return brandingAsset{data: data, contentType: contentType, updated: info.ModTime()}, nil
}

func (resolver *BrandingAssetResolver) remoteAsset(raw string) (result brandingAsset, resultErr error) {
	parsed, err := url.Parse(raw)
	if err != nil {
		return brandingAsset{}, fmt.Errorf("parse branding image URL: %w", err)
	}
	if err := validatePublicHTTPSURL(parsed); err != nil {
		return brandingAsset{}, err
	}
	cacheKey := sha256.Sum256([]byte(raw))
	cacheBase := filepath.Join(resolver.cacheDir, hex.EncodeToString(cacheKey[:]))
	if asset, ok := readBrandingCache(cacheBase); ok && time.Since(asset.updated) < brandingAssetCacheAge {
		return asset, nil
	}
	request, err := http.NewRequestWithContext(context.Background(), http.MethodGet, raw, nil)
	if err != nil {
		return brandingAsset{}, fmt.Errorf("create branding image request: %w", err)
	}
	request.Header.Set("Accept", "image/png, image/jpeg, image/webp, image/x-icon, image/svg+xml")
	response, err := resolver.client.Do(request)
	if err != nil {
		if asset, ok := readBrandingCache(cacheBase); ok {
			return asset, nil
		}
		return brandingAsset{}, fmt.Errorf("fetch branding image: %w", err)
	}
	defer func() {
		if err := response.Body.Close(); err != nil {
			resultErr = errors.Join(resultErr, fmt.Errorf("close branding image response: %w", err))
		}
	}()
	if response.StatusCode != http.StatusOK {
		return brandingAsset{}, fmt.Errorf("branding image fetch returned %s", response.Status)
	}
	mediaType, _, err := mime.ParseMediaType(response.Header.Get("Content-Type"))
	if err != nil || !isBrandingImageType(mediaType) {
		return brandingAsset{}, errors.New("branding image response has an unsupported content type")
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, maxBrandingAssetBytes+1))
	if err != nil {
		return brandingAsset{}, fmt.Errorf("read branding image response: %w", err)
	}
	if len(data) == 0 || len(data) > maxBrandingAssetBytes {
		return brandingAsset{}, errors.New("branding image response is empty or exceeds 10 MiB")
	}
	if err := validateImageData(data, mediaType); err != nil {
		return brandingAsset{}, err
	}
	asset := brandingAsset{data: data, contentType: mediaType, updated: time.Now().UTC()}
	if err := writeBrandingCache(cacheBase, asset); err != nil {
		return brandingAsset{}, fmt.Errorf("cache branding image: %w", err)
	}
	return asset, nil
}

func readBrandingCache(base string) (brandingAsset, bool) {
	data, err := os.ReadFile(base + ".data")
	if err != nil || len(data) == 0 || len(data) > maxBrandingAssetBytes {
		return brandingAsset{}, false
	}
	metadata, err := os.ReadFile(base + ".meta")
	if err != nil {
		return brandingAsset{}, false
	}
	fields := strings.SplitN(string(metadata), "\n", 2)
	if len(fields) != 2 || !isBrandingImageType(fields[0]) {
		return brandingAsset{}, false
	}
	updated, err := time.Parse(time.RFC3339, fields[1])
	if err != nil {
		return brandingAsset{}, false
	}
	return brandingAsset{data: data, contentType: fields[0], updated: updated}, true
}

func writeBrandingCache(base string, asset brandingAsset) (resultErr error) {
	dataFile, err := os.CreateTemp(filepath.Dir(base), ".branding-asset-*.tmp")
	if err != nil {
		return err
	}
	dataTemp := dataFile.Name()
	defer func() {
		if err := os.Remove(dataTemp); err != nil && !errors.Is(err, os.ErrNotExist) {
			resultErr = errors.Join(resultErr, fmt.Errorf("remove temporary branding image: %w", err))
		}
	}()
	if err := dataFile.Chmod(0o600); err != nil {
		_ = dataFile.Close()
		return err
	}
	if _, err := dataFile.Write(asset.data); err != nil {
		_ = dataFile.Close()
		return err
	}
	if err := dataFile.Close(); err != nil {
		return err
	}
	if err := os.Rename(dataTemp, base+".data"); err != nil {
		return err
	}
	metadata := asset.contentType + "\n" + asset.updated.Format(time.RFC3339)
	metaFile, err := os.CreateTemp(filepath.Dir(base), ".branding-meta-*.tmp")
	if err != nil {
		return err
	}
	metaTemp := metaFile.Name()
	defer func() {
		if err := os.Remove(metaTemp); err != nil && !errors.Is(err, os.ErrNotExist) {
			resultErr = errors.Join(resultErr, fmt.Errorf("remove temporary branding metadata: %w", err))
		}
	}()
	if err := metaFile.Chmod(0o600); err != nil {
		_ = metaFile.Close()
		return err
	}
	if _, err := metaFile.WriteString(metadata); err != nil {
		_ = metaFile.Close()
		return err
	}
	if err := metaFile.Close(); err != nil {
		return err
	}
	return os.Rename(metaTemp, base+".meta")
}

func parseBrandingDataURI(raw string) ([]byte, string, error) {
	if len(raw) > maxBrandingDataURIBytes*3+128 {
		return nil, "", errors.New("branding data URI is too large")
	}
	comma := strings.IndexByte(raw, ',')
	if comma < 0 {
		return nil, "", errors.New("malformed branding data URI")
	}
	metadata := strings.Split(raw[5:comma], ";")
	contentType := strings.ToLower(strings.TrimSpace(metadata[0]))
	if !isBrandingImageType(contentType) {
		return nil, "", errors.New("unsupported branding data URI type")
	}
	var data []byte
	var err error
	if len(metadata) > 1 && strings.EqualFold(metadata[len(metadata)-1], "base64") {
		data, err = base64.StdEncoding.DecodeString(raw[comma+1:])
	} else {
		var decoded string
		decoded, err = url.PathUnescape(raw[comma+1:])
		data = []byte(decoded)
	}
	if err != nil {
		return nil, "", fmt.Errorf("decode branding data URI: %w", err)
	}
	if len(data) == 0 || len(data) > maxBrandingDataURIBytes {
		return nil, "", errors.New("branding data URI is empty or exceeds 256 KiB")
	}
	if err := validateImageData(data, contentType); err != nil {
		return nil, "", err
	}
	return data, contentType, nil
}

func validateImageData(data []byte, contentType string) error {
	if len(data) == 0 {
		return errors.New("branding image is empty")
	}
	if contentType == "image/svg+xml" {
		lower := strings.ToLower(strings.TrimSpace(string(data[:min(len(data), 1024)])))
		if !strings.Contains(lower, "<svg") {
			return errors.New("branding SVG image is malformed")
		}
		return nil
	}
	detected := http.DetectContentType(data[:min(len(data), 512)])
	mediaType, _, _ := mime.ParseMediaType(detected)
	if contentType == "image/x-icon" {
		if len(data) < 4 || data[0] != 0 || data[1] != 0 || data[2] != 1 || data[3] != 0 {
			return errors.New("branding icon image is malformed")
		}
		return nil
	}
	if contentType == "image/webp" {
		if len(data) < 12 || string(data[:4]) != "RIFF" || string(data[8:12]) != "WEBP" {
			return errors.New("branding WebP image is malformed")
		}
		return nil
	}
	if mediaType != contentType {
		return fmt.Errorf("branding image bytes do not match content type %s", contentType)
	}
	return nil
}

func isBrandingImageType(value string) bool {
	switch strings.ToLower(value) {
	case "image/png", "image/jpeg", "image/webp", "image/x-icon", "image/svg+xml":
		return true
	default:
		return false
	}
}

func validatePublicHTTPSURL(parsed *url.URL) error {
	if parsed == nil || parsed.Scheme != "https" || parsed.Hostname() == "" || parsed.User != nil {
		return errors.New("branding image URL must be an absolute HTTPS URL without user information")
	}
	if net.ParseIP(parsed.Hostname()) != nil {
		ip := net.ParseIP(parsed.Hostname())
		if !publicBrandingIP(ip) {
			return errors.New("branding image URL resolves to a non-public IP")
		}
		return nil
	}
	host := strings.ToLower(strings.TrimSuffix(parsed.Hostname(), "."))
	if host == "localhost" || strings.HasSuffix(host, ".localhost") || strings.HasSuffix(host, ".local") || strings.HasSuffix(host, ".internal") || strings.HasSuffix(host, ".test") {
		return errors.New("branding image URL uses a local or reserved host")
	}
	return nil
}

func safeBrandingDialContext(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, fmt.Errorf("parse branding image address: %w", err)
	}
	resolver := net.DefaultResolver
	ips, err := resolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, fmt.Errorf("resolve branding image host: %w", err)
	}
	if len(ips) == 0 {
		return nil, errors.New("branding image host resolved to no addresses")
	}
	for _, candidate := range ips {
		if !publicBrandingIP(candidate.IP) {
			return nil, errors.New("branding image host resolves to a non-public IP")
		}
	}
	dialer := net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}
	var lastErr error
	for _, candidate := range ips {
		connection, dialErr := dialer.DialContext(ctx, network, net.JoinHostPort(candidate.IP.String(), port))
		if dialErr == nil {
			return connection, nil
		}
		lastErr = dialErr
	}
	return nil, fmt.Errorf("connect to branding image host: %w", lastErr)
}

func publicBrandingIP(ip net.IP) bool {
	if ip == nil || ip.IsUnspecified() || ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsMulticast() {
		return false
	}
	if ip4 := ip.To4(); ip4 != nil {
		parsed := netip.AddrFrom4([4]byte{ip4[0], ip4[1], ip4[2], ip4[3]})
		for _, prefix := range []string{"0.0.0.0/8", "10.0.0.0/8", "100.64.0.0/10", "127.0.0.0/8", "169.254.0.0/16", "172.16.0.0/12", "192.0.0.0/24", "192.0.2.0/24", "192.168.0.0/16", "198.18.0.0/15", "198.51.100.0/24", "203.0.113.0/24", "224.0.0.0/4", "240.0.0.0/4"} {
			if parsed.IsValid() && netip.MustParsePrefix(prefix).Contains(parsed) {
				return false
			}
		}
		return true
	}
	parsed, ok := netip.AddrFromSlice(ip)
	if !ok {
		return false
	}
	for _, prefix := range []string{"2001:db8::/32", "2001::/23"} {
		if netip.MustParsePrefix(prefix).Contains(parsed) {
			return false
		}
	}
	return parsed.IsGlobalUnicast()
}
