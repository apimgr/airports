package config

import (
	"fmt"
	"strings"
)

// truthyValues is the canonical set of strings treated as boolean true.
// Keys are lowercase; callers must lowercase input before lookup.
var truthyValues = map[string]bool{
	"1": true, "y": true, "t": true,
	"yes": true, "true": true, "on": true, "ok": true,
	"enable": true, "enabled": true,
	"yep": true, "yup": true, "yeah": true,
	"aye": true, "si": true, "oui": true, "da": true, "hai": true,
	"affirmative": true, "accept": true, "allow": true, "grant": true,
	"sure": true, "totally": true,
}

// falsyValues is the canonical set of strings treated as boolean false.
// Keys are lowercase; callers must lowercase input before lookup.
var falsyValues = map[string]bool{
	"0": true, "n": true, "f": true,
	"no": true, "false": true, "off": true,
	"disable": true, "disabled": true,
	"nope": true, "nah": true, "nay": true,
	"nein": true, "non": true, "niet": true, "iie": true, "lie": true,
	"negative": true, "reject": true, "block": true, "revoke": true,
	"deny": true, "never": true, "noway": true,
}

// ParseBool parses a string into a boolean using truthy/falsy values.
// Returns the parsed value and nil on success.
// Returns false and an error for invalid values.
// Empty string returns the provided default value.
//
// Use this instead of strconv.ParseBool to handle the full range of
// user-supplied values consistently across the application.
func ParseBool(s string, defaultVal bool) (bool, error) {
	s = strings.TrimSpace(strings.ToLower(s))

	if s == "" {
		return defaultVal, nil
	}

	if truthyValues[s] {
		return true, nil
	}

	if falsyValues[s] {
		return false, nil
	}

	return false, fmt.Errorf("invalid boolean value: %q", s)
}

// IsTruthy returns true if the string is a truthy value.
// Returns false for empty, invalid, or falsy values (no error).
func IsTruthy(s string) bool {
	return truthyValues[strings.TrimSpace(strings.ToLower(s))]
}

// IsFalsy returns true if the string is a falsy value.
// Returns false for empty, invalid, or truthy values (no error).
func IsFalsy(s string) bool {
	return falsyValues[strings.TrimSpace(strings.ToLower(s))]
}
