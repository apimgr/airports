package terminal

import "testing"

func TestCalculateModeBreakpoints(t *testing.T) {
	tests := []struct {
		cols, rows int
		want       SizeMode
	}{
		{39, 24, SizeModeMicro},
		{80, 9, SizeModeMicro},
		{40, 24, SizeModeMinimal},
		{80, 15, SizeModeMinimal},
		{60, 24, SizeModeCompact},
		{80, 23, SizeModeCompact},
		{80, 40, SizeModeStandard},
		{119, 50, SizeModeStandard},
		{120, 60, SizeModeWide},
		{199, 80, SizeModeWide},
		{200, 80, SizeModeUltrawide},
		{399, 90, SizeModeUltrawide},
		{400, 80, SizeModeMassive},
	}
	for _, test := range tests {
		if got := calculateMode(test.cols, test.rows); got != test.want {
			t.Errorf("calculateMode(%d, %d) = %v, want %v", test.cols, test.rows, got, test.want)
		}
	}
}

func TestSizeModeCapabilities(t *testing.T) {
	if !SizeModeMinimal.ShowIcons() || SizeModeMicro.ShowIcons() {
		t.Fatal("icon capability does not match the minimal breakpoint")
	}
	if !SizeModeCompact.ShowBorders() || SizeModeMinimal.ShowBorders() {
		t.Fatal("border capability does not match the compact breakpoint")
	}
	if !SizeModeStandard.ShowASCIIArt() || SizeModeCompact.ShowASCIIArt() {
		t.Fatal("ASCII art capability does not match the standard breakpoint")
	}
	if !SizeModeWide.ShowSidebar() || SizeModeStandard.ShowSidebar() {
		t.Fatal("sidebar capability does not match the wide breakpoint")
	}
}

func TestGetTerminalSizeReturnsUsableDimensions(t *testing.T) {
	size := GetTerminalSize()
	if size.Cols < 1 || size.Rows < 1 {
		t.Fatalf("terminal size = %+v", size)
	}
	if size.Mode != calculateMode(size.Cols, size.Rows) {
		t.Fatalf("terminal mode = %v for %dx%d", size.Mode, size.Cols, size.Rows)
	}
}
