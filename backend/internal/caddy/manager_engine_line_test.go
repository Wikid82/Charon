package caddy

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/models"
)

// applyRulesetAndCaptureWrite applies one proxy host using a rule set with the given content and returns the text
// written to that rule set's file. The test name keeps the in-memory database separate per test.
func applyRulesetAndCaptureWrite(t *testing.T, ruleName, content string) string {
	t.Helper()
	tmp := t.TempDir()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.ProxyHost{}, &models.Location{}, &models.Setting{}, &models.CaddyConfig{}, &models.SSLCertificate{}, &models.SecurityConfig{}, &models.SecurityRuleSet{}))

	host := models.ProxyHost{DomainNames: ruleName + ".example.com", ForwardHost: "127.0.0.1", ForwardPort: 8080, Enabled: true}
	require.NoError(t, db.Create(&host).Error)
	require.NoError(t, db.Create(&models.SecurityRuleSet{Name: ruleName, Content: content}).Error)
	sec := models.SecurityConfig{Name: "default", Enabled: true, AdminWhitelist: "10.0.0.1/32", WAFMode: "block", WAFRulesSource: ruleName}
	require.NoError(t, db.Create(&sec).Error)

	caddyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/config/" && r.Method == http.MethodGet {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"apps":{"http":{}}}`))
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(caddyServer.Close)

	var written []byte
	origWrite := writeFileFunc
	writeFileFunc = func(path string, b []byte, perm os.FileMode) error {
		if strings.Contains(path, ruleName) {
			written = b
		}
		return origWrite(path, b, perm)
	}
	t.Cleanup(func() { writeFileFunc = origWrite })

	manager := NewManager(newTestClient(t, caddyServer.URL), db, tmp, "", false, config.SecurityConfig{CerberusEnabled: true, WAFMode: "block"})
	require.NoError(t, manager.ApplyConfig(context.Background()))
	require.NotNil(t, written, "rule set file should be written")
	return string(written)
}

// TestManager_ApplyConfig_EngineMentionInCommentStillPrependsEngine describes a rule set whose only SecRuleEngine
// mention is a comment: the engine line is not the rule set's own, so Charon prepends its line.
func TestManager_ApplyConfig_EngineMentionInCommentStillPrependsEngine(t *testing.T) {
	content := "# SecRuleEngine is managed by Charon\nSecRule REQUEST_BODY \"<script>\" \"id:12345,phase:2,deny\""
	written := applyRulesetAndCaptureWrite(t, "comment-mention", content)
	assert.True(t, strings.HasPrefix(written, "SecRuleEngine On\n"), "engine line should be prepended, got: %q", written)
	assert.Contains(t, written, content)
}

// TestManager_ApplyConfig_OwnEngineLineIsKept describes a rule set with its own SecRuleEngine line: Charon adds nothing.
func TestManager_ApplyConfig_OwnEngineLineIsKept(t *testing.T) {
	content := "# tuned\nSecRuleEngine DetectionOnly\nSecRule REQUEST_BODY \"<script>\" \"id:12345,phase:2,deny\""
	written := applyRulesetAndCaptureWrite(t, "own-engine", content)
	assert.Equal(t, content, written)
}
