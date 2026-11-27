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
	}{{"full", 120, []string{"Airports", "1.2.3", "development", "abc123", "8443"}}, {"compact", 70, []string{"Airports", "v1.2.3", "development"}}, {"minimal", 50, []string{"Airports 1.2.3", "example.test:8443"}}, {"micro", 30, []string{"Airports :8443"}}} {
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

type countedWriter struct {
	bytes.Buffer
	writes int
}

func (writer *countedWriter) Write(data []byte) (int, error) {
	writer.writes++
	return writer.Buffer.Write(data)
}

func TestCompactBannerUsesStartedLabel(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	t.Setenv("TERM", "xterm-256color")
	var output bytes.Buffer
	printServerStartupBannerTo(&output, bannerConfig{AppName: "Airports", Version: "1.0", AppMode: "production", StartupDatetime: "Now"}, "en", terminal.TerminalSize{Cols: 70}, display.DisplayEnv{IsTerminal: true, TerminalType: "xterm-256color"})
	if !strings.Contains(output.String(), "Started: Now") || strings.Contains(output.String(), "Server started on") {
		t.Fatalf("compact banner used the wrong startup label: %q", output.String())
	}
}

func TestMicroBannerIsOneLineAndDisclosesSetupToken(t *testing.T) {
	var output countedWriter
	printServerStartupBannerTo(&output, bannerConfig{
		AppName: "airports", Version: "1.0", URLs: []string{"https://example.test:8080"},
		StartupDatetime: "Nov 26, 2026", OnionAddress: "hidden.onion", SetupToken: "adm_private-token-value",
	}, "en", terminal.TerminalSize{Cols: 30}, display.DisplayEnv{IsTerminal: true, TerminalType: "xterm-256color"})
	if output.String() != "airports :8080 [SETUP]\nAdmin token (shown once): adm_private-token-value\n" || output.writes != 1 {
		t.Fatalf("micro banner = %q with %d writes", output.String(), output.writes)
	}
}

func TestMicroSetupMarkerDoesNotExceedTerminalWidth(t *testing.T) {
	var output bytes.Buffer
	printServerStartupBannerTo(&output, bannerConfig{AppName: "airports", URLs: []string{"https://example.test:8080"}, SetupToken: "adm_firststarttoken"}, "en", terminal.TerminalSize{Cols: 20}, display.DisplayEnv{IsTerminal: true, TerminalType: "xterm-256color"})
	firstLine := strings.SplitN(output.String(), "\n", 2)[0]
	if len(firstLine) > 20 || strings.Contains(firstLine, "[SETUP]") {
		t.Fatalf("micro banner overflowed or retained an oversized setup marker: %q", firstLine)
	}
	if !strings.Contains(output.String(), "Admin token (shown once): adm_firststarttoken") {
		t.Fatalf("separate setup token disclosure is missing: %q", output.String())
	}
}

func TestPlainBannerDisclosesFirstStartSetupToken(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	t.Setenv("TERM", "dumb")
	var output bytes.Buffer
	printServerStartupBannerTo(&output, bannerConfig{AppName: "airports", Version: "1.0", AppMode: "production", SetupToken: "adm_private-token-value"}, "en", terminal.TerminalSize{Cols: 120}, display.DisplayEnv{TerminalType: "dumb"})
	if !strings.Contains(output.String(), "Admin token (shown once): adm_private-token-value") {
		t.Fatalf("plain banner omitted first-start setup token: %q", output.String())
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
	if !strings.Contains(output.String(), "Mode: production") {
		t.Fatalf("TERM=dumb banner omitted fallback details: %q", output.String())
	}
}

func TestBannerFallbackDefaultsAndURLParsing(t *testing.T) {
	var output bytes.Buffer
	cfg := bannerConfig{URLs: []string{"https://example.test:443", "http://example.test:8080", "http://[invalid"}}
	printServerStartupBannerTo(&output, cfg, "en", terminal.TerminalSize{Cols: 50}, display.DisplayEnv{TerminalType: "dumb"})
	for _, want := range []string{"Airports API Server vunknown", "example.test", "example.test:8080", "http://[invalid"} {
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
	PrintServerStartupBanner(ServerBannerInfo{AppName: "Airports", Version: "1.0", AppMode: "production", Lang: "en"})
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

func TestBannerPrintsResolvedStartupAndFirstStartDetails(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	t.Setenv("TERM", "xterm-256color")
	var output bytes.Buffer
	printServerStartupBannerTo(&output, bannerConfig{
		AppName: "Airports", Version: "1.0", AppMode: "production",
		StartupDatetime: "November 26, 2026 at 10:30:00 EST",
		URLs:            []string{"https://airports.example.test"},
		OnionAddress:    "http://airports.onion", I2PAddress: "http://airports.b32.i2p",
		SMTPAddress: "mail.example.test:25", SetupToken: "adm_firststarttoken",
	}, "en", terminal.TerminalSize{Cols: 120}, display.DisplayEnv{IsTerminal: true, TerminalType: "xterm-256color"})
	for _, value := range []string{"November 26, 2026 at 10:30:00 EST", "http://airports.onion", "http://airports.b32.i2p", "mail.example.test:25", "Admin token (shown once): adm_firststarttoken"} {
		if !strings.Contains(output.String(), value) {
			t.Errorf("startup banner omitted %q: %s", value, output.String())
		}
	}
	var laterStart bytes.Buffer
	printServerStartupBannerTo(&laterStart, bannerConfig{AppName: "Airports", Version: "1.0", AppMode: "production"}, "en", terminal.TerminalSize{Cols: 120}, display.DisplayEnv{IsTerminal: true, TerminalType: "xterm-256color"})
	if strings.Contains(laterStart.String(), "adm_firststarttoken") {
		t.Fatal("startup banner disclosed a setup token after first start")
	}
}
