package caddy

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/models"
)

func TestDroppedHostUUIDs_ListsOnlyHostsMissingFromKept(t *testing.T) {
	t.Parallel()
	all := []models.ProxyHost{{UUID: "a"}, {UUID: "b"}, {UUID: "c"}}
	kept := []models.ProxyHost{{UUID: "a"}, {UUID: "c"}}
	assert.Equal(t, map[string]struct{}{"b": {}}, droppedHostUUIDs(all, kept))
	assert.Empty(t, droppedHostUUIDs(all, all))
	assert.Equal(t, map[string]struct{}{"a": {}, "b": {}, "c": {}}, droppedHostUUIDs(all, nil))
}

func TestLeftOutHostUUIDs_NilAndUnappliedManagerReportNone(t *testing.T) {
	t.Parallel()
	var nilManager *Manager
	assert.Nil(t, nilManager.LeftOutHostUUIDs())
	assert.Nil(t, (&Manager{}).LeftOutHostUUIDs())
}

func TestLeftOutHostUUIDs_ReturnsLastRecordedSet(t *testing.T) {
	t.Parallel()
	m := &Manager{}
	set := map[string]struct{}{"host-1": {}}
	m.leftOut.Store(&set)
	assert.Equal(t, map[string]struct{}{"host-1": {}}, m.LeftOutHostUUIDs())
}

func TestManagerApplyConfig_RecordsHostsLeftOut(t *testing.T) {
	caddyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer caddyServer.Close()

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.ProxyHost{}, &models.Location{}, &models.Setting{}, &models.CaddyConfig{}, &models.SSLCertificate{}))
	for _, h := range []models.ProxyHost{
		{UUID: "left-out", DomainNames: "left.example.com", ForwardHost: "169.254.1.5", ForwardPort: 80, Enabled: true},
		{UUID: "served", DomainNames: "served.example.com", ForwardHost: "10.0.0.5", ForwardPort: 8080, Enabled: true},
	} {
		require.NoError(t, db.Create(&h).Error)
	}

	manager := NewManager(newTestClient(t, caddyServer.URL), db, t.TempDir(), "", false, config.SecurityConfig{})
	manager.SetForwardTargetGuard(prefixGuard{prefix: "169.254."})

	require.NoError(t, manager.ApplyConfig(context.Background()))
	assert.Equal(t, map[string]struct{}{"left-out": {}}, manager.LeftOutHostUUIDs())
}
