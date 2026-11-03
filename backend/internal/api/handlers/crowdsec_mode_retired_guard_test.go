package handlers

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// retiredModeTokens are the spellings of the retired CrowdSec mode setting that must not appear outside the files
// allowed to name it. The literal is the stored key; the identifier is the Go constant that holds it.
var retiredModeTokens = []struct {
	token   string
	allowed []string
}{
	{token: `"security.crowdsec.mode"`, allowed: []string{"internal/models/crowdsec_setting.go"}},
	{token: "CrowdSecRetiredModeSettingKey", allowed: []string{"internal/models/crowdsec_setting.go", "internal/api/handlers/settings_handler.go"}},
}

// backendRoot walks up from the test's working directory to the directory that holds go.mod.
func backendRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	require.NoError(t, err)
	for {
		if _, statErr := os.Stat(filepath.Join(dir, "go.mod")); statErr == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		require.NotEqual(t, parent, dir, "go.mod not found above the test directory")
		dir = parent
	}
}

// TestRetiredCrowdSecModeSetting_NoNewWriterOrReader fails when a non-test backend source names the retired mode
// setting outside the files that may: the migration, and the settings API refusal. A writer or a reader added later
// would bring the old decision back.
func TestRetiredCrowdSecModeSetting_NoNewWriterOrReader(t *testing.T) {
	root := backendRoot(t)

	var offenders []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			if d.Name() == "testdata" || d.Name() == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		rel = filepath.ToSlash(rel)

		content, readErr := os.ReadFile(path) //nolint:gosec // G304: path comes from walking the repository's own source tree
		if readErr != nil {
			return readErr
		}
		for _, entry := range retiredModeTokens {
			if !strings.Contains(string(content), entry.token) {
				continue
			}
			if !isAllowedRetiredModeFile(rel, entry.allowed) {
				offenders = append(offenders, rel+" names "+entry.token)
			}
		}
		return nil
	})
	require.NoError(t, err)
	assert.Empty(t, offenders, "the retired CrowdSec mode setting may be named only by the migration and the settings refusal")
}

// isAllowedRetiredModeFile reports whether rel, a slash path relative to the backend root, is one of allowed.
func isAllowedRetiredModeFile(rel string, allowed []string) bool {
	for _, a := range allowed {
		if rel == a {
			return true
		}
	}
	return false
}
