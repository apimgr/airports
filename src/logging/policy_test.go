package logging

import (
	"testing"
	"time"
)

func TestParseRotatePolicy(t *testing.T) {
	cases := []struct {
		in        string
		wantIv    Interval
		wantBytes int64
		wantErr   bool
	}{
		{"never", IntervalNever, 0, false},
		{"", IntervalNever, 0, false},
		{"daily", IntervalDaily, 0, false},
		{"weekly", IntervalWeekly, 0, false},
		{"monthly", IntervalMonthly, 0, false},
		{"yearly", IntervalYearly, 0, false},
		{"50MB", IntervalNever, 50 * 1024 * 1024, false},
		{"1GB", IntervalNever, 1024 * 1024 * 1024, false},
		{"512KB", IntervalNever, 512 * 1024, false},
		{"weekly,50MB", IntervalWeekly, 50 * 1024 * 1024, false},
		{"monthly, 1GB", IntervalMonthly, 1024 * 1024 * 1024, false},
		{"bogus", IntervalNever, 0, true},
		{"weekly,bogus", IntervalWeekly, 0, true},
	}

	for _, c := range cases {
		got, err := ParseRotatePolicy(c.in)
		if c.wantErr {
			if err == nil {
				t.Errorf("ParseRotatePolicy(%q): expected error, got nil", c.in)
			}
			continue
		}
		if err != nil {
			t.Fatalf("ParseRotatePolicy(%q): unexpected error: %v", c.in, err)
		}
		if got.Interval != c.wantIv {
			t.Errorf("ParseRotatePolicy(%q).Interval = %v, want %v", c.in, got.Interval, c.wantIv)
		}
		if got.SizeBytes != c.wantBytes {
			t.Errorf("ParseRotatePolicy(%q).SizeBytes = %d, want %d", c.in, got.SizeBytes, c.wantBytes)
		}
	}
}

func TestParseRetentionPolicy(t *testing.T) {
	cases := []struct {
		in      string
		wantM   RetentionMode
		wantN   int
		wantErr bool
	}{
		{"none", RetentionNone, 0, false},
		{"", RetentionNone, 0, false},
		{"forever", RetentionForever, 0, false},
		{"4", RetentionCount, 4, false},
		{"30d", RetentionDays, 30, false},
		{"12w", RetentionWeeks, 12, false},
		{"6m", RetentionMonths, 6, false},
		{"weekly:4", RetentionCount, 4, false},
		{"monthly:12", RetentionCount, 12, false},
		{"bogus", RetentionCount, 0, true},
		{"xd", RetentionDays, 0, true},
	}

	for _, c := range cases {
		got, err := ParseRetentionPolicy(c.in)
		if c.wantErr {
			if err == nil {
				t.Errorf("ParseRetentionPolicy(%q): expected error, got nil", c.in)
			}
			continue
		}
		if err != nil {
			t.Fatalf("ParseRetentionPolicy(%q): unexpected error: %v", c.in, err)
		}
		if got.Mode != c.wantM {
			t.Errorf("ParseRetentionPolicy(%q).Mode = %v, want %v", c.in, got.Mode, c.wantM)
		}
		if got.N != c.wantN {
			t.Errorf("ParseRetentionPolicy(%q).N = %d, want %d", c.in, got.N, c.wantN)
		}
	}
}

func TestIntervalDuration(t *testing.T) {
	cases := map[Interval]bool{
		IntervalNever:   false,
		IntervalDaily:   true,
		IntervalWeekly:  true,
		IntervalMonthly: true,
		IntervalYearly:  true,
	}
	for iv, wantPositive := range cases {
		d := iv.Duration()
		if wantPositive && d <= 0 {
			t.Errorf("Interval(%d).Duration() = %v, want positive", iv, d)
		}
		if !wantPositive && d != 0 {
			t.Errorf("IntervalNever.Duration() = %v, want 0", d)
		}
	}
}

func TestRetentionPolicyMaxAge(t *testing.T) {
	if got := (RetentionPolicy{Mode: RetentionDays, N: 2}).MaxAge(); got != 48*time.Hour {
		t.Errorf("MaxAge(2 days) = %v, want 48h", got)
	}
	if got := (RetentionPolicy{Mode: RetentionNone}).MaxAge(); got != 0 {
		t.Errorf("MaxAge(none) = %v, want 0", got)
	}
}
