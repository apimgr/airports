// Package banner prints responsive startup information for the server binary.
package banner

import (
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"

	"github.com/apimgr/airports/src/common/display"
	"github.com/apimgr/airports/src/common/i18n"
	"github.com/apimgr/airports/src/common/terminal"
	"github.com/apimgr/airports/src/logging"
)

const (
	ansiReset = "\x1b[0m"
	ansiBold  = "\x1b[1m"
	ansiCyan  = "\x1b[36m"
)

// bannerConfig contains the server details shown after startup.
type bannerConfig struct {
	AppName         string
	Version         string
	CommitID        string
	BuildDate       string
	AppMode         string
	StartupDatetime string
	Debug           bool
	URLs            []string
	OnionAddress    string
	I2PAddress      string
	SMTPAddress     string
	SetupToken      string
	ForceColor      *bool
	ConfigColor     *bool
	ConfigEmoji     *bool
}

// ServerBannerInfo contains all resolved values displayed by the startup banner.
type ServerBannerInfo struct {
	AppName         string
	Version         string
	AppMode         string
	StartupDatetime string
	URLs            []string
	OnionAddress    string
	I2PAddress      string
	SMTPAddress     string
	SetupToken      string
	ForceColor      *bool
	Lang            string
}

// PrintServerStartupBanner prints the resolved startup details using the terminal's display size.
func PrintServerStartupBanner(info ServerBannerInfo) {
	printServerBanner(bannerConfig{
		AppName: info.AppName, Version: info.Version, AppMode: info.AppMode,
		StartupDatetime: info.StartupDatetime, URLs: info.URLs,
		OnionAddress: info.OnionAddress, I2PAddress: info.I2PAddress,
		SMTPAddress: info.SMTPAddress, SetupToken: info.SetupToken,
		ForceColor: info.ForceColor,
	}, info.Lang)
}

// printServerBanner writes the banner using the live terminal environment.
func printServerBanner(cfg bannerConfig, lang string) {
	printServerStartupBannerTo(os.Stdout, cfg, lang, terminal.GetTerminalSize(), display.DetectDisplayEnv())
}

// printServerStartupBannerTo writes a banner using an explicit terminal environment.
func printServerStartupBannerTo(w io.Writer, cfg bannerConfig, lang string, size terminal.TerminalSize, env display.DisplayEnv) {
	if cfg.AppName == "" {
		cfg.AppName = "Airports API Server"
	}
	if cfg.Version == "" {
		cfg.Version = "unknown"
	}

	useColor := display.ColorEnabledFor(cfg.ForceColor, cfg.ConfigColor, os.Getenv("NO_COLOR"), env)
	useIcons := display.EmojiEnabledFor(cfg.ConfigEmoji, os.Getenv("NO_COLOR"), env.TerminalType)
	var output strings.Builder
	if !useIcons {
		printPlain(&output, cfg, lang)
		printSetupToken(&output, cfg.SetupToken)
		_, _ = logging.WriteSynchronized(w, []byte(output.String()))
		return
	}

	switch {
	case size.Cols >= 80:
		printFull(&output, cfg, lang, useColor, useIcons)
	case size.Cols >= 60:
		printCompact(&output, cfg, lang, useColor, useIcons)
	case size.Cols >= 40:
		printMinimal(&output, cfg, useIcons, lang)
	default:
		printMicro(&output, cfg, useIcons, size.Cols)
	}
	printSetupToken(&output, cfg.SetupToken)
	_, _ = logging.WriteSynchronized(w, []byte(output.String()))
}

func printFull(w io.Writer, cfg bannerConfig, lang string, useColor, useIcons bool) {
	if useIcons {
		_, _ = fmt.Fprintln(w, "      __|__")
		_, _ = fmt.Fprintln(w, "--o--o--(_)--o--o--")
		_, _ = fmt.Fprintln(w, "       / |")
		_, _ = fmt.Fprintln(w)
	}
	name := cfg.AppName
	if useColor {
		name = ansiBold + ansiCyan + name + ansiReset
	}
	icon := ""
	if useIcons {
		icon = "✈  "
	}
	_, _ = fmt.Fprintf(w, "%s%s · 📦 %s\n", icon, name, cfg.Version)
	printMode(w, cfg, useIcons, lang)
	for _, value := range cfg.URLs {
		_, _ = fmt.Fprintf(w, "📡 %s: %s\n", translatedLabel(lang, "startup.listening", "Listening"), value)
	}
	printAdditionalDetails(w, cfg, useIcons, lang, false)
	if cfg.CommitID != "" || cfg.BuildDate != "" {
		_, _ = fmt.Fprintf(w, "   Commit: %s  Built: %s\n", fallback(cfg.CommitID), fallback(cfg.BuildDate))
	}
}

