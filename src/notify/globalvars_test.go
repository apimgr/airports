package notify

import (
	"testing"

	"github.com/apimgr/airports/src/config"
)

func TestGlobalVarsDefaults(t *testing.T) {
	cfg := &config.Config{}
	vars := GlobalVars(cfg)

	if vars["app_name"] != defaultAppName {
		t.Errorf("app_name = %q, want %q", vars["app_name"], defaultAppName)
	}
	if vars["fqdn"] != "localhost" {
		t.Errorf("fqdn = %q, want localhost", vars["fqdn"])
	}
	if vars["app_url"] != "http://localhost" {
		t.Errorf("app_url = %q, want http://localhost", vars["app_url"])
	}
	for _, key := range []string{"onion_url", "onion_address", "i2p_url", "i2p_address"} {
		if vars[key] != "" {
			t.Errorf("%s = %q, want empty", key, vars[key])
		}
	}
	if vars["year"] == "" {
		t.Error("year is empty")
	}
	if vars["timestamp"] == "" {
		t.Error("timestamp is empty")
	}
}

func TestGlobalVarsCustomBrandingAndSSL(t *testing.T) {
	cfg := &config.Config{}
	cfg.Server.Branding.Title = "My Airports"
	cfg.Server.FQDN = "example.com"
	cfg.Server.SSL.Enabled = true
	cfg.Server.Notifications.Email.ReplyTo = "reply@example.com"

	vars := GlobalVars(cfg)

	if vars["app_name"] != "My Airports" {
		t.Errorf("app_name = %q", vars["app_name"])
	}
	if vars["fqdn"] != "example.com" {
		t.Errorf("fqdn = %q", vars["fqdn"])
	}
	if vars["app_url"] != "https://example.com" {
		t.Errorf("app_url = %q, want https scheme when SSL enabled", vars["app_url"])
	}
	if vars["notification_reply_to"] != "reply@example.com" {
		t.Errorf("notification_reply_to = %q", vars["notification_reply_to"])
	}
}
