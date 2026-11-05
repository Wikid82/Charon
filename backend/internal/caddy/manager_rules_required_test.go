package caddy

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/models"
)

const emptyRulesetLogMessage = "skipping WAF rule set that has no rules"

// newRulesRequiredManager builds a manager against a mock Caddy admin API and a file-backed database that holds the
// given rule sets. It returns the manager and the directory the rule set files are written to.
func newRulesRequiredManager(t *testing.T, rulesets []models.SecurityRuleSet) (manager *Manager, rulesetDir string) {
	t.Helper()
	caddyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/load" && r.Method == http.MethodPost {
			var cfg Config
			if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(caddyServer.Close)

	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "rules_required.db")), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.ProxyHost{}, &models.Location{}, &models.Setting{}, &models.CaddyConfig{}, &models.SSLCertificate{}, &models.AccessList{}, &models.SecurityConfig{}, &models.SecurityDecision{}, &models.SecurityRuleSet{}))
	for i := range rulesets {
		rulesets[i].UUID = uuid.NewString()
		require.NoError(t, db.Create(&rulesets[i]).Error)
	}

	configDir := t.TempDir()
	manager = NewManager(newTestClient(t, caddyServer.URL), db, configDir, "", false, config.SecurityConfig{})
	return manager, filepath.Join(configDir, "coraza", "rulesets")
}

// captureLogs points the global logger at a buffer for the duration of the test and returns it.
func captureLogs(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	logger.Init(false, &buf)
	t.Cleanup(func() { logger.Init(false, io.Discard) })
	return &buf
}

func TestManager_ApplyConfig_SkipsRulesetWithoutRules(t *testing.T) {
	manager, rulesetDir := newRulesRequiredManager(t, []models.SecurityRuleSet{
		{Name: "empty-crs", Content: "# no rules here"},
		{Name: "valid-crs", Content: rulesRequiredValidRule},
	})
	require.NoError(t, os.MkdirAll(rulesetDir, 0o700))
	// A file left by an earlier apply, from when the empty set was still written.
	staleFile := filepath.Join(rulesetDir, "empty-crs-deadbeef.conf")
	require.NoError(t, os.WriteFile(staleFile, []byte("# stale"), 0o600))
	logs := captureLogs(t)

	require.NoError(t, manager.ApplyConfig(context.Background()))
	require.NoError(t, manager.ApplyConfig(context.Background()))

	emptyFiles, err := filepath.Glob(filepath.Join(rulesetDir, "empty-crs-*"))
	require.NoError(t, err)
	assert.Empty(t, emptyFiles, "a rule set without rules must not get a file, and the stale file must be removed")
	validFiles, err := filepath.Glob(filepath.Join(rulesetDir, "valid-crs-*.conf"))
	require.NoError(t, err)
	assert.Len(t, validFiles, 1, "the valid rule set must still be written")

	// One Warn per skipped set per apply: two applies, one skipped set each.
	assert.Equal(t, 2, strings.Count(logs.String(), emptyRulesetLogMessage))
	assert.Contains(t, logs.String(), `"ruleset":"empty-crs"`)
}

func TestManager_ApplyConfig_OnlyEmptyRulesetWritesNoFile(t *testing.T) {
	manager, rulesetDir := newRulesRequiredManager(t, []models.SecurityRuleSet{
		{Name: "empty-crs", Content: "SecRuleEngine On\n"},
	})
	logs := captureLogs(t)

	require.NoError(t, manager.ApplyConfig(context.Background()))

	entries, _ := os.ReadDir(rulesetDir)
	assert.Empty(t, entries)
	assert.Equal(t, 1, strings.Count(logs.String(), emptyRulesetLogMessage))
}
