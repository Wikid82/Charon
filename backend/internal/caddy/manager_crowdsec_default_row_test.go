package caddy

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// applyAndCaptureLoad runs ApplyConfig against a fake Caddy admin endpoint and returns the decoded /load payload.
func applyAndCaptureLoad(t *testing.T, db *gorm.DB, secCfg config.SecurityConfig) *Config {
	t.Helper()
	loads := make(chan []byte, 1)
	caddyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/load" && r.Method == http.MethodPost {
			body, _ := io.ReadAll(r.Body)
			loads <- body
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(caddyServer.Close)

	manager := NewManager(newTestClient(t, caddyServer.URL), db, t.TempDir(), "", false, secCfg)
	require.NoError(t, manager.ApplyConfig(context.Background()))

	var cfg Config
	require.NoError(t, json.Unmarshal(<-loads, &cfg))
	return &cfg
}

// newCrowdSecLookupDB opens an isolated in-memory database with the tables ApplyConfig reads.
func newCrowdSecLookupDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.ProxyHost{}, &models.Location{}, &models.Setting{}, &models.CaddyConfig{}, &models.SSLCertificate{}, &models.AccessList{}, &models.SecurityConfig{}))
	return db
}

// TestManager_ApplyConfig_DefaultSecurityRowLocal_AttachesCrowdSec is the #1727 regression at the proxy:
// the row the proxy reads (name "default", mode local) with a bouncer key must put apps.crowdsec in the config.
func TestManager_ApplyConfig_DefaultSecurityRowLocal_AttachesCrowdSec(t *testing.T) {
	withTestBouncerKey(t)
	db := newCrowdSecLookupDB(t)
	require.NoError(t, db.Create(&models.SecurityConfig{UUID: "sc-default", Name: models.DefaultSecurityConfigName, Enabled: true, CrowdSecMode: "local"}).Error)

	cfg := applyAndCaptureLoad(t, db, config.SecurityConfig{CerberusEnabled: true, CrowdSecMode: "disabled"})

	require.NotNil(t, cfg.Apps.CrowdSec, "apps.crowdsec must be present for the default row in local mode with a key")
	require.Equal(t, "test-env-key", cfg.Apps.CrowdSec.APIKey)
}

// TestManager_ApplyConfig_WrongNamedSecurityRow_DoesNotAttachCrowdSec pins the lookup: a row under any other
// name is never read by the proxy, so it cannot switch CrowdSec on.
func TestManager_ApplyConfig_WrongNamedSecurityRow_DoesNotAttachCrowdSec(t *testing.T) {
	withTestBouncerKey(t)
	db := newCrowdSecLookupDB(t)
	require.NoError(t, db.Create(&models.SecurityConfig{UUID: "sc-legacy", Name: "Default Security Config", Enabled: true, CrowdSecMode: "local"}).Error)

	cfg := applyAndCaptureLoad(t, db, config.SecurityConfig{CerberusEnabled: true, CrowdSecMode: "disabled"})

	require.Nil(t, cfg.Apps.CrowdSec)
}
