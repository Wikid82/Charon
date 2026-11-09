package cscliguard

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeShellFile writes src to a fresh temporary file named name and returns its path.
func writeShellFile(t *testing.T, name, src string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	require.NoError(t, os.WriteFile(path, []byte(src), 0o600))
	return path
}

// Each wrapper in front of a bare cscli start is still a start, so every line of the fixture is reported.
func TestShellOffendersFlagsStartsBehindWrappers(t *testing.T) {
	t.Parallel()
	offenders, err := ShellOffenders(shellFixture("wrappers.sh.txt"))
	require.NoError(t, err)
	assert.Equal(t, []string{
		"wrappers.sh.txt:2: sudo cscli machines list",
		"wrappers.sh.txt:3: sudo -E cscli machines list",
		"wrappers.sh.txt:4: sudo -u charon cscli machines list",
		"wrappers.sh.txt:5: su-exec charon cscli machines list",
		"wrappers.sh.txt:6: env CHARON_X=1 cscli machines list",
		"wrappers.sh.txt:7: env -i PATH=/bin cscli machines list",
		"wrappers.sh.txt:8: nohup cscli hub update &",
		"wrappers.sh.txt:9: command cscli machines list",
		"wrappers.sh.txt:10: command -p cscli machines list",
		"wrappers.sh.txt:11: exec cscli machines list",
		"wrappers.sh.txt:12: xargs cscli machines delete",
		"wrappers.sh.txt:13: xargs -I {} cscli bouncers delete {}",
		"wrappers.sh.txt:14: echo ok && sudo cscli bouncers list",
		"wrappers.sh.txt:15: run_interruptible sudo cscli hub upgrade",
		"wrappers.sh.txt:16: timeout 60 sudo cscli hub update",
		"wrappers.sh.txt:17: >/dev/null cscli machines list",
		"wrappers.sh.txt:18: 2>&1 cscli machines list",
		"wrappers.sh.txt:19: x=$(sudo cscli machines list)",
	}, offenders)
}

// A start right after a pipe, a list operator, a subshell, a brace group, a negation or a backquote is reported,
// with or without the blank that would normally separate it.
func TestShellOffendersFlagsStartsAfterSeparators(t *testing.T) {
	t.Parallel()
	offenders, err := ShellOffenders(shellFixture("separators.sh.txt"))
	require.NoError(t, err)
	assert.Equal(t, []string{
		"separators.sh.txt:2: true|cscli machines list",
		"separators.sh.txt:3: true&cscli machines list",
		"separators.sh.txt:4: true&&cscli machines list",
		"separators.sh.txt:5: false||cscli machines list",
		"separators.sh.txt:6: true;cscli machines list",
		"separators.sh.txt:7: (cscli machines list)",
		"separators.sh.txt:8: { cscli machines list; }",
		"separators.sh.txt:9: ! cscli machines list",
		"separators.sh.txt:10: x=`cscli machines list`",
	}, offenders)
}

// Lookups, the version probe, configured starts and a cscli_cfg wrapper behind a wrapper are not starts. A wrapper
// word that is only an argument of an echo does not start anything either.
func TestShellOffendersAcceptsWrapperLookupsAndConfiguredStarts(t *testing.T) {
	t.Parallel()
	offenders, err := ShellOffenders(shellFixture("wrapped.sh.txt"))
	require.NoError(t, err)
	assert.Empty(t, offenders)
}

// The last logical line is flushed even when the file ends in a continuation with no newline after it.
func TestShellOffendersFlushesContinuationAtEndOfFile(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		src  string
		want []string
	}{
		{
			name: "bare start continued to end of file",
			src:  "#!/bin/sh\necho ok\ncscli machines list \\",
			want: []string{"x.sh:3: cscli machines list"},
		},
		{
			name: "continued start joins its parts",
			src:  "#!/bin/sh\ncscli \\\n    machines \\",
			want: []string{"x.sh:2: cscli machines"},
		},
		{
			name: "continuation with no cscli is clean",
			src:  "#!/bin/sh\necho ok \\",
			want: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			offenders, err := ShellOffenders(writeShellFile(t, "x.sh", tt.src))
			require.NoError(t, err)
			assert.Equal(t, tt.want, offenders)
		})
	}
}

// A backslash inside double quotes escapes the next character, so an escaped quote does not close the string.
// The cscli words after it are quoted text and never a start, while a start after the real close is reported.
func TestShellOffendersHonoursBackslashEscapesInDoubleQuotes(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		src  string
		want []string
	}{
		{
			name: "escaped quote keeps the string open",
			src:  `echo "a\" cscli machines list"`,
			want: nil,
		},
		{
			name: "start after a string with an escaped quote is reported",
			src:  `echo "a\"b" && cscli machines list`,
			want: []string{`x.sh:1: echo "a\"b" && cscli machines list`},
		},
		{
			name: "escaped backslash does not escape the closing quote",
			src:  `echo "x\\" ; cscli machines list`,
			want: []string{`x.sh:1: echo "x\\" ; cscli machines list`},
		},
		{
			name: "unterminated quote at end of file is not a start",
			src:  `echo "cscli machines list\`,
			want: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			offenders, err := ShellOffenders(writeShellFile(t, "x.sh", tt.src+"\n"))
			require.NoError(t, err)
			assert.Equal(t, tt.want, offenders)
		})
	}
}
