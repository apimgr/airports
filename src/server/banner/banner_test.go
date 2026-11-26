package banner

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/apimgr/airports/src/common/display"
	"github.com/apimgr/airports/src/common/terminal"
)

func TestBannerWidthsAndBuildDetails(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	t.Setenv("TERM", "xterm-256color")
	forceColor := true
	emoji := true
	cfg := bannerConfig{
		AppName: "Airports", Version: "1.2.3", CommitID: "abc123", BuildDate: "today",
		AppMode: "development", Debug: true, URLs: []string{"https://example.test:8443", "https://example.test"},
		ForceColor: &forceColor, ConfigEmoji: &emoji,
	}
	env := display.DisplayEnv{IsTerminal: true, TerminalType: "xterm-256color"}

	for _, test := range []struct {
		name string
		cols int
		want []string
	}{{"full", 120, []string{"Airports", "v1.2.3", "development", "abc123", "8443"}}, {"compact", 70, []string{"Airports", "v1.2.3", "development"}}, {"minimal", 50, []string{"Airports 1.2.3", "example.test:8443"}}, {"micro", 30, []string{"Airports example.test:8443"}}} {
		t.Run(test.name, func(t *testing.T) {
			var output bytes.Buffer
			printServerStartupBannerTo(&output, cfg, "en", terminal.TerminalSize{Cols: test.cols}, env)
			for _, want := range test.want {
				if !strings.Contains(output.String(), want) {
					t.Errorf("banner at %d columns missing %q: %s", test.cols, want, output.String())
				}
			}
			if test.name == "full" && (!strings.Contains(output.String(), "\x1b[1m") || !strings.Contains(output.String(), "✈")) {
				t.Errorf("full banner did not honor color and emoji configuration: %s", output.String())
			}
		})
	}
}

func TestBannerDumbTerminalDisablesDecorations(t *testing.T) {
	forceColor := true
	emoji := true
	cfg := bannerConfig{AppName: "Airports", Version: "1.0", AppMode: "production", URLs: []string{"https://example.test"}, ForceColor: &forceColor, ConfigColor: &forceColor, ConfigEmoji: &emoji}
	var output bytes.Buffer
	printServerStartupBannerTo(&output, cfg, "en", terminal.TerminalSize{Cols: 120}, display.DisplayEnv{IsTerminal: true, TerminalType: "dumb"})
	if strings.Contains(output.String(), "\x1b[") || strings.Contains(output.String(), "✈") || strings.Contains(output.String(), "🔒") {
		t.Fatalf("TERM=dumb banner contains terminal decoration: %q", output.String())
	}
	if !strings.Contains(output.String(), "Running in mode: production") {
		t.Fatalf("TERM=dumb banner omitted fallback details: %q", output.String())
	}
}

func TestBannerFallbackDefaultsAndURLParsing(t *testing.T) {
	var output bytes.Buffer
	cfg := bannerConfig{URLs: []string{"https://example.test:443", "http://example.test:8080", "http://[invalid"}}
	printServerStartupBannerTo(&output, cfg, "en", terminal.TerminalSize{Cols: 50}, display.DisplayEnv{TerminalType: "dumb"})
	for _, want := range []string{"Airports API Server unknown", "example.test", "example.test:8080", "http://[invalid"} {
		if !strings.Contains(output.String(), want) {
			t.Errorf("banner missing %q: %q", want, output.String())
		}
	}
}

func TestBannerHelpersAndStartupWrapper(t *testing.T) {
	if got := hostPort("https://example.test:443/path"); got != "example.test" {
		t.Errorf("hostPort(https default) = %q", got)
	}
	if got := hostPort("http://example.test:8080/path"); got != "example.test:8080" {
		t.Errorf("hostPort(custom port) = %q", got)
	}
	if got := hostPort("http://[invalid"); got != "http://[invalid" {
		t.Errorf("hostPort(invalid URL) = %q", got)
	}
	if fallback(" \t") != "unknown" || fallback("ok") != "ok" {
		t.Fatal("fallback() did not preserve the empty/non-empty behavior")
	}
	t.Setenv("TERM", "dumb")
	t.Setenv("NO_COLOR", "")
	oldStdout := os.Stdout
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = writer
	PrintServerStartupBanner("Airports", "1.0", "production", nil, nil, "en")
	_ = writer.Close()
	os.Stdout = oldStdout
	var output bytes.Buffer
	if _, err := output.ReadFrom(reader); err != nil {
		t.Fatal(err)
	}
	_ = reader.Close()
	if !strings.Contains(output.String(), "Airports v1.0") {
		t.Fatalf("startup wrapper output = %q", output.String())
	}
}
