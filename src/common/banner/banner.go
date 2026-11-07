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
)

const (
	ansiReset = "\x1b[0m"
	ansiBold  = "\x1b[1m"
	ansiCyan  = "\x1b[36m"
)

// BannerConfig contains the server details shown after startup.
type BannerConfig struct {
	AppName     string
	Version     string
	CommitID    string
	BuildDate   string
	AppMode     string
	Debug       bool
	URLs        []string
	ForceColor  *bool
	ConfigColor *bool
	ConfigEmoji *bool
}

// PrintServerStartupBanner prints a terminal-aware status banner.
func PrintServerStartupBanner(appName, version, appMode string, urls []string, forceColor *bool, lang string) {
	PrintServerBanner(BannerConfig{AppName: appName, Version: version, AppMode: appMode, URLs: urls, ForceColor: forceColor}, lang)
}

// PrintServerBanner prints build details alongside the server status.
func PrintServerBanner(cfg BannerConfig, lang string) {
	PrintServerStartupBannerTo(os.Stdout, cfg, lang, terminal.GetTerminalSize(), display.DetectDisplayEnv())
}

// PrintServerStartupBannerTo writes a banner using an explicit terminal environment.
func PrintServerStartupBannerTo(w io.Writer, cfg BannerConfig, lang string, size terminal.TerminalSize, env display.DisplayEnv) {
	if cfg.AppName == "" {
		cfg.AppName = "Airports API Server"
	}
	if cfg.Version == "" {
		cfg.Version = "unknown"
	}

	useColor := display.ColorEnabledFor(cfg.ForceColor, cfg.ConfigColor, os.Getenv("NO_COLOR"), env)
	useIcons := display.EmojiEnabledFor(cfg.ConfigEmoji, os.Getenv("NO_COLOR"), env.TerminalType)

	switch {
	case size.Cols >= 80:
		printFull(w, cfg, lang, useColor, useIcons)
	case size.Cols >= 60:
		printCompact(w, cfg, lang, useColor, useIcons)
	case size.Cols >= 40:
		printMinimal(w, cfg, useIcons)
	default:
		printMicro(w, cfg, useIcons)
	}
}

func printFull(w io.Writer, cfg BannerConfig, lang string, useColor, useIcons bool) {
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
	_, _ = fmt.Fprintf(w, "%s%s v%s\n", icon, name, cfg.Version)
	printMode(w, cfg, useIcons, lang)
	for _, value := range cfg.URLs {
		if useIcons {
			_, _ = fmt.Fprintf(w, "🌐 %s\n", value)
		} else {
			_, _ = fmt.Fprintf(w, "  %s\n", value)
		}
	}
	if cfg.CommitID != "" || cfg.BuildDate != "" {
		_, _ = fmt.Fprintf(w, "   Commit: %s  Built: %s\n", fallback(cfg.CommitID), fallback(cfg.BuildDate))
	}
}

func printCompact(w io.Writer, cfg BannerConfig, lang string, useColor, useIcons bool) {
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
		if useIcons {
			_, _ = fmt.Fprintf(w, "🌐 %s\n", value)
		} else {
			_, _ = fmt.Fprintf(w, "  %s\n", value)
		}
	}
}

func printMinimal(w io.Writer, cfg BannerConfig, useIcons bool) {
	icon := ""
	if useIcons {
		icon = "✈ "
	}
	_, _ = fmt.Fprintf(w, "%s%s %s\n", icon, cfg.AppName, cfg.Version)
	for _, value := range cfg.URLs {
		_, _ = fmt.Fprintln(w, hostPort(value))
	}
}

func printMicro(w io.Writer, cfg BannerConfig, useIcons bool) {
	icon := ""
	if useIcons {
		icon = "✈ "
	}
	if len(cfg.URLs) == 0 {
		_, _ = fmt.Fprintln(w, icon+cfg.AppName)
		return
	}
	_, _ = fmt.Fprintf(w, "%s%s %s\n", icon, cfg.AppName, hostPort(cfg.URLs[0]))
}

func printMode(w io.Writer, cfg BannerConfig, icons bool, lang string) {
	label := i18n.T(lang, "cli.running_in_mode_label")
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
