package models

import "strings"

// utf8BOM is stripped from the start of rule set content before the first line is read.
const utf8BOM = "\uFEFF"

// HasRuleLines reports whether a rule set has at least one line that Coraza reads as a rule: a SecRule, SecAction or
// Include line. Blank lines and comment lines (after trimming whitespace, a line starting with '#') do not count, and
// neither do engine or other directives such as SecRuleEngine or SecRuleRemoveById, which only match the start of the
// directive name. A line that ends in a backslash continues onto the next line; a continuation of a non-rule line is
// never a rule, while a continued rule line counts by its first line. Comment lines are never continued. The scan has
// no per-line size limit because legacy rows may be larger than the save cap.
func HasRuleLines(content string) bool {
	return scanDirectiveLines(content, isRuleDirectiveName)
}

// scanDirectiveLines reports whether match accepts the directive name of any line that starts a directive. Blank and
// comment lines are skipped, a line that ends in a backslash continues onto the next line, and comment lines are never
// continued, so a comment ending in a backslash cannot hide the next line. The directive name is the first word of the
// trimmed line, ending at a space or tab.
func scanDirectiveLines(content string, match func(name string) bool) bool {
	content = strings.TrimPrefix(content, utf8BOM)
	continued := false
	for rest := content; ; {
		line, after, more := strings.Cut(rest, "\n")
		trimmed := strings.TrimSpace(line)
		switch {
		case continued:
			continued = strings.HasSuffix(trimmed, `\`)
		case trimmed == "" || strings.HasPrefix(trimmed, "#"):
			// Blank and comment lines never continue.
		case match(directiveName(trimmed)):
			return true
		default:
			continued = strings.HasSuffix(trimmed, `\`)
		}
		if !more {
			return false
		}
		rest = after
	}
}

// directiveName returns the first word of a trimmed, non-comment line.
func directiveName(trimmed string) string {
	if i := strings.IndexAny(trimmed, " \t"); i >= 0 {
		return trimmed[:i]
	}
	return trimmed
}

// isRuleDirectiveName reports whether a directive name starts a rule: SecRule, SecAction or Include.
func isRuleDirectiveName(name string) bool {
	switch strings.ToLower(name) {
	case "secrule", "secaction", "include":
		return true
	}
	return false
}
