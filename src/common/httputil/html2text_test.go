package httputil

import (
	"strings"
	"testing"
)

func TestHTML2TextConverter(t *testing.T) {
	got := HTML2TextConverter(`<main><h1>Airport</h1><p>JFK <strong>New York</strong></p><a href="/airports/KJFK">Details</a><form><button>Submit</button></form><script>alert(1)</script></main>`, 80)
	for _, want := range []string{"AIRPORT", "JFK *New York*", "Details [/airports/KJFK]"} {
		if !strings.Contains(got, want) {
			t.Errorf("converted text missing %q: %s", want, got)
		}
	}
	for _, forbidden := range []string{"Submit", "alert(1)"} {
		if strings.Contains(got, forbidden) {
			t.Errorf("converted text contains non-readable content %q", forbidden)
		}
	}
	if !strings.HasSuffix(got, "\n") || strings.HasSuffix(got, "\n\n") {
		t.Errorf("converted text must end with exactly one newline: %q", got)
	}
}

func TestRenderNoJSHTML(t *testing.T) {
	got, err := RenderNoJSHTML(`<html><body><p>Airport list</p><script>run()</script></body></html>`)
	if err != nil {
		t.Fatalf("RenderNoJSHTML: %v", err)
	}
	if !strings.Contains(got, "Airport list") || strings.Contains(got, "run()") {
		t.Fatalf("unexpected no-JS HTML: %s", got)
	}
}
