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
	content = strings.TrimPrefix(content, utf8BOM)
	continued := false
	for rest := content; ; {
		line, after, more := strings.Cut(rest, "\n")
		trimmed := strings.TrimSpace(line)
		switch {
		case continued:
			continued = strings.HasSuffix(trimmed, `\`)
		case trimmed == "" || strings.HasPrefix(trimmed, "#"):
			// Blank and comment lines never continue, so a comment ending in a backslash cannot hide the next line.
		case isRuleDirective(trimmed):
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

// isRuleDirective reports whether a trimmed, non-comment line starts with one of the directives that carry a rule.
// The directive name must end at a space or tab, so SecRuleEngine does not match SecRule.
func isRuleDirective(trimmed string) bool {
	name := trimmed
	if i := strings.IndexAny(trimmed, " \t"); i >= 0 {
		name = trimmed[:i]
	}
	switch strings.ToLower(name) {
	case "secrule", "secaction", "include":
		return true
	}
	return false
}
