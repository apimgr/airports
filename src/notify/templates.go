// Package notify implements operator email notifications per AI.md PART 17
// "EMAIL & NOTIFICATIONS": SMTP autodetection, connection testing, template
// loading/rendering, and event-gated sending. airports has no user accounts,
// so every email here is an operator notification (never an account email).
package notify

import (
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"

	emailtemplates "github.com/apimgr/airports/src/server/template/email"
)

var (
	templateVariablePattern = regexp.MustCompile(`\{([a-z0-9_]+)\}`)
	knownTemplateVarsOnce   sync.Once
	knownTemplateVars       map[string]struct{}
	warnedTemplateVars      sync.Map
)

// LoadTemplate resolves the subject/body pair for name per AI.md PART 17
// "Template Storage": a custom override at {configDir}/template/email/{name}.txt
// wins if present, otherwise the embedded default is used. configDir may be
// empty, in which case only the embedded default is consulted.
func LoadTemplate(configDir, name string) (subject, body string, err error) {
	if name == "" || filepath.Base(name) != name || strings.ContainsAny(name, `/\\`) {
		return "", "", fmt.Errorf("invalid email template name %q", name)
	}
	if configDir != "" {
		customPath := filepath.Join(configDir, "template", "email", name+".txt")
		if data, readErr := os.ReadFile(customPath); readErr == nil {
			subject, body, parseErr := parseTemplate(string(data))
			if parseErr == nil {
				warnUnknownTemplateVariables(name, subject, body)
			}
			return subject, body, parseErr
		}
	}

	data, err := emailtemplates.Templates.ReadFile(name + ".txt")
	if err != nil {
		return "", "", fmt.Errorf("no default template embedded for %q: %w", name, err)
	}
	return parseTemplate(string(data))
}

func warnUnknownTemplateVariables(templateName string, parts ...string) {
	knownTemplateVarsOnce.Do(loadKnownTemplateVariables)
	for _, part := range parts {
		for _, match := range templateVariablePattern.FindAllStringSubmatch(part, -1) {
			variable := match[1]
			if _, known := knownTemplateVars[variable]; known {
				continue
			}
			key := templateName + ":" + variable
			if _, warned := warnedTemplateVars.LoadOrStore(key, struct{}{}); warned {
				continue
			}
			log.Printf("Warning: email template %q uses unknown variable {%s}; did you mean {%s}?", templateName, variable, closestTemplateVariable(variable))
		}
	}
}

func loadKnownTemplateVariables() {
	knownTemplateVars = make(map[string]struct{})
	for _, variable := range []string{
		"app_name", "app_url", "fqdn", "onion_url", "onion_address", "i2p_url", "i2p_address",
		"notification_reply_to", "timestamp", "year",
	} {
		knownTemplateVars[variable] = struct{}{}
	}
	entries, err := fs.ReadDir(emailtemplates.Templates, ".")
	if err != nil {
		return
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		data, err := emailtemplates.Templates.ReadFile(entry.Name())
		if err != nil {
			continue
		}
		for _, match := range templateVariablePattern.FindAllStringSubmatch(string(data), -1) {
			knownTemplateVars[match[1]] = struct{}{}
		}
	}
}

func closestTemplateVariable(unknown string) string {
	known := make([]string, 0, len(knownTemplateVars))
	for candidate := range knownTemplateVars {
		known = append(known, candidate)
	}
	sort.Strings(known)
	closest := "fqdn"
	best := len(unknown) + len(closest)
	for _, candidate := range known {
		if distance := templateVariableDistance(unknown, candidate); distance < best {
			closest, best = candidate, distance
		}
	}
	return closest
}

func templateVariableDistance(a, b string) int {
	previous := make([]int, len(b)+1)
	for j := range previous {
		previous[j] = j
	}
	for i := 1; i <= len(a); i++ {
		current := make([]int, len(b)+1)
		current[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 0
			if a[i-1] != b[j-1] {
				cost = 1
			}
			current[j] = min(current[j-1]+1, previous[j]+1, previous[j-1]+cost)
		}
		previous = current
	}
	return previous[len(b)]
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
	if subject == "" {
		return "", "", fmt.Errorf("subject cannot be empty")
	}

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
	if strings.TrimSpace(body) == "" {
		return "", "", fmt.Errorf("body cannot be empty")
	}
	if err := validateTemplateVariables(subject, body); err != nil {
		return "", "", err
	}

	return subject, body, nil
}

// validateTemplateVariables checks that every brace-delimited token follows
// the documented {variable_name} format. Whether a valid variable is known
// depends on the notification event and is checked by the editor when present.
func validateTemplateVariables(parts ...string) error {
	for _, part := range parts {
		for i := 0; i < len(part); {
			open := strings.IndexByte(part[i:], '{')
			close := strings.IndexByte(part[i:], '}')
			if close >= 0 && (open < 0 || close < open) {
				return fmt.Errorf("invalid template syntax: unmatched }")
			}
			if open < 0 {
				break
			}
			open += i
			end := strings.IndexByte(part[open+1:], '}')
			if end < 0 {
				return fmt.Errorf("invalid template syntax: unmatched {")
			}
			end += open + 1
			name := part[open+1 : end]
			if name == "" {
				return fmt.Errorf("invalid template syntax: empty variable")
			}
			for _, ch := range name {
				if (ch < 'a' || ch > 'z') && (ch < '0' || ch > '9') && ch != '_' {
					return fmt.Errorf("invalid template syntax: invalid variable {%s}", name)
				}
			}
			i = end + 1
		}
	}
	return nil
}