func printCompact(w io.Writer, cfg bannerConfig, lang string, useColor, useIcons bool) {
	name := cfg.AppName
	if useColor {
		name = ansiBold + name + ansiReset
	}
	icon := ""
	if useIcons {
		icon = "🚀 "
	}
	_, _ = fmt.Fprintf(w, "%s%s v%s\n", icon, name, cfg.Version)
	printMode(w, cfg, useIcons, lang)
	for _, value := range cfg.URLs {
		_, _ = fmt.Fprintf(w, "📡 %s: %s\n", translatedLabel(lang, "startup.listening", "Listening"), value)
	}
	printAdditionalDetails(w, cfg, useIcons, lang, true)
}

func printMinimal(w io.Writer, cfg bannerConfig, useIcons bool, lang string) {
	_, _ = fmt.Fprintf(w, "%s %s\n%s\n", cfg.AppName, cfg.Version, cfg.AppMode)
	for _, value := range cfg.URLs {
		_, _ = fmt.Fprintln(w, fqdnPort(value))
	}
	_ = useIcons
}

func printMicro(w io.Writer, cfg bannerConfig, useIcons bool, width int) {
	_ = useIcons
	line := cfg.AppName + " :" + bannerPort(cfg.URLs)
	setupMarker := " [SETUP]"
	if cfg.SetupToken != "" && len(line)+len(setupMarker) <= width {
		line += setupMarker
	}
	_, _ = fmt.Fprintln(w, line)
}

func bannerPort(urls []string) string {
	if len(urls) == 0 {
		return ""
	}
	parsed, err := url.Parse(urls[0])
	if err != nil {
		return ""
	}
	if port := parsed.Port(); port != "" {
		return port
	}
	switch parsed.Scheme {
	case "https":
		return "443"
	case "http":
		return "80"
	default:
		return ""
	}
}

func fqdnPort(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	host := parsed.Hostname()
	if strings.Contains(host, ":") {
		host = "[" + host + "]"
	}
	if port := bannerPort([]string{raw}); port != "" {
		return host + ":" + port
	}
	return host
}

func printPlain(w io.Writer, cfg bannerConfig, lang string) {
	_, _ = fmt.Fprintf(w, "%s v%s\n", cfg.AppName, cfg.Version)
	_, _ = fmt.Fprintf(w, "%s: %s\n", translatedLabel(lang, "startup.mode", "Mode"), cfg.AppMode)
	for _, value := range cfg.URLs {
		_, _ = fmt.Fprintf(w, "%s: %s\n", translatedLabel(lang, "startup.listening", "Listening"), value)
	}
	if cfg.StartupDatetime != "" {
		_, _ = fmt.Fprintf(w, "%s: %s\n", translatedLabel(lang, "startup.started", "Started"), cfg.StartupDatetime)
	}
}

func printAdditionalDetails(w io.Writer, cfg bannerConfig, useIcons bool, lang string, compact bool) {
	write := func(icon, label, value string) {
		if value == "" {
			return
		}
		if useIcons {
			_, _ = fmt.Fprintf(w, "%s %s: %s\n", icon, label, value)
		} else {
			_, _ = fmt.Fprintf(w, "%s: %s\n", label, value)
		}
	}
	startedKey, startedFallback := "startup.server_started", "Server started on"
	if compact {
		startedKey, startedFallback = "startup.started", "Started"
	}
	write("✅", translatedLabel(lang, startedKey, startedFallback), cfg.StartupDatetime)
	write("🧅", translatedLabel(lang, "startup.tor", "Tor"), cfg.OnionAddress)
	write("🔗", translatedLabel(lang, "startup.i2p", "I2P"), cfg.I2PAddress)
	write("📧", translatedLabel(lang, "startup.smtp", "SMTP"), cfg.SMTPAddress)
}

func printSetupToken(w io.Writer, token string) {
	if token != "" {
		_, _ = fmt.Fprintf(w, "Admin token (shown once): %s\n", token)
	}
}

func translatedLabel(lang, key, fallback string) string {
	if value := i18n.TranslateText(lang, key); value != "" && value != key {
		return value
	}
	return fallback
}

func printMode(w io.Writer, cfg bannerConfig, icons bool, lang string) {
	label := i18n.TranslateText(lang, "cli.running_in_mode_label")
	if label == "" || label == "cli.running_in_mode_label" {
		label = "Running in mode"
	}
	icon := ""
	if icons {
		switch cfg.AppMode {
		case "development":
			icon = "🔧 "
		case "debug":
			icon = "🐛 "
		default:
			icon = "🔒 "
		}
	}
	debug := ""
	if cfg.Debug {
		debug = " [debugging]"
	}
	_, _ = fmt.Fprintf(w, "%s%s: %s%s\n", icon, label, cfg.AppMode, debug)
}

func hostPort(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	host := parsed.Hostname()
	port := parsed.Port()
	if port == "80" || port == "443" {
		port = ""
	}
	if port != "" {
		return host + ":" + port
	}
	return host
}

func fallback(value string) string {
	if strings.TrimSpace(value) == "" {
		return "unknown"
	}
	return value
}
