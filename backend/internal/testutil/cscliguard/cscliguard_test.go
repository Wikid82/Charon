package cscliguard

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fixture returns the path of a testdata directory of this package.
func fixture(name string) string {
	return filepath.Join("testdata", name)
}

// The guard must find an offender of every start shape, or a passing check proves nothing.
func TestOffendersFlagsEveryStartShape(t *testing.T) {
	t.Parallel()
	offenders, err := Offenders(fixture("shapes"), nil, nil)
	require.NoError(t, err)
	assert.Equal(t, []string{
		"shapes.go: executeStart",
		"shapes.go: executeWithEnvStart",
		"shapes.go: execCommandStart",
		"shapes.go: execCommandContextStart",
		"shapes.go: exemptStart",
	}, offenders)
}

// A function is clean when its helper calls cover its starts, so only the function with too few calls is reported.
func TestOffendersCountsHelperCallsAgainstStarts(t *testing.T) {
	t.Parallel()
	offenders, err := Offenders(fixture("helpers"), []string{"withConfigFlag"}, nil)
	require.NoError(t, err)
	assert.Equal(t, []string{"helpers.go: twoStartsOneHelper"}, offenders)
}

func TestOffendersExemptFunctionsAreSkipped(t *testing.T) {
	t.Parallel()
	offenders, err := Offenders(fixture("shapes"), nil, map[string]bool{"executeStart": true, "exemptStart": true})
	require.NoError(t, err)
	assert.Equal(t, []string{
		"shapes.go: executeWithEnvStart",
		"shapes.go: execCommandStart",
		"shapes.go: execCommandContextStart",
	}, offenders)
}

// Test files and files without a start are never reported, and a directory with only clean files returns nothing.
func TestOffendersSkipsTestFiles(t *testing.T) {
	t.Parallel()
	offenders, err := Offenders(fixture("skips"), nil, nil)
	require.NoError(t, err)
	assert.Empty(t, offenders)
}

// An empty check would pass vacuously, so a directory with no Go files to check is an error.
func TestOffendersErrorsWithoutGoFiles(t *testing.T) {
	t.Parallel()
	_, err := Offenders(t.TempDir(), nil, nil)
	require.Error(t, err)
	assert.ErrorContains(t, err, "no Go files")
}

func TestOffendersErrorsOnMissingDirectory(t *testing.T) {
	t.Parallel()
	_, err := Offenders(filepath.Join(t.TempDir(), "missing"), nil, nil)
	require.ErrorContains(t, err, "read directory")
}

func TestOffendersErrorsOnUnparsableFile(t *testing.T) {
	t.Parallel()
	_, err := Offenders(fixture("broken"), nil, nil)
	require.ErrorContains(t, err, "parse")
}

func TestOffendersErrorsOnUnreadableFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.Symlink(filepath.Join(dir, "missing-target"), filepath.Join(dir, "dangling.go")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	_, err := Offenders(dir, nil, nil)
	require.ErrorContains(t, err, "read ")
}

// shellFixture returns the path of a shell fixture. The fixtures end in .sh.txt, so lint tools that match *.sh skip them.
func shellFixture(name string) string {
	return filepath.Join("testdata", "shell", name)
}

// Every executable form that starts cscli without -c is reported, with its line number and the logical line
// (a continued line is joined). A comment, a quoted message and a wrapper call are not.
func TestShellOffendersFlagsEveryBareStart(t *testing.T) {
	t.Parallel()
	offenders, err := ShellOffenders(shellFixture("bare.sh.txt"))
	require.NoError(t, err)
	assert.Equal(t, []string{
		"bare.sh.txt:2: cscli machines add -a --force",
		"bare.sh.txt:3: if cscli parsers inspect x >/dev/null 2>&1; then",
		"bare.sh.txt:6: timeout 60 cscli hub update",
		"bare.sh.txt:8: run_interruptible cscli collections list",
		"bare.sh.txt:9: x=$(cscli bouncers list)",
		"bare.sh.txt:10: cscli machines list",
	}, offenders)
}

// Calls that carry -c, the version probe, the wrappers and the command lookup are accepted, including a continued line.
func TestShellOffendersAcceptsConfiguredAndProbeCalls(t *testing.T) {
	t.Parallel()
	offenders, err := ShellOffenders(shellFixture("wrapped.sh.txt"))
	require.NoError(t, err)
	assert.Empty(t, offenders)
}

// A deliberate start carries the trailing marker with a reason and is accepted.
func TestShellOffendersAcceptsTheMarker(t *testing.T) {
	t.Parallel()
	offenders, err := ShellOffenders(shellFixture("marker.sh.txt"))
	require.NoError(t, err)
	assert.Empty(t, offenders)
}

// Comments and quoted messages never read as commands, so a message that names cscli is not an offender.
func TestShellOffendersIgnoresCommentsAndQuotedText(t *testing.T) {
	t.Parallel()
	offenders, err := ShellOffenders(shellFixture("comment_and_echo.sh.txt"))
	require.NoError(t, err)
	assert.Empty(t, offenders)
}

// A command substitution inside double quotes runs, so its cscli start is reported.
func TestShellOffendersReportsStartsInsideCommandSubstitution(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "subst.sh")
	require.NoError(t, os.WriteFile(path, []byte("echo \"$(cscli machines list)\"\n"), 0o600))
	offenders, err := ShellOffenders(path)
	require.NoError(t, err)
	assert.Equal(t, []string{"subst.sh:1: echo \"$(cscli machines list)\""}, offenders)
}

func TestShellOffendersErrorsOnMissingFile(t *testing.T) {
	t.Parallel()
	_, err := ShellOffenders(filepath.Join(t.TempDir(), "missing.sh"))
	require.ErrorContains(t, err, "read ")
}
