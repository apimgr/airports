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
// AI.md PART 17 "Global Variables (Available in All Templates)". Onion/I2P
// variables are always empty: onion address resolution requires a live
// *tor.Manager instance not reachable from a bare *config.Config, and this
// project has no I2P subsystem at all (confirmed: no I2P code anywhere in
// src/), so fabricating those values would be a spec violation.
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
