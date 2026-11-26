package format

import (
	"testing"
	"time"
)

func TestDuration(t *testing.T) {
	cases := []struct {
		name  string
		lang  string
		value time.Duration
		want  string
	}{
		{name: "seconds singular", lang: "en", value: time.Second, want: "1 second"},
		{name: "seconds plural", lang: "en", value: 45 * time.Second, want: "45 seconds"},
		{name: "minutes and seconds", lang: "en", value: 125 * time.Second, want: "2 minutes 5 seconds"},
		{name: "hours and minutes", lang: "en", value: 90 * time.Minute, want: "1 hour 30 minutes"},
		{name: "days and hours", lang: "en", value: 76 * time.Hour, want: "3 days 4 hours"},
		{name: "localized unit", lang: "es", value: 2 * time.Second, want: "2 segundos"},
		{name: "negative clamps to zero", lang: "en", value: -time.Second, want: "0 seconds"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Duration(tc.lang, tc.value); got != tc.want {
				t.Errorf("Duration(%q, %s) = %q, want %q", tc.lang, tc.value, got, tc.want)
			}
		})
	}
}

func TestSize(t *testing.T) {
	cases := []struct {
		name  string
		lang  string
		value int64
		want  string
	}{
		{name: "one byte", lang: "en", value: 1, want: "1 byte"},
		{name: "bytes", lang: "en", value: 512, want: "512 bytes"},
		{name: "kilobyte", lang: "en", value: 1024, want: "1 kilobyte"},
		{name: "fractional megabytes", lang: "en", value: 2_621_440, want: "2.5 megabytes"},
		{name: "localized", lang: "fr", value: 1024, want: "1 kilooctet"},
		{name: "negative clamps to zero", lang: "en", value: -1, want: "0 bytes"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Size(tc.lang, tc.value); got != tc.want {
				t.Errorf("Size(%q, %d) = %q, want %q", tc.lang, tc.value, got, tc.want)
			}
		})
	}
}

func TestCount(t *testing.T) {
	cases := []struct {
		name  string
		lang  string
		value int64
		want  string
	}{
		{name: "english", lang: "en", value: 12847, want: "12,847"},
		{name: "german", lang: "de", value: 12847, want: "12.847"},
		{name: "french", lang: "fr", value: 12847, want: "12\u202f847"},
		{name: "arabic", lang: "ar", value: 12847, want: "١٢٬٨٤٧"},
		{name: "negative", lang: "en", value: -12847, want: "-12,847"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Count(tc.lang, tc.value); got != tc.want {
				t.Errorf("Count(%q, %d) = %q, want %q", tc.lang, tc.value, got, tc.want)
			}
		})
	}
}
