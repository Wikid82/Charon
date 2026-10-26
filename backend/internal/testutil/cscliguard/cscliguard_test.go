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
