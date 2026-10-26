package crowdsec

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A file that cannot be opened, such as one removed after the walk found it, is not reported as a whitelist owner.
// A file that opens and declares the section is reported, so the false result is not a blanket answer.
func TestHasWhitelistSection_UnopenableFileReportsFalse(t *testing.T) {
	dir := t.TempDir()
	missing := filepath.Join(dir, "removed-parser.yaml")
	assert.False(t, hasWhitelistSection(missing))

	present := filepath.Join(dir, "present-parser.yaml")
	require.NoError(t, os.WriteFile(present, []byte("whitelist:\n  ips: []\n"), 0o600))
	assert.True(t, hasWhitelistSection(present))
}
