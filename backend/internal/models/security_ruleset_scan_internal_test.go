package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestScanDirectiveLines_MatchesOnDirectiveName describes the shared scanner: the matcher sees the first word of each
// directive line, and blank, comment and continued lines are never offered to it.
func TestScanDirectiveLines_MatchesOnDirectiveName(t *testing.T) {
	isOnlyFoo := func(name string) bool { return name == "Foo" }
	tests := []struct {
		name    string
		content string
		want    bool
	}{
		{name: "first word matches", content: "Foo bar", want: true},
		{name: "tab ends the name", content: "Foo\tbar", want: true},
		{name: "name that only starts with the match", content: "FooBar baz", want: false},
		{name: "comment mentioning the name", content: "# Foo here", want: false},
		{name: "continued line is not a directive", content: "Bar \\\nFoo x", want: false},
		{name: "directive after a comment ending in a backslash", content: "# note \\\nFoo x", want: true},
		{name: "bare name with no argument", content: "Foo", want: true},
		{name: "byte order mark is stripped", content: utf8BOM + "Foo", want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, scanDirectiveLines(tt.content, isOnlyFoo))
		})
	}
}

func TestDirectiveName(t *testing.T) {
	assert.Equal(t, "SecRule", directiveName("SecRule ARGS"))
	assert.Equal(t, "SecRule", directiveName("SecRule\tARGS"))
	assert.Equal(t, "Include", directiveName("Include"))
}
