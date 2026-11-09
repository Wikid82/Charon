package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/models"
)

// countingApplier records how many times a rule set write triggered a proxy config apply.
type countingApplier struct{ calls int }

func (c *countingApplier) ApplyConfig(_ context.Context) error {
	c.calls++
	return nil
}

func setupRulesetRequiredRouter(t *testing.T) (*gin.Engine, *gorm.DB, *countingApplier) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "rules_required.db")), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	require.NoError(t, db.AutoMigrate(&models.SecurityConfig{}, &models.SecurityAudit{}, &models.SecurityRuleSet{}))

	applier := &countingApplier{}
	h := NewSecurityHandler(config.SecurityConfig{}, db, applier)
	t.Cleanup(h.Close)

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("role", "admin")
		c.Next()
	})
	r.POST("/api/v1/security/rulesets", h.UpsertRuleSet)
	return r, db, applier
}

func postRuleset(t *testing.T, r *gin.Engine, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/security/rulesets", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	r.ServeHTTP(resp, req)
	return resp
}

func TestSecurityHandler_UpsertRuleSet_RefusesRulelessContent(t *testing.T) {
	cases := map[string]string{
		"comment only":    `{"name":"owasp-crs","content":"# nothing here"}`,
		"blank":           `{"name":"owasp-crs","content":"   "}`,
		"source url only": `{"name":"owasp-crs","source_url":"https://example.com/crs.tar.gz"}`,
		"engine only":     `{"name":"owasp-crs","content":"SecRuleEngine On"}`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			r, db, applier := setupRulesetRequiredRouter(t)

			resp := postRuleset(t, r, body)
			require.Equal(t, http.StatusBadRequest, resp.Code)

			var got map[string]string
			require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &got))
			assert.Equal(t, "ruleset_empty", got["error_code"])
			assert.Equal(t, "A rule set needs at least one rule. Add a SecRule line.", got["error"])

			var count int64
			require.NoError(t, db.Model(&models.SecurityRuleSet{}).Count(&count).Error)
			assert.Zero(t, count, "a refused rule set must not be stored")
			assert.Zero(t, applier.calls, "a refused rule set must not trigger a proxy apply")
		})
	}
}

func TestSecurityHandler_UpsertRuleSet_UpdateBlankingContentIsRefused(t *testing.T) {
	r, db, applier := setupRulesetRequiredRouter(t)

	resp := postRuleset(t, r, `{"name":"owasp-crs","content":"SecRule ARGS \"@contains a\" \"id:1,deny\""}`)
	require.Equal(t, http.StatusOK, resp.Code)
	require.Equal(t, 1, applier.calls)

	resp = postRuleset(t, r, `{"name":"owasp-crs","content":"# cleared"}`)
	require.Equal(t, http.StatusBadRequest, resp.Code)
	assert.Contains(t, resp.Body.String(), `"error_code":"ruleset_empty"`)
	assert.Equal(t, 1, applier.calls, "the refused update must not trigger a proxy apply")

	var stored models.SecurityRuleSet
	require.NoError(t, db.Where("name = ?", "owasp-crs").First(&stored).Error)
	assert.Contains(t, stored.Content, "id:1,deny")
}

func TestSecurityHandler_UpsertRuleSet_AcceptsRuleLine(t *testing.T) {
	r, _, applier := setupRulesetRequiredRouter(t)

	resp := postRuleset(t, r, `{"name":"owasp-crs","content":"SecAction \"id:900,pass\""}`)
	assert.Equal(t, http.StatusOK, resp.Code)
	assert.Equal(t, 1, applier.calls)
}
