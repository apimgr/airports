package server

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"

	"github.com/apimgr/airports/src/common/i18n"
)

type accentColor struct {
	r, g, b uint8
}

func (s *Server) handleAccentCSS(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, i18n.TranslateText(i18n.FromContext(r.Context()), "errors.method_not_allowed"), http.StatusMethodNotAllowed)
		return
	}
	s.configMu.RLock()
	value := ""
	if s.config != nil {
		value = strings.TrimSpace(s.config.Web.UI.Accent)
	}
	s.configMu.RUnlock()
	base, ok := parseAccentColor(value)
	if !ok {
		http.NotFound(w, r)
		return
	}
	dark := accentForBackground(base, accentColor{r: 40, g: 42, b: 54}, true)
	light := accentForBackground(base, accentColor{r: 255, g: 255, b: 255}, false)
	css := fmt.Sprintf(`:root {
  --color-primary: %s;
  --color-primary-bg: rgba(%d, %d, %d, 0.15);
}
html.theme-light {
  --color-primary: %s;
  --color-primary-bg: rgba(%d, %d, %d, 0.12);
}
@media (prefers-color-scheme: light) {
  html.theme-auto {
    --color-primary: %s;
    --color-primary-bg: rgba(%d, %d, %d, 0.12);
  }
}
`, accentHex(dark), dark.r, dark.g, dark.b, accentHex(light), light.r, light.g, light.b, accentHex(light), light.r, light.g, light.b)
	sum := sha256.Sum256([]byte(css))
	etag := `"` + hex.EncodeToString(sum[:]) + `"`
	w.Header().Set("Content-Type", "text/css; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=300")
	w.Header().Set("ETag", etag)
	if r.Header.Get("If-None-Match") == etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	if r.Method == http.MethodHead {
		w.Header().Set("Content-Length", strconv.Itoa(len(css)))
		w.WriteHeader(http.StatusOK)
		return
	}
	_, _ = w.Write([]byte(css))
}

func parseAccentColor(value string) (accentColor, bool) {
	if len(value) != 7 || value[0] != '#' {
		return accentColor{}, false
	}
	parsed, err := strconv.ParseUint(value[1:], 16, 24)
	if err != nil {
		return accentColor{}, false
	}
	return accentColor{r: uint8(parsed >> 16), g: uint8(parsed >> 8), b: uint8(parsed)}, true
}

func accentHex(color accentColor) string {
	return fmt.Sprintf("#%02x%02x%02x", color.r, color.g, color.b)
}

func accentForBackground(base, background accentColor, lighten bool) accentColor {
	if accentContrast(base, background) >= 4.5 {
		return base
	}
	h, s, l := rgbToHSL(base)
	for step := 1; step <= 100; step++ {
		candidateLightness := l + float64(step)/100
		if !lighten {
			candidateLightness = l - float64(step)/100
		}
		if candidateLightness < 0 {
			candidateLightness = 0
		}
		if candidateLightness > 1 {
			candidateLightness = 1
		}
		candidate := hslToRGB(h, s, candidateLightness)
		if accentContrast(candidate, background) >= 4.5 || candidateLightness == 0 || candidateLightness == 1 {
			return candidate
		}
	}
	return base
}

func accentContrast(foreground, background accentColor) float64 {
	first := accentLuminance(foreground)
	second := accentLuminance(background)
	if first < second {
		first, second = second, first
	}
	return (first + 0.05) / (second + 0.05)
}

func accentLuminance(color accentColor) float64 {
	channels := []uint8{color.r, color.g, color.b}
	linear := make([]float64, len(channels))
	for index, channel := range channels {
		value := float64(channel) / 255
		if value <= 0.04045 {
			linear[index] = value / 12.92
		} else {
			linear[index] = math.Pow((value+0.055)/1.055, 2.4)
		}
	}
	return 0.2126*linear[0] + 0.7152*linear[1] + 0.0722*linear[2]
}

func rgbToHSL(color accentColor) (h, s, l float64) {
	r := float64(color.r) / 255
	g := float64(color.g) / 255
	b := float64(color.b) / 255
	maximum := max(r, g, b)
	minimum := min(r, g, b)
	delta := maximum - minimum
	l = (maximum + minimum) / 2
	if delta == 0 {
		return 0, 0, l
	}
	if l > 0.5 {
		s = delta / (2 - maximum - minimum)
	} else {
		s = delta / (maximum + minimum)
	}
	switch maximum {
	case r:
		h = (g - b) / delta
		if g < b {
			h += 6
		}
	case g:
		h = (b-r)/delta + 2
	default:
		h = (r-g)/delta + 4
	}
	return h / 6, s, l
}

func hslToRGB(h, s, l float64) accentColor {
	if s == 0 {
		value := uint8(l * 255)
		return accentColor{r: value, g: value, b: value}
	}
	var q float64
	if l < 0.5 {
		q = l * (1 + s)
	} else {
		q = l + s - l*s
	}
	p := 2*l - q
	convert := func(t float64) uint8 {
		if t < 0 {
			t++
		}
		if t > 1 {
			t--
		}
		var value float64
		switch {
		case t < 1.0/6:
			value = p + (q-p)*6*t
		case t < 1.0/2:
			value = q
		case t < 2.0/3:
			value = p + (q-p)*(2.0/3-t)*6
		default:
			value = p
		}
		return uint8(value * 255)
	}
	return accentColor{r: convert(h + 1.0/3), g: convert(h), b: convert(h - 1.0/3)}
}
