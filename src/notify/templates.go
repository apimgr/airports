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
	"time"

	"github.com/apimgr/airports/src/common/i18n"
	"github.com/apimgr/airports/src/config"
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
func LoadEmailTemplate(configDir, name string) (subject, body string, err error) {
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
	subject, body, err = parseTemplate(string(data))
	if err == nil {
		warnUnknownTemplateVariables(name, subject, body)
	}
	return subject, body, err
}

// EmailTemplatePreview contains one rendered template and any non-blocking
// validation warnings for the CLI dry-run preview.
type EmailTemplatePreview struct {
	Name     string
	Subject  string
	Body     string
	Warnings []string
}

// EmailTemplateNames returns embedded defaults and file overrides in stable
// name order. An override-only template is included in the preview too.
func EmailTemplateNames(configDir string) ([]string, error) {
	names := make(map[string]struct{})
	entries, err := fs.ReadDir(emailtemplates.Templates, ".")
	if err != nil {
		return nil, fmt.Errorf("read embedded email templates: %w", err)
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".txt" {
			continue
		}
		names[strings.TrimSuffix(entry.Name(), ".txt")] = struct{}{}
	}
	if configDir != "" {
		overrideDir := filepath.Join(configDir, "template", "email")
		overrides, readErr := os.ReadDir(overrideDir)
		if readErr != nil && !os.IsNotExist(readErr) {
			return nil, fmt.Errorf("read email template overrides: %w", readErr)
		}
		for _, entry := range overrides {
			if entry.IsDir() || filepath.Ext(entry.Name()) != ".txt" {
				continue
			}
			name := strings.TrimSuffix(entry.Name(), ".txt")
			if name != "" && filepath.Base(name) == name && !strings.ContainsAny(name, `/\\`) {
				names[name] = struct{}{}
			}
		}
	}
	result := make([]string, 0, len(names))
	for name := range names {
		result = append(result, name)
	}
	sort.Strings(result)
	return result, nil
}

// PreviewEmailTemplates loads, validates and renders every template using the
// PART 17 sample values. Unknown variables are reported as warnings and remain
// visible in the rendered output so an operator can correct the file.
func PreviewEmailTemplates(configDir string, cfg *config.ApplicationConfig) ([]EmailTemplatePreview, error) {
	names, err := EmailTemplateNames(configDir)
	if err != nil {
		return nil, err
	}
	variables := previewEmailVariables(cfg)
	previews := make([]EmailTemplatePreview, 0, len(names))
	for _, name := range names {
		subjectTemplate, bodyTemplate, err := LoadEmailTemplate(configDir, name)
		if err != nil {
			return nil, fmt.Errorf("template %q: %w", name, err)
		}
		warnings := unknownEmailTemplateVariableWarnings(subjectTemplate, bodyTemplate)
		if len(subjectTemplate) > 78 {
			warnings = append(warnings, "Subject is longer than 78 characters")
		}
		subjectTemplate, bodyTemplate = TranslateEmailTemplate(configDir, name, subjectTemplate, bodyTemplate, cfg.Server.I18n.DefaultLanguage, variables)
		subject, body := Render(subjectTemplate, bodyTemplate, variables)
		previews = append(previews, EmailTemplatePreview{Name: name, Subject: subject, Body: body, Warnings: warnings})
	}
	return previews, nil
}

// TranslateEmailTemplate resolves embedded template text through locale keys
// before plain variable substitution. Operator overrides remain literal.
func TranslateEmailTemplate(configDir, name, subject, body, lang string, variables map[string]string) (string, string) {
	if configDir != "" {
		if _, err := os.Stat(filepath.Join(configDir, "template", "email", name+".txt")); err == nil {
			return subject, body
		}
	}
	if lang == "" {
		lang = "en"
	}
	args := make([]interface{}, 0, len(variables)*2)
	for name, value := range variables {
		args = append(args, name, value)
	}
	return i18n.TranslateFormattedText(lang, "email.templates."+name+".subject", args...),
		i18n.TranslateFormattedText(lang, "email.templates."+name+".body", args...)
}

