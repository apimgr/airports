package mode

import (
	"errors"
	"os"
	"testing"
)

// resetState restores the package-level mode/debug singletons to their
// zero-value equivalents before and after each test so tests never leak
// state into one another (Get/Set/SetDebug are backed by shared globals).
func resetState(t *testing.T) {
	t.Helper()
	currentMode = Production
	debugEnabled = false
	t.Cleanup(func() {
		currentMode = Production
		debugEnabled = false
	})
}

func TestParseMode(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    Mode
		wantErr bool
	}{
		{"development", "development", Development, false},
		{"dev", "dev", Development, false},
		{"devel", "devel", Development, false},
		{"debug", "debug", Development, false},
		{"production", "production", Production, false},
		{"prod", "prod", Production, false},
		{"uppercase", "PRODUCTION", Production, false},
		{"mixed-case", "DeVeLoPmEnT", Development, false},
		{"whitespace", "  production  ", Production, false},
		{"empty", "", "", true},
		{"invalid", "bogus", "", true},
		{"invalid-with-spaces", "  bogus  ", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseMode(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ParseMode(%q): expected error, got nil", tt.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseMode(%q): unexpected error: %v", tt.input, err)
			}
			if got != tt.want {
				t.Errorf("ParseMode(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestSet(t *testing.T) {
	resetState(t)

	if err := SetAppMode("development"); err != nil {
		t.Fatalf("SetAppMode(development): unexpected error: %v", err)
	}
	if GetAppMode() != Development {
		t.Errorf("GetAppMode() = %q, want %q", GetAppMode(), Development)
	}

	if err := SetAppMode("production"); err != nil {
		t.Fatalf("SetAppMode(production): unexpected error: %v", err)
	}
	if GetAppMode() != Production {
		t.Errorf("GetAppMode() = %q, want %q", GetAppMode(), Production)
	}

	// Invalid input must not change the previously set mode.
	if err := SetAppMode("bogus"); err == nil {
		t.Fatal("SetAppMode(bogus): expected error, got nil")
	}
	if GetAppMode() != Production {
		t.Errorf("GetAppMode() after failed SetAppMode = %q, want unchanged %q", GetAppMode(), Production)
	}
}

func TestSetFromValue(t *testing.T) {
	t.Run("debug-alias-enables-debug", func(t *testing.T) {
		resetState(t)
		if err := SetFromValue("debug"); err != nil {
			t.Fatalf("SetFromValue(debug): unexpected error: %v", err)
		}
		if GetAppMode() != Development {
			t.Errorf("GetAppMode() = %q, want %q", GetAppMode(), Development)
		}
		if !IsDebug() {
			t.Error("IsDebug() = false, want true after SetFromValue(\"debug\")")
		}
	})

	t.Run("debug-alias-case-insensitive-and-trimmed", func(t *testing.T) {
		resetState(t)
		if err := SetFromValue("  DEBUG  "); err != nil {
			t.Fatalf("SetFromValue: unexpected error: %v", err)
		}
		if !IsDebug() {
			t.Error("IsDebug() = false, want true")
		}
	})

	t.Run("non-debug-value-does-not-touch-debug-flag", func(t *testing.T) {
		resetState(t)
		SetDebug(true)
		if err := SetFromValue("production"); err != nil {
			t.Fatalf("SetFromValue(production): unexpected error: %v", err)
		}
		if !IsDebug() {
			t.Error("IsDebug() = false, want true (SetFromValue must not clear debug for non-debug values)")
		}
	})

	t.Run("invalid-value-returns-error", func(t *testing.T) {
		resetState(t)
		if err := SetFromValue("bogus"); err == nil {
			t.Fatal("SetFromValue(bogus): expected error, got nil")
		}
	})
}

func TestSetDebugAndIsDebug(t *testing.T) {
	resetState(t)

	if IsDebug() {
		t.Fatal("IsDebug() = true before any SetDebug call, want false")
	}
	SetDebug(true)
	if !IsDebug() {
		t.Error("IsDebug() = false after SetDebug(true), want true")
	}
	SetDebug(false)
	if IsDebug() {
		t.Error("IsDebug() = true after SetDebug(false), want false")
	}
}

func TestIsDevelopmentIsProduction(t *testing.T) {
	resetState(t)

	if err := SetAppMode("production"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if !IsProduction() {
		t.Error("IsProduction() = false, want true")
	}
	if IsDevelopment() {
		t.Error("IsDevelopment() = true, want false")
	}

	if err := SetAppMode("development"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if !IsDevelopment() {
		t.Error("IsDevelopment() = false, want true")
	}
	if IsProduction() {
		t.Error("IsProduction() = true, want false")
	}
}

func TestInitialize(t *testing.T) {
	t.Run("cli-flag-wins", func(t *testing.T) {
		resetState(t)
		t.Setenv("MODE", "production")
		if err := Initialize("development"); err != nil {
			t.Fatalf("Initialize: unexpected error: %v", err)
		}
		if GetAppMode() != Development {
			t.Errorf("GetAppMode() = %q, want %q (CLI flag must win over env)", GetAppMode(), Development)
		}
	})

	t.Run("env-var-used-when-no-cli-flag", func(t *testing.T) {
		resetState(t)
		t.Setenv("MODE", "development")
		if err := Initialize(""); err != nil {
			t.Fatalf("Initialize: unexpected error: %v", err)
		}
		if GetAppMode() != Development {
			t.Errorf("GetAppMode() = %q, want %q", GetAppMode(), Development)
		}
	})

	t.Run("default-production-when-nothing-set", func(t *testing.T) {
		resetState(t)
		t.Setenv("MODE", "")
		if err := Initialize(""); err != nil {
			t.Fatalf("Initialize: unexpected error: %v", err)
		}
		if GetAppMode() != Production {
			t.Errorf("GetAppMode() = %q, want default %q", GetAppMode(), Production)
		}
	})

	t.Run("invalid-cli-flag-returns-error", func(t *testing.T) {
		resetState(t)
		if err := Initialize("bogus"); err == nil {
			t.Fatal("Initialize(bogus): expected error, got nil")
		}
	})

	t.Run("invalid-env-var-returns-error", func(t *testing.T) {
		resetState(t)
		t.Setenv("MODE", "bogus")
		if err := Initialize(""); err == nil {
			t.Fatal("Initialize with bad MODE env: expected error, got nil")
		}
	})

	t.Run("mode-debug-alias-via-env", func(t *testing.T) {
		resetState(t)
		t.Setenv("MODE", "debug")
		if err := Initialize(""); err != nil {
			t.Fatalf("Initialize: unexpected error: %v", err)
		}
		if GetAppMode() != Development {
			t.Errorf("GetAppMode() = %q, want %q", GetAppMode(), Development)
		}
		if !IsDebug() {
			t.Error("IsDebug() = false, want true (MODE=debug alias)")
		}
	})
}

func TestInitializeDebug(t *testing.T) {
	t.Run("cli-flag-highest-priority", func(t *testing.T) {
		resetState(t)
		t.Setenv("DEBUG", "false")
		InitializeDebug(true)
		if !IsDebug() {
			t.Error("IsDebug() = false, want true (CLI flag must win)")
		}
	})

	t.Run("env-var-truthy", func(t *testing.T) {
		resetState(t)
		t.Setenv("DEBUG", "true")
		InitializeDebug(false)
		if !IsDebug() {
			t.Error("IsDebug() = false, want true")
		}
	})

	t.Run("env-var-falsy-overrides-alias", func(t *testing.T) {
		resetState(t)
		SetDebug(true) // simulate MODE=debug alias already applied
		t.Setenv("DEBUG", "false")
		InitializeDebug(false)
		if IsDebug() {
			t.Error("IsDebug() = true, want false (DEBUG=false must win over alias)")
		}
	})

	t.Run("env-var-unset-leaves-alias-untouched", func(t *testing.T) {
		resetState(t)
		SetDebug(true) // simulate MODE=debug alias already applied
		unsetEnv(t, "DEBUG")
		InitializeDebug(false)
		if !IsDebug() {
			t.Error("IsDebug() = false, want true (unset DEBUG must leave alias in place)")
		}
	})

	t.Run("default-false-when-nothing-set", func(t *testing.T) {
		resetState(t)
		unsetEnv(t, "DEBUG")
		InitializeDebug(false)
		if IsDebug() {
			t.Error("IsDebug() = true, want false (default)")
		}
	})
}

// unsetEnv removes an environment variable for the duration of the test,
// restoring its previous value (or absence) afterward.
func unsetEnv(t *testing.T, key string) {
	t.Helper()
	prev, existed := os.LookupEnv(key)
	if err := os.Unsetenv(key); err != nil {
		t.Fatalf("unset %s: %v", key, err)
	}
	t.Cleanup(func() {
		if existed {
			_ = os.Setenv(key, prev)
		} else {
			_ = os.Unsetenv(key)
		}
	})
}

func TestGetErrorDetail(t *testing.T) {
	resetState(t)

	if got := GetErrorDetail(nil); got != "" {
		t.Errorf("GetErrorDetail(nil) = %q, want empty string", got)
	}

	testErr := errors.New("boom: connection refused")

	if err := SetAppMode("development"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if got := GetErrorDetail(testErr); got != testErr.Error() {
		t.Errorf("GetErrorDetail in development = %q, want %q", got, testErr.Error())
	}

	if err := SetAppMode("production"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	got := GetErrorDetail(testErr)
	if got == testErr.Error() {
		t.Error("GetErrorDetail in production leaked the raw error detail")
	}
	if got == "" {
		t.Error("GetErrorDetail in production returned empty string, want generic message")
	}
}

func TestShouldShowDebugEndpoints(t *testing.T) {
	resetState(t)

	if err := SetAppMode("production"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	SetDebug(false)
	if ShouldShowDebugEndpoints() {
		t.Error("ShouldShowDebugEndpoints() = true with debug off, want false")
	}

	if err := SetAppMode("development"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if ShouldShowDebugEndpoints() {
		t.Error("ShouldShowDebugEndpoints() = true in development with debug off, want false")
	}

	SetDebug(true)
	if !ShouldShowDebugEndpoints() {
		t.Error("ShouldShowDebugEndpoints() = false with debug on, want true")
	}

	if err := SetAppMode("production"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if !ShouldShowDebugEndpoints() {
		t.Error("ShouldShowDebugEndpoints() = false in production with debug on, want true (Production+Debug must be reachable)")
	}
	SetDebug(false)
}

func TestGetCacheHeaders(t *testing.T) {
	resetState(t)

	if err := SetAppMode("development"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	dev := GetCacheHeaders()
	if dev.CacheControl != "no-cache, no-store, must-revalidate" {
		t.Errorf("development CacheControl = %q", dev.CacheControl)
	}
	if dev.Pragma != "no-cache" {
		t.Errorf("development Pragma = %q, want no-cache", dev.Pragma)
	}
	if dev.Expires != "0" {
		t.Errorf("development Expires = %q, want 0", dev.Expires)
	}

	if err := SetAppMode("production"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	prod := GetCacheHeaders()
	if prod.CacheControl != "public, max-age=31536000, immutable" {
		t.Errorf("production CacheControl = %q", prod.CacheControl)
	}
	if prod.Pragma != "" {
		t.Errorf("production Pragma = %q, want empty", prod.Pragma)
	}
	if prod.Expires != "" {
		t.Errorf("production Expires = %q, want empty", prod.Expires)
	}
}

func TestGetLogLevel(t *testing.T) {
	resetState(t)

	if err := SetAppMode("development"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if got := getLogLevel(); got != "debug" {
		t.Errorf("getLogLevel() in development = %q, want debug", got)
	}

	if err := SetAppMode("production"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if got := getLogLevel(); got != "info" {
		t.Errorf("getLogLevel() in production = %q, want info", got)
	}
}

func TestShouldCacheTemplates(t *testing.T) {
	resetState(t)

	if err := SetAppMode("production"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if !ShouldCacheTemplates() {
		t.Error("ShouldCacheTemplates() = false in production, want true")
	}

	if err := SetAppMode("development"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if ShouldCacheTemplates() {
		t.Error("ShouldCacheTemplates() = true in development, want false")
	}
}

func TestShouldEnableAutoReload(t *testing.T) {
	resetState(t)

	if err := SetAppMode("development"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if !ShouldEnableAutoReload() {
		t.Error("ShouldEnableAutoReload() = false in development, want true")
	}

	if err := SetAppMode("production"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if ShouldEnableAutoReload() {
		t.Error("ShouldEnableAutoReload() = true in production, want false")
	}
}

func TestShouldEnableProfiling(t *testing.T) {
	resetState(t)

	if err := SetAppMode("development"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	SetDebug(false)
	if ShouldEnableProfiling() {
		t.Error("ShouldEnableProfiling() = true in development with debug off, want false")
	}

	SetDebug(true)
	if !ShouldEnableProfiling() {
		t.Error("ShouldEnableProfiling() = false with debug on, want true")
	}

	if err := SetAppMode("production"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if !ShouldEnableProfiling() {
		t.Error("ShouldEnableProfiling() = false in production with debug on, want true (Production+Debug must be reachable)")
	}
	SetDebug(false)
}

func TestSetDebugTogglesRuntimeProfiling(t *testing.T) {
	resetState(t)

	SetDebug(true)
	if !IsDebug() {
		t.Error("IsDebug() = false after SetDebug(true)")
	}

	SetDebug(false)
	if IsDebug() {
		t.Error("IsDebug() = true after SetDebug(false)")
	}
}

func TestGetPanicRecoveryMode(t *testing.T) {
	resetState(t)

	if err := SetAppMode("development"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if got := GetPanicRecoveryMode(); got != "verbose" {
		t.Errorf("GetPanicRecoveryMode() in development = %q, want verbose", got)
	}

	if err := SetAppMode("production"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if got := GetPanicRecoveryMode(); got != "graceful" {
		t.Errorf("GetPanicRecoveryMode() in production = %q, want graceful", got)
	}
}

func TestModeString(t *testing.T) {
	if Development.String() != "development" {
		t.Errorf("Development.String() = %q, want development", Development.String())
	}
	if Production.String() != "production" {
		t.Errorf("Production.String() = %q, want production", Production.String())
	}
}

func TestModeValidate(t *testing.T) {
	if err := Production.Validate(); err != nil {
		t.Errorf("Production.Validate() = %v, want nil", err)
	}
	if err := Development.Validate(); err != nil {
		t.Errorf("Development.Validate() = %v, want nil", err)
	}
	if err := Mode("bogus").Validate(); err == nil {
		t.Error("Mode(\"bogus\").Validate(): expected error, got nil")
	}
}
