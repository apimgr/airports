package version

import "testing"

func TestParseBuildEpoch(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  int64
	}{
		{name: "valid", input: "1765112713", want: 1765112713},
		{name: "empty", input: "", want: 0},
		{name: "invalid", input: "not-an-epoch", want: 0},
		{name: "non-positive", input: "0", want: 0},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := ParseBuildEpoch(test.input); got != test.want {
				t.Errorf("ParseBuildEpoch(%q) = %d, want %d", test.input, got, test.want)
			}
		})
	}
}

func TestBuildDateFromEpoch(t *testing.T) {
	if got := BuildDateFromEpoch("1765112713", "unknown"); got != "2025-12-07T13:05:13Z" {
		t.Errorf("BuildDateFromEpoch(valid) = %q, want RFC 3339 UTC value", got)
	}
	if got := BuildDateFromEpoch("invalid", "unknown"); got != "unknown" {
		t.Errorf("BuildDateFromEpoch(invalid) = %q, want fallback", got)
	}
}

func TestBuildInfoFormat(t *testing.T) {
	info := BuildInfo{Version: "1.2.3", CommitID: "abc1234", BuildDate: "2025-12-07T04:25:13Z"}
	if got, want := info.Format("airports"), "airports 1.2.3 (commit abc1234, built 2025-12-07T04:25:13Z)"; got != want {
		t.Errorf("BuildInfo.Format() = %q, want %q", got, want)
	}
}
