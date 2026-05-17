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
			got, err := ParseBool(v)
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
			got, err := ParseBool(v)
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
		got, err := ParseBool("  true  ")
		if err != nil {
			t.Fatalf("ParseBool with whitespace: unexpected error: %v", err)
		}
		if !got {
			t.Error("ParseBool(\"  true  \") = false, want true")
		}
	})

	// Unknown value
	t.Run("unknown", func(t *testing.T) {
		_, err := ParseBool("maybe")
		if err == nil {
			t.Error("ParseBool(\"maybe\"): expected error, got nil")
		}
	})

	// Empty string
	t.Run("empty", func(t *testing.T) {
		_, err := ParseBool("")
		if err == nil {
			t.Error("ParseBool(\"\"): expected error, got nil")
		}
	})
}

func TestMustParseBool(t *testing.T) {
	if !MustParseBool("true") {
		t.Error("MustParseBool(\"true\") = false")
	}
	if MustParseBool("false") {
		t.Error("MustParseBool(\"false\") = true")
	}
}

func TestParseBoolDefault(t *testing.T) {
	got := ParseBoolDefault("yes", false)
	if !got {
		t.Error("ParseBoolDefault(\"yes\", false) = false, want true")
	}
	got = ParseBoolDefault("maybe", true)
	if !got {
		t.Error("ParseBoolDefault(\"maybe\", true) = false, want true (default)")
	}
	got = ParseBoolDefault("maybe", false)
	if got {
		t.Error("ParseBoolDefault(\"maybe\", false) = true, want false (default)")
	}
}
