package gui

import (
	"context"
	"net/url"
	"testing"
)

func TestStubReportsGUIUnavailable(t *testing.T) {
	if IsGUIAvailable() {
		t.Fatal("stub build reported GUI support")
	}
	if err := LaunchGUI(NativeGUIConfig{
		OnConnect: func(context.Context, string, string) (string, error) { return "", nil },
		OnRequest: func(context.Context, string, string, string, string, url.Values, string) (string, error) {
			return "", nil
		},
		Save: func(string, string) error { return nil },
	}); err == nil {
		t.Fatal("stub GUI launch unexpectedly succeeded")
	}
}
