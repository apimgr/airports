// Package logging implements the per-log-type file logging and rotation
// subsystem described in AI.md PART 11 "Logging". It never crashes on file
// errors — every failure for one log file is logged as a warning and the
// remaining log files continue to be processed.
package logging

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Interval is a calendar-based rotation cadence.
type Interval int

// Supported rotation cadences, per AI.md "Rotation Options".
const (
	IntervalNever Interval = iota
	IntervalDaily
	IntervalWeekly
	IntervalMonthly
	IntervalYearly
)

// Duration returns the fixed-length approximation used to decide whether an
// interval-based rotation is due. Calendar months/years are approximated as
// 30/365 days respectively, which is an intentional simplification — AI.md
// only requires "rotate roughly on this cadence", not exact calendar-month
// boundaries.
func (iv Interval) Duration() time.Duration {
	switch iv {
	case IntervalDaily:
		return 24 * time.Hour
	case IntervalWeekly:
		return 7 * 24 * time.Hour
	case IntervalMonthly:
		return 30 * 24 * time.Hour
	case IntervalYearly:
		return 365 * 24 * time.Hour
	default:
		return 0
	}
}

// RotatePolicy is a parsed "rotate:" value, e.g. "weekly,50MB" meaning
// rotate on whichever of the two thresholds is reached first.
type RotatePolicy struct {
	Interval  Interval
	SizeBytes int64
}

// ParseRotatePolicy parses AI.md's rotation option grammar: "never", one of
// the calendar interval keywords, a bare size ("50MB", "1GB"), or a
// comma-separated combination of an interval and a size ("weekly,50MB").
func ParseRotatePolicy(s string) (RotatePolicy, error) {
	s = strings.TrimSpace(s)
	if s == "" || strings.EqualFold(s, "never") {
		return RotatePolicy{Interval: IntervalNever}, nil
	}

	var policy RotatePolicy
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		switch strings.ToLower(part) {
		case "never":
			policy.Interval = IntervalNever
		case "daily":
			policy.Interval = IntervalDaily
		case "weekly":
			policy.Interval = IntervalWeekly
		case "monthly":
			policy.Interval = IntervalMonthly
		case "yearly":
			policy.Interval = IntervalYearly
		default:
			size, err := parseSizeSpec(part)
			if err != nil {
				return RotatePolicy{}, fmt.Errorf("logging: invalid rotate spec %q: %w", s, err)
			}
			policy.SizeBytes = size
		}
	}
	return policy, nil
}

// parseSizeSpec parses a size threshold such as "50MB" or "1.5GB" into bytes.
func parseSizeSpec(s string) (int64, error) {
	upper := strings.ToUpper(strings.TrimSpace(s))
	var unitBytes float64
	var numeric string
	switch {
	case strings.HasSuffix(upper, "GB"):
		unitBytes = 1024 * 1024 * 1024
		numeric = strings.TrimSuffix(upper, "GB")
	case strings.HasSuffix(upper, "MB"):
		unitBytes = 1024 * 1024
		numeric = strings.TrimSuffix(upper, "MB")
	case strings.HasSuffix(upper, "KB"):
		unitBytes = 1024
		numeric = strings.TrimSuffix(upper, "KB")
	case strings.HasSuffix(upper, "B"):
		unitBytes = 1
		numeric = strings.TrimSuffix(upper, "B")
	default:
		return 0, fmt.Errorf("unrecognized size unit in %q", s)
	}
	n, err := strconv.ParseFloat(strings.TrimSpace(numeric), 64)
	if err != nil {
		return 0, fmt.Errorf("invalid numeric size in %q: %w", s, err)
	}
	return int64(n * unitBytes), nil
}

// RetentionMode is the kind of retention policy applied to rotated backups.
type RetentionMode int

// Supported retention modes, per AI.md "Retention Options".
const (
	// RetentionNone deletes the rotated backup immediately (default).
	RetentionNone RetentionMode = iota
	// RetentionCount keeps the N most recent rotated backups.
	RetentionCount
	// RetentionDays keeps rotated backups newer than N days.
	RetentionDays
	// RetentionWeeks keeps rotated backups newer than N weeks.
	RetentionWeeks
	// RetentionMonths keeps rotated backups newer than N months.
	RetentionMonths
	// RetentionForever never deletes rotated backups automatically.
	RetentionForever
)

// RetentionPolicy is a parsed "keep:" value.
type RetentionPolicy struct {
	Mode RetentionMode
	N    int
}

// MaxAge returns the maximum age a rotated backup may have before it is
// pruned under RetentionDays/RetentionWeeks/RetentionMonths. It is only
// meaningful for those three modes.
func (p RetentionPolicy) MaxAge() time.Duration {
	switch p.Mode {
	case RetentionDays:
		return time.Duration(p.N) * 24 * time.Hour
	case RetentionWeeks:
		return time.Duration(p.N) * 7 * 24 * time.Hour
	case RetentionMonths:
		return time.Duration(p.N) * 30 * 24 * time.Hour
	default:
		return 0
	}
}

// ParseRetentionPolicy parses AI.md's retention option grammar: "none",
// "forever", a bare count ("4"), a suffixed duration ("30d", "12w", "6m"),
// or the "label:N" shorthand ("weekly:4", "monthly:4") which is treated as
// an N-count retention (the label only documents intent, since rotation
// cadence already comes from the paired "rotate:" policy).
func ParseRetentionPolicy(s string) (RetentionPolicy, error) {
	trimmed := strings.ToLower(strings.TrimSpace(s))
	switch trimmed {
	case "", "none":
		return RetentionPolicy{Mode: RetentionNone}, nil
	case "forever":
		return RetentionPolicy{Mode: RetentionForever}, nil
	}

	if idx := strings.Index(trimmed, ":"); idx >= 0 {
		n, err := strconv.Atoi(strings.TrimSpace(trimmed[idx+1:]))
		if err != nil {
			return RetentionPolicy{}, fmt.Errorf("logging: invalid keep spec %q: %w", s, err)
		}
		return RetentionPolicy{Mode: RetentionCount, N: n}, nil
	}

	switch {
	case strings.HasSuffix(trimmed, "d"):
		n, err := strconv.Atoi(strings.TrimSuffix(trimmed, "d"))
		if err != nil {
			return RetentionPolicy{}, fmt.Errorf("logging: invalid keep spec %q: %w", s, err)
		}
		return RetentionPolicy{Mode: RetentionDays, N: n}, nil
	case strings.HasSuffix(trimmed, "w"):
		n, err := strconv.Atoi(strings.TrimSuffix(trimmed, "w"))
		if err != nil {
			return RetentionPolicy{}, fmt.Errorf("logging: invalid keep spec %q: %w", s, err)
		}
		return RetentionPolicy{Mode: RetentionWeeks, N: n}, nil
	case strings.HasSuffix(trimmed, "m"):
		n, err := strconv.Atoi(strings.TrimSuffix(trimmed, "m"))
		if err != nil {
			return RetentionPolicy{}, fmt.Errorf("logging: invalid keep spec %q: %w", s, err)
		}
		return RetentionPolicy{Mode: RetentionMonths, N: n}, nil
	}

	n, err := strconv.Atoi(trimmed)
	if err != nil {
		return RetentionPolicy{}, fmt.Errorf("logging: invalid keep spec %q: %w", s, err)
	}
	return RetentionPolicy{Mode: RetentionCount, N: n}, nil
}
