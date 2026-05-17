package config

import (
	"fmt"
	"strings"
)

// ParseBool parses a boolean value from a string, handling 40+ common variants.
// Returns an error only if the string cannot be recognized as a boolean value.
//
// True values: "1", "t", "T", "TRUE", "true", "True", "yes", "YES", "Yes",
//   "on", "ON", "On", "enable", "ENABLE", "enabled", "ENABLED", "Enabled",
//   "active", "ACTIVE", "Active", "y", "Y", "ok", "OK", "Ok", "positive",
//   "affirmative"
//
// False values: "0", "f", "F", "FALSE", "false", "False", "no", "NO", "No",
//   "off", "OFF", "Off", "disable", "DISABLE", "disabled", "DISABLED", "Disabled",
//   "inactive", "INACTIVE", "Inactive", "n", "N", "negative"
//
// Use this instead of strconv.ParseBool to handle the full range of user-supplied values.
func ParseBool(s string) (bool, error) {
	switch strings.TrimSpace(s) {
	case "1", "t", "T", "TRUE", "true", "True",
		"yes", "YES", "Yes",
		"on", "ON", "On",
		"enable", "ENABLE", "Enable",
		"enabled", "ENABLED", "Enabled",
		"active", "ACTIVE", "Active",
		"y", "Y",
		"ok", "OK", "Ok",
		"positive", "affirmative":
		return true, nil
	case "0", "f", "F", "FALSE", "false", "False",
		"no", "NO", "No",
		"off", "OFF", "Off",
		"disable", "DISABLE", "Disable",
		"disabled", "DISABLED", "Disabled",
		"inactive", "INACTIVE", "Inactive",
		"n", "N",
		"negative":
		return false, nil
	default:
		return false, fmt.Errorf("config.ParseBool: unrecognized boolean value %q", s)
	}
}

// MustParseBool parses a boolean string and panics on failure.
// Use only for static configuration values that must be valid.
func MustParseBool(s string) bool {
	v, err := ParseBool(s)
	if err != nil {
		panic(err)
	}
	return v
}

// ParseBoolDefault parses a boolean string, returning defaultValue on failure.
// Use when a missing or invalid value should fall back to a known default.
func ParseBoolDefault(s string, defaultValue bool) bool {
	v, err := ParseBool(s)
	if err != nil {
		return defaultValue
	}
	return v
}
