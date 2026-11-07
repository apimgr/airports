package i18n

import (
	"os"
	"strings"
)

// ResolveOutputLanguage selects output language by CLI flag, config, locale
// environment, then English, falling back silently for unsupported values.
func ResolveOutputLanguage(flagLang, configLang string) string {
	for _, candidate := range []string{flagLang, configLang} {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" || strings.EqualFold(candidate, "auto") {
			continue
		}
		return supportedOutputLanguage(candidate)
	}
	if locale := strings.TrimSpace(os.Getenv("LC_ALL")); locale != "" {
		return supportedOutputLanguage(locale)
	}
	if locale := strings.TrimSpace(os.Getenv("LANG")); locale != "" {
		return supportedOutputLanguage(locale)
	}
	return "en"
}

func supportedOutputLanguage(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if separator := strings.IndexAny(value, "_.@-"); separator >= 0 {
		value = value[:separator]
	}
	if IsSupported(value) {
		return value
	}
	return "en"
}
