package models_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/Wikid82/charon/backend/internal/models"
)

func TestHasRuleLines(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    bool
	}{
		{name: "empty", content: "", want: false},
		{name: "blank lines and spaces", content: "   \n\t\n \r\n", want: false},
		{name: "comment only", content: "# SecRule ARGS x id:1,deny", want: false},
		{name: "indented comment", content: "    # SecRule ARGS x id:1,deny", want: false},
		{name: "comment ending in backslash does not swallow the next line", content: "# note \\\nSecRule ARGS x id:1,deny", want: true},
		{name: "crlf line endings", content: "SecRule ARGS x id:1,deny\r\n", want: true},
		{name: "crlf engine only", content: "SecRuleEngine On\r\n", want: false},
		{name: "utf-8 bom before rule", content: "\uFEFFSecRule ARGS x id:1,deny", want: true},
		{name: "engine directive is not a rule", content: "SecRuleEngine On", want: false},
		{name: "engine and body access only", content: "SecRuleEngine On\nSecRequestBodyAccess On\n", want: false},
		{name: "remove by id is not a rule", content: "SecRuleRemoveById 1", want: false},
		{name: "update target by id is not a rule", content: "SecRuleUpdateTargetById 1 ARGS", want: false},
		{name: "default action is not a rule", content: "SecDefaultAction phase:1,deny", want: false},
		{name: "marker is not a rule", content: "SecMarker END", want: false},
		{name: "includes is not include", content: "Includes foo", want: false},
		{name: "mixed case secrule", content: "SECRULE ARGS x id:1,deny", want: true},
		{name: "lowercase secrule", content: "secrule ARGS x id:1,deny", want: true},
		{name: "secaction", content: "SecAction \"id:900,pass\"", want: true},
		{name: "secaction followed by tab", content: "SecAction\tid:900,pass", want: true},
		{name: "include followed by tab", content: "Include\t/etc/rules.conf", want: true},
		{name: "include", content: "Include /etc/rules.conf", want: true},
		{name: "continued rule counts by its first line", content: "SecRule ARGS x \\\n id:1,deny", want: true},
		{name: "continuation of a non-rule line is not a rule", content: "SecRuleEngine On \\\nSecRule ARGS x id:1,deny", want: false},
		{name: "continuation chain of a non-rule line", content: "SecRuleEngine On \\\nid:1 \\\nSecRule ARGS x id:1,deny", want: false},
		{name: "leading whitespace before rule", content: "   \t SecRule ARGS x id:1,deny", want: true},
		{name: "rule after comment and blank lines", content: "# header\n\n   \nSecRule ARGS x id:1,deny", want: true},
		{name: "single line over 1 MiB", content: "SecRule ARGS x " + strings.Repeat("a", 1<<20+1), want: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, models.HasRuleLines(tc.content))
		})
	}
}
