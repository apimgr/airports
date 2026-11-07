package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/apimgr/airports/src/config"
	"github.com/apimgr/airports/src/mode"
	"github.com/apimgr/airports/src/service"
	"gopkg.in/yaml.v3"
)

func TestAuthorizeMaintenanceModeChange(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Server.User = "airports"
	cfg.Server.Token = "operator-token"
	tests := []struct {
		name        string
		cfg         *config.Config
		root        bool
		username    string
		token       string
		wantAllowed bool
	}{
		{name: "root is authorized without token", cfg: cfg, root: true, username: "arbitrary-user", wantAllowed: true},
		{name: "service user with valid token", cfg: cfg, username: "airports", token: "operator-token", wantAllowed: true},
		{name: "service user without token", cfg: cfg, username: "airports"},
		{name: "service user with invalid token", cfg: cfg, username: "airports", token: "wrong-token"},
		{name: "other user with valid token", cfg: cfg, username: "other", token: "operator-token"},
		{name: "missing config", username: "airports", token: "operator-token"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := authorizeMaintenanceModeChange(tt.cfg, tt.root, tt.username, tt.token)
			if (err == nil) != tt.wantAllowed {
				t.Fatalf("authorizeMaintenanceModeChange() error = %v, wantAllowed=%t", err, tt.wantAllowed)
			}
		})
	}
}

func TestIsConfiguredServiceUserDefaultsToProjectName(t *testing.T) {
	cfg := config.DefaultConfig()
	if !isConfiguredServiceUser(cfg, ProjectName) {
		t.Fatalf("isConfiguredServiceUser() = false, want true for default service account %q", ProjectName)
	}
	if isConfiguredServiceUser(cfg, "unrelated-user") {
		t.Fatal("isConfiguredServiceUser() = true for an unrelated account")
	}
}

func TestMaintenanceModeNamesCanonicalizeAliases(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{input: "prod", want: "production"},
		{input: "development", want: "development"},
		{input: "devel", want: "development"},
		{input: "debug", want: "debug"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := mode.ParseAppMode(tt.input)
			if err != nil {
				t.Fatalf("ParseAppMode(%q): %v", tt.input, err)
			}
			if got.String() != tt.want {
				t.Errorf("ParseAppMode(%q).String() = %q, want %q", tt.input, got.String(), tt.want)
			}
		})
	}
}

func TestRunMaintenanceModePersistsRequestedMode(t *testing.T) {
	if !service.IsElevated() {
		t.Skip("end-to-end mode command persistence test requires administrator authorization")
	}
	configDir := newMainTestDir(t)
	if err := runMaintenanceMode(configDir, "development"); err != nil {
		t.Fatalf("runMaintenanceMode(): %v", err)
	}
	configPath := filepath.Join(configDir, "server.yml")
	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read persisted config: %v", err)
	}
	var persisted struct {
		Server struct {
			Mode string `yaml:"mode"`
		} `yaml:"server"`
	}
	if err := yaml.Unmarshal(content, &persisted); err != nil {
		t.Fatalf("parse persisted config: %v", err)
	}
	if persisted.Server.Mode != "development" {
		t.Errorf("persisted server.mode = %q, want development", persisted.Server.Mode)
	}
}
