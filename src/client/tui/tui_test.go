package tui

import (
	"testing"

	"github.com/apimgr/airports/src/common/terminal"
)

func TestValidateServerURL(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		want    string
		wantErr bool
	}{
		{name: "host", value: "https://api.example.test", want: "https://api.example.test"},
		{name: "subpath", value: "https://api.example.test/airports/", want: "https://api.example.test/airports"},
		{name: "query rejected", value: "https://api.example.test?token=secret", wantErr: true},
		{name: "credentials rejected", value: "https://user" + ":pass@api.example.test", wantErr: true},
		{name: "scheme required", value: "api.example.test", wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := validateServerURL(test.value)
			if (err != nil) != test.wantErr {
				t.Fatalf("validateServerURL() error = %v, wantErr %v", err, test.wantErr)
			}
			if got != test.want {
				t.Errorf("validateServerURL() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestTUIInputValidation(t *testing.T) {
	if !validAirportCode("KJFK") || !validAirportCode("LHR") {
		t.Fatal("valid airport codes rejected")
	}
	if validAirportCode("JFK!") || validAirportCode("AB") || validAirportCode("ABCDE") {
		t.Fatal("invalid airport code accepted")
	}
	if !validCoordinate("40.64", -90, 90) || !validCoordinate("-73.78", -180, 180) {
		t.Fatal("valid coordinate rejected")
	}
	if validCoordinate("91", -90, 90) || validCoordinate("west", -180, 180) {
		t.Fatal("invalid coordinate accepted")
	}
}

func TestResponsiveLayoutAndTruncation(t *testing.T) {
	if got := terminalSizeMode(32, 8); got != terminal.SizeModeMicro {
		t.Errorf("micro size mode = %v", got)
	}
	if got := terminalSizeMode(100, 30); got != terminal.SizeModeStandard {
		t.Errorf("standard size mode = %v", got)
	}
	if got := GetLayoutConfig(terminal.SizeModeWide); !got.ShowSidebar || got.SidebarWidth == 0 {
		t.Errorf("wide layout = %#v, want sidebar", got)
	}
	if got := truncate("airport search result", 12); got != "airport s..." {
		t.Errorf("truncate() = %q, want airport s...", got)
	}
	if got := truncateMiddle("https://example.com/a/path", 11); got != "http...path" {
		t.Errorf("truncateMiddle() = %q, want http...path", got)
	}
}

func TestNewModelStartsSetupWhenServerMissing(t *testing.T) {
	m := newModel(Config{}, Actions{})
	if m.page != pageSetup || len(m.inputs) != 2 || !m.inputs[0].Focused() {
		t.Fatalf("initial setup state = page %v, inputs %d, focused %v", m.page, len(m.inputs), len(m.inputs) > 0 && m.inputs[0].Focused())
	}
}
