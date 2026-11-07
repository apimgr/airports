package notify

import (
	"fmt"
	"time"

	"github.com/apimgr/airports/src/config"
)

// defaultAppName is used when cfg.Server.Branding.Title is unset, per AI.md
// PART 17 "Sane Defaults": "From Name: {app_name} (defaults to binary name
// if not set)". The internal project name is hardcoded per binary-rules.md
// ("ALWAYS hardcode the internal project name ... for config paths").
const defaultAppName = "airports"

// GlobalVars returns the template variables available in every email per
// AI.md PART 17 "Global Variables (Available in All Templates)". Overlay
// network addresses require live Tor/I2P manager state, so callers must
// supply those values as Send overrides when the services are running.
func GlobalVars(cfg *config.Config) map[string]string {
	appName := cfg.Server.Branding.Title
	if appName == "" {
		appName = defaultAppName
	}

	fqdn := cfg.Server.FQDN
	if fqdn == "" {
		fqdn = "localhost"
	}

	scheme := "http"
	if cfg.Server.SSL.Enabled {
		scheme = "https"
	}
	appURL := fmt.Sprintf("%s://%s", scheme, fqdn)

	now := time.Now()

	return map[string]string{
		"app_name":              appName,
		"app_url":               appURL,
		"fqdn":                  fqdn,
		"onion_url":             "",
		"onion_address":         "",
		"i2p_url":               "",
		"i2p_address":           "",
		"notification_reply_to": cfg.Server.Notifications.Email.ReplyTo,
		"timestamp":             now.Format(time.RFC1123),
		"year":                  fmt.Sprintf("%d", now.Year()),
	}
}
