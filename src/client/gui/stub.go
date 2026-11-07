//go:build !gui || freebsd || netbsd || openbsd

package gui

import (
	"context"
	"errors"
	"net/url"
)

// Config supplies the client operations so the GUI stays independent of the
// command package and can be embedded by other clients.
type Config struct {
	ServerURL string
	Token     string
	OnConnect func(context.Context, string, string) (string, error)
	OnRequest func(context.Context, string, string, string, string, url.Values, string) (string, error)
	Save      func(string, string) error
}

// IsGUIAvailable reports whether this build includes a supported native GUI.
func IsGUIAvailable() bool { return false }

// LaunchGUI reports that native GUI support is unavailable in this build.
func LaunchGUI(Config) error {
	return errors.New("native GUI is unavailable; build with -tags gui on Linux, macOS, or Windows")
}
