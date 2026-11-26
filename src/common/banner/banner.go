// Package banner prints responsive startup information for client binaries.
package banner

import (
	"fmt"
	"os"

	"github.com/apimgr/airports/src/common/display"
	"github.com/apimgr/airports/src/common/terminal"
)

// BannerConfig contains the client details shown at startup.
type BannerConfig struct {
	AppName     string
	Version     string
	AppMode     string
	Debug       bool
	URLs        []string
	ForceColor  *bool
	ConfigColor *bool
	ConfigEmoji *bool
}

// PrintStartupBanner writes a banner suited to the current terminal width.
func PrintStartupBanner(cfg BannerConfig) {
	size := terminal.GetTerminalSize()
	if cfg.AppName == "" {
		cfg.AppName = "Airports"
	}
	if cfg.Version == "" {
		cfg.Version = "unknown"
	}
	env := display.DetectDisplayEnv()
	useIcons := display.EmojiEnabledFor(cfg.ConfigEmoji, os.Getenv("NO_COLOR"), env.TerminalType)
	useColor := display.ColorEnabledFor(cfg.ForceColor, cfg.ConfigColor, os.Getenv("NO_COLOR"), env)
	switch {
	case size.Cols >= 80:
		if useIcons {
			fmt.Println("      __|__")
			fmt.Println("--o--o--(_)--o--o--")
			fmt.Println("       / |")
			fmt.Println()
		}
		name := cfg.AppName
		if useColor {
			name = "\x1b[1m" + name + "\x1b[0m"
		}
		icon := ""
		if useIcons {
			icon = "🚀 "
		}
		fmt.Printf("%s%s v%s\n", icon, name, cfg.Version)
		if cfg.AppMode != "" {
			fmt.Printf("Running in mode: %s\n", cfg.AppMode)
		}
		for _, address := range cfg.URLs {
			prefix := "  "
			if useIcons {
				prefix = "🌐 "
			}
			fmt.Printf("%s%s\n", prefix, address)
		}
	case size.Cols >= 60:
		icon := ""
		if useIcons {
			icon = "🚀 "
		}
		fmt.Printf("%s%s v%s\n", icon, cfg.AppName, cfg.Version)
		for _, address := range cfg.URLs {
			fmt.Printf("%s%s\n", icon, address)
		}
	case size.Cols >= 40:
		fmt.Printf("%s %s\n", cfg.AppName, cfg.Version)
	default:
		fmt.Println(cfg.AppName)
	}
}
