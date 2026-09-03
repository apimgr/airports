package config

import (
	"testing"
)

func TestParseBool(t *testing.T) {
	trueValues := []string{
		"1", "t", "T", "true", "True", "TRUE",
		"yes", "Yes", "YES", "on", "On", "ON",
		"enable", "Enable", "ENABLE",
		"enabled", "Enabled", "ENABLED",
		"y", "Y",
	}
	for _, v := range trueValues {
		t.Run("true:"+v, func(t *testing.T) {
			got, err := ParseBool(v, false)
			if err != nil {
				t.Fatalf("ParseBool(%q): unexpected error: %v", v, err)
			}
			if !got {
				t.Errorf("ParseBool(%q) = false, want true", v)
			}
		})
	}

	falseValues := []string{
		"0", "f", "F", "false", "False", "FALSE",
		"no", "No", "NO", "off", "Off", "OFF",
		"disable", "Disable", "DISABLE",
		"disabled", "Disabled", "DISABLED",
		"n", "N",
	}
	for _, v := range falseValues {
		t.Run("false:"+v, func(t *testing.T) {
			got, err := ParseBool(v, true)
			if err != nil {
				t.Fatalf("ParseBool(%q): unexpected error: %v", v, err)
			}
			if got {
				t.Errorf("ParseBool(%q) = true, want false", v)
			}
		})
	}

	// Whitespace trimming
	t.Run("whitespace", func(t *testing.T) {
		got, err := ParseBool("  true  ", false)
		if err != nil {
			t.Fatalf("ParseBool with whitespace: unexpected error: %v", err)
		}
		if !got {
			t.Error("ParseBool(\"  true  \") = false, want true")
		}
	})

	// Unknown value
	t.Run("unknown", func(t *testing.T) {
		_, err := ParseBool("maybe", false)
		if err == nil {
			t.Error("ParseBool(\"maybe\"): expected error, got nil")
		}
	})

	// Empty string returns the provided default
	t.Run("empty-default-true", func(t *testing.T) {
		got, err := ParseBool("", true)
		if err != nil {
			t.Fatalf("ParseBool(\"\", true): unexpected error: %v", err)
		}
		if !got {
			t.Error("ParseBool(\"\", true) = false, want true (default)")
		}
	})
	t.Run("empty-default-false", func(t *testing.T) {
		got, err := ParseBool("", false)
		if err != nil {
			t.Fatalf("ParseBool(\"\", false): unexpected error: %v", err)
		}
		if got {
			t.Error("ParseBool(\"\", false) = true, want false (default)")
		}
	})
}

func TestIsTruthy(t *testing.T) {
	if !IsTruthy("yes") {
		t.Error("IsTruthy(\"yes\") = false, want true")
	}
	if IsTruthy("maybe") {
		t.Error("IsTruthy(\"maybe\") = true, want false")
	}
	if IsTruthy("") {
		t.Error("IsTruthy(\"\") = true, want false")
	}
	if IsTruthy("no") {
		t.Error("IsTruthy(\"no\") = true, want false")
	}
}

func TestIsFalsy(t *testing.T) {
	if !IsFalsy("no") {
		t.Error("IsFalsy(\"no\") = false, want true")
	}
	if IsFalsy("maybe") {
		t.Error("IsFalsy(\"maybe\") = true, want false")
	}
	if IsFalsy("") {
		t.Error("IsFalsy(\"\") = true, want false")
	}
	if IsFalsy("yes") {
		t.Error("IsFalsy(\"yes\") = true, want false")
	}
}
