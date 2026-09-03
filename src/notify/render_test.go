package notify

import "testing"

func TestRenderSubstitutesKnownTokens(t *testing.T) {
	subject, body := Render("Hi {name}", "Body {name} at {app_name}.", map[string]string{
		"name":     "World",
		"app_name": "airports",
	})
	if subject != "Hi World" {
		t.Errorf("subject = %q", subject)
	}
	if body != "Body World at airports." {
		t.Errorf("body = %q", body)
	}
}

func TestRenderLeavesUnmatchedTokensLiteral(t *testing.T) {
	subject, body := Render("Hi {name}", "Body {unknown_token}.", map[string]string{
		"name": "World",
	})
	if subject != "Hi World" {
		t.Errorf("subject = %q", subject)
	}
	if body != "Body {unknown_token}." {
		t.Errorf("body = %q, want unmatched token left literal", body)
	}
}

func TestRenderEmptyVars(t *testing.T) {
	subject, body := Render("Plain subject", "Plain body", map[string]string{})
	if subject != "Plain subject" || body != "Plain body" {
		t.Errorf("expected no substitution, got subject=%q body=%q", subject, body)
	}
}
