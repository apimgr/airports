package paths

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

func TestSafePath(t *testing.T) {
	tests := []struct {
		input string
		want  string
		err   error
	}{
		{input: "/my//admin/", want: "my/admin"},
		{input: "/../admin", err: ErrPathTraversal},
		{input: "Admin", err: ErrInvalidPath},
		{input: strings.Repeat("a", 65), err: ErrPathTooLong},
	}
	for _, tt := range tests {
		got, err := SafePath(tt.input)
		if !errors.Is(err, tt.err) {
			t.Errorf("SafePath(%q) error = %v, want %v", tt.input, err, tt.err)
			continue
		}
		if err == nil && got != tt.want {
			t.Errorf("SafePath(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestSafeFilePath(t *testing.T) {
	base := filepath.Join("/safe", "base")
	got, err := SafeFilePath(base, "config")
	if err != nil {
		t.Fatalf("SafeFilePath: %v", err)
	}
	want := filepath.Join(base, "config")
	if got != want {
		t.Fatalf("SafeFilePath = %q, want %q", got, want)
	}
	if _, err := SafeFilePath(base, "../outside"); !errors.Is(err, ErrPathTraversal) {
		t.Fatalf("SafeFilePath traversal error = %v, want ErrPathTraversal", err)
	}
}