func previewEmailVariables(cfg *config.ApplicationConfig) map[string]string {
	variables := GlobalVars(cfg)
	variables["ip"] = "192.168.1.100"
	variables["timestamp"] = time.Now().Format(time.RFC1123)
	variables["hostname"] = "airport-server"
	variables["version"] = "1.2.3"
	variables["mode"] = "production"
	variables["reason"] = "service stop"
	variables["path"] = "/var/lib/airports"
	variables["used_percent"] = "85"
	variables["free"] = "1.5 GB"
	variables["threshold_percent"] = "85"
	variables["error"] = "Sample error message"
	variables["since"] = time.Now().Add(-5 * time.Minute).Format(time.RFC1123)
	variables["filename"] = "airports-backup.tar.gz"
	variables["size"] = "10 MB"
	variables["expires_in"] = "7"
	variables["expiry_date"] = time.Now().Add(7 * 24 * time.Hour).Format("January 2, 2006")
	variables["valid_until"] = time.Now().AddDate(1, 0, 0).Format("January 2, 2006")
	variables["next_retry"] = time.Now().Add(time.Hour).Format(time.RFC1123)
	variables["task_name"] = "geoip_update"
	variables["next_run"] = time.Now().Add(time.Hour).Format(time.RFC1123)
	variables["event"] = "Sample security event"
	variables["details"] = "Sample event details"
	variables["old_version"] = "1.2.2"
	variables["new_version"] = "1.2.3"
	variables["current_version"] = "1.2.2"
	variables["channel"] = "stable"
	variables["tracking_id"] = "sec_0123456789abcdef"
	variables["status"] = "received"
	variables["status_url"] = "https://localhost/server/security/report/sec_0123456789abcdef"
	variables["contact_subject"] = "Sample contact request"
	variables["contact_message"] = "Sample contact message"
	variables["sender_name"] = "Sample Sender"
	variables["sender_email"] = "sender@example.test"
	variables["duration"] = "5 minutes"
	variables["manual_command"] = "airports --update yes"
	return variables
}

func unknownEmailTemplateVariableWarnings(parts ...string) []string {
	knownTemplateVarsOnce.Do(loadKnownTemplateVariables)
	unknown := make(map[string]struct{})
	for _, part := range parts {
		for _, match := range templateVariablePattern.FindAllStringSubmatch(part, -1) {
			if _, known := knownTemplateVars[match[1]]; !known {
				unknown[match[1]] = struct{}{}
			}
		}
	}
	variables := make([]string, 0, len(unknown))
	for variable := range unknown {
		variables = append(variables, variable)
	}
	sort.Strings(variables)
	warnings := make([]string, 0, len(variables))
	for _, variable := range variables {
		warnings = append(warnings, fmt.Sprintf("Unknown variable: {%s}. Did you mean {%s}?", variable, closestTemplateVariable(variable)))
	}
	return warnings
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
	if err := validateTemplateVariableSyntax(subject, 1); err != nil {
		return "", "", err
	}
	if err := validateTemplateVariableSyntax(body, separatorIdx+2); err != nil {
		return "", "", err
	}

	return subject, body, nil
}

// validateTemplateVariableSyntax checks brace-delimited tokens against the
// documented {variable_name} format and reports the source line on failure.
func validateTemplateVariableSyntax(part string, firstLine int) error {
	for i := 0; i < len(part); {
		open := strings.IndexByte(part[i:], '{')
		close := strings.IndexByte(part[i:], '}')
		if close >= 0 && (open < 0 || close < open) {
			line := firstLine + strings.Count(part[:i+close], "\n")
			return fmt.Errorf("invalid template syntax at line %d: unmatched }", line)
		}
		if open < 0 {
			return nil
		}
		open += i
		line := firstLine + strings.Count(part[:open], "\n")
		end := strings.IndexByte(part[open+1:], '}')
		if end < 0 {
			return fmt.Errorf("invalid template syntax at line %d: unmatched {", line)
		}
		end += open + 1
		name := part[open+1 : end]
		if name == "" {
			return fmt.Errorf("invalid template syntax at line %d: empty variable", line)
		}
		for _, ch := range name {
			if (ch < 'a' || ch > 'z') && (ch < '0' || ch > '9') && ch != '_' {
				return fmt.Errorf("invalid template syntax at line %d: invalid variable {%s}", line, name)
			}
		}
		i = end + 1
	}
	return nil
}
