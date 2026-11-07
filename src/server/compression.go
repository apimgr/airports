package server

import (
	"bufio"
	"compress/gzip"
	"log"
	"mime"
	"net"
	"net/http"
	"strconv"
	"strings"

	"github.com/apimgr/airports/src/config"
)

func gzipCompressionMiddleware(settings func() config.CompressionConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			current := config.CompressionConfig{}
			if settings != nil {
				current = settings()
			}
			if !current.Enabled {
				next.ServeHTTP(w, r)
				return
			}
			writer := &gzipResponseWriter{ResponseWriter: w, request: r, settings: current}
			defer writer.close()
			next.ServeHTTP(writer, r)
		})
	}
}

type gzipResponseWriter struct {
	http.ResponseWriter
	request    *http.Request
	settings   config.CompressionConfig
	status     int
	committed  bool
	compressor *gzip.Writer
}

func (w *gzipResponseWriter) WriteHeader(status int) {
	if w.committed {
		return
	}
	if status >= 100 && status < 200 && status != http.StatusSwitchingProtocols {
		w.ResponseWriter.WriteHeader(status)
		return
	}
	w.status = status
	if status == http.StatusSwitchingProtocols {
		w.commit(false)
	}
}

func (w *gzipResponseWriter) Write(body []byte) (int, error) {
	if !w.committed {
		if w.Header().Get("Content-Type") == "" && len(body) > 0 {
			w.Header().Set("Content-Type", http.DetectContentType(body))
		}
		w.commit(w.shouldCompress())
	}
	if w.compressor != nil {
		return w.compressor.Write(body)
	}
	return w.ResponseWriter.Write(body)
}

func (w *gzipResponseWriter) shouldCompress() bool {
	status := w.status
	if status == 0 {
		status = http.StatusOK
	}
	if w.request.Method == http.MethodHead || status == http.StatusPartialContent || status < 200 || status == http.StatusNoContent || status == http.StatusNotModified {
		return false
	}
	if w.Header().Get("Content-Encoding") != "" || w.Header().Get("Content-Range") != "" || w.Header().Get("Upgrade") != "" || connectionHasToken(w.request.Header.Get("Connection"), "upgrade") {
		return false
	}
	if connectionHasToken(w.Header().Get("Cache-Control"), "no-transform") {
		return false
	}
	if !acceptsGzip(strings.Join(w.request.Header.Values("Accept-Encoding"), ",")) {
		return false
	}
	mediaType, _, err := mime.ParseMediaType(w.Header().Get("Content-Type"))
	if err != nil {
		return false
	}
	mediaType = strings.ToLower(mediaType)
	for _, configured := range w.settings.Types {
		configured = strings.ToLower(strings.TrimSpace(configured))
		if configured == mediaType || strings.HasSuffix(configured, "/*") && strings.HasPrefix(mediaType, strings.TrimSuffix(configured, "*")) {
			return true
		}
	}
	return false
}

func (w *gzipResponseWriter) commit(compress bool) {
	if w.committed {
		return
	}
	w.committed = true
	if w.status == 0 {
		w.status = http.StatusOK
	}
	addVary(w.Header(), "Accept-Encoding")
	if compress {
		level := w.settings.Level
		if level < gzip.HuffmanOnly || level > gzip.BestCompression || level == 0 {
			level = 5
		}
		compressor, err := gzip.NewWriterLevel(w.ResponseWriter, level)
		if err == nil {
			w.compressor = compressor
			w.Header().Set("Content-Encoding", "gzip")
			w.Header().Del("Content-Length")
			w.Header().Del("ETag")
		} else {
			log.Printf("warning: cannot initialize gzip response: %v", err)
		}
	}
	w.ResponseWriter.WriteHeader(w.status)
}

func (w *gzipResponseWriter) Flush() {
	if !w.committed {
		w.commit(w.shouldCompress())
	}
	if w.compressor != nil {
		if err := w.compressor.Flush(); err != nil {
			log.Printf("warning: cannot flush gzip response: %v", err)
		}
	}
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (w *gzipResponseWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func (w *gzipResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hijacker, ok := w.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, http.ErrNotSupported
	}
	conn, buffered, err := hijacker.Hijack()
	if err == nil {
		w.committed = true
	}
	return conn, buffered, err
}

func (w *gzipResponseWriter) Push(target string, options *http.PushOptions) error {
	pusher, ok := w.ResponseWriter.(http.Pusher)
	if !ok {
		return http.ErrNotSupported
	}
	return pusher.Push(target, options)
}

func (w *gzipResponseWriter) close() {
	if !w.committed {
		w.commit(false)
	}
	if w.compressor != nil {
		if err := w.compressor.Close(); err != nil {
			log.Printf("warning: cannot finish gzip response: %v", err)
		}
	}
}

func acceptsGzip(header string) bool {
	var wildcardQuality float64
	var gzipQuality float64
	gzipSpecified := false
	for _, item := range strings.Split(header, ",") {
		parts := strings.Split(item, ";")
		encoding := strings.ToLower(strings.TrimSpace(parts[0]))
		quality := 1.0
		for _, parameter := range parts[1:] {
			key, value, found := strings.Cut(strings.TrimSpace(parameter), "=")
			if found && strings.EqualFold(strings.TrimSpace(key), "q") {
				parsed, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
				if err != nil || parsed < 0 || parsed > 1 {
					quality = 0
				} else {
					quality = parsed
				}
			}
		}
		switch encoding {
		case "gzip":
			gzipSpecified = true
			gzipQuality = quality
		case "*":
			wildcardQuality = quality
		}
	}
	if gzipSpecified {
		return gzipQuality > 0
	}
	return wildcardQuality > 0
}

func connectionHasToken(header, wanted string) bool {
	for _, token := range strings.Split(header, ",") {
		if strings.EqualFold(strings.TrimSpace(token), wanted) {
			return true
		}
	}
	return false
}

func addVary(header http.Header, value string) {
	for _, existing := range header.Values("Vary") {
		for _, token := range strings.Split(existing, ",") {
			if strings.TrimSpace(token) == "*" || strings.EqualFold(strings.TrimSpace(token), value) {
				return
			}
		}
	}
	header.Add("Vary", value)
}

var _ http.Flusher = (*gzipResponseWriter)(nil)
var _ http.Hijacker = (*gzipResponseWriter)(nil)
var _ http.Pusher = (*gzipResponseWriter)(nil)
