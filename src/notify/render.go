package notify

import "strings"

// Render substitutes every {variable} token present in vars into both
// subject and body per AI.md PART 17 "Template Format". Tokens with no
// matching key in vars are left untouched as literal text, matching the
// spec's "Variables: {variable_name} syntax" with no error on unknowns
// (validation of unknown variables is an editor-only concern this project
// has no admin UI for, per binary-rules.md's "no admin web UI" rule).
func Render(subject, body string, vars map[string]string) (string, string) {
	replacements := make([]string, 0, len(vars)*2)
	for key, value := range vars {
		replacements = append(replacements, "{"+key+"}", value)
	}
	replacer := strings.NewReplacer(replacements...)
	return replacer.Replace(subject), replacer.Replace(body)
}
