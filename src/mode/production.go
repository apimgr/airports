package mode

// CacheHeaders represents HTTP cache control headers.
type CacheHeaders struct {
	CacheControl string
	Pragma       string
	Expires      string
}

// GetCacheHeaders returns mode-appropriate cache headers.
func GetCacheHeaders() CacheHeaders {
	if !IsAppModeProd() {
		return CacheHeaders{
			CacheControl: "no-cache, no-store, must-revalidate",
			Pragma:       "no-cache",
			Expires:      "0",
		}
	}
	return CacheHeaders{
		CacheControl: "public, max-age=31536000, immutable",
	}
}

// getLogLevel returns the required baseline log level for the current mode.
func getLogLevel() string {
	if IsAppModeDev() || GetAppMode() == Debug {
		return "debug"
	}
	return "info"
}

// ShouldCacheTemplates reports whether templates should be cached.
func ShouldCacheTemplates() bool {
	return IsAppModeProd()
}
