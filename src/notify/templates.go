// Package notify implements operator email notifications per AI.md PART 17
// "EMAIL & NOTIFICATIONS": SMTP autodetection, connection testing, template
// loading/rendering, and event-gated sending. airports has no user accounts,
// so every email here is an operator notification (never an account email).
package notify

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

//go:embed templates/*.txt
var embeddedTemplates embed.FS

// LoadTemplate resolves the subject/body pair for name per AI.md PART 17
// "Template Storage": a custom override at {configDir}/template/email/{name}.txt
// wins if present, otherwise the embedded default is used. configDir may be
// empty, in which case only the embedded default is consulted.
func LoadTemplate(configDir, name string) (subject, body string, err error) {
	if configDir != "" {
		customPath := filepath.Join(configDir, "template", "email", name+".txt")
		if data, readErr := os.ReadFile(customPath); readErr == nil {
			return parseTemplate(string(data))
		}
	}

	data, err := embeddedTemplates.ReadFile("templates/" + name + ".txt")
	if err != nil {
		return "", "", fmt.Errorf("no default template embedded for %q: %w", name, err)
	}
	return parseTemplate(string(data))
}

// parseTemplate splits raw template content into subject/body per AI.md
// PART 17 "Template Format": first line "Subject: ...", a line containing
// only "---", then the plain-text body.
func parseTemplate(raw string) (subject, body string, err error) {
	lines := strings.Split(raw, "\n")
	if len(lines) < 2 {
		return "", "", fmt.Errorf("template too short: expected Subject line, separator, and body")
	}

	subjectLine := strings.TrimRight(lines[0], "\r")
	const subjectPrefix = "Subject:"
	if !strings.HasPrefix(subjectLine, subjectPrefix) {
		return "", "", fmt.Errorf("template first line must start with %q", subjectPrefix)
	}
	subject = strings.TrimSpace(strings.TrimPrefix(subjectLine, subjectPrefix))

	separatorIdx := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimRight(lines[i], "\r") == "---" {
			separatorIdx = i
			break
		}
	}
	if separatorIdx == -1 {
		return "", "", fmt.Errorf("template missing '---' separator line")
	}

	bodyLines := lines[separatorIdx+1:]
	body = strings.TrimPrefix(strings.Join(bodyLines, "\n"), "\n")
	body = strings.TrimRight(body, "\r\n") + "\n"

	return subject, body, nil
}
