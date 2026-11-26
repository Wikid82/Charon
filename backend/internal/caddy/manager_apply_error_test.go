package caddy

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	logtest "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/testutil"
)

// managerRefusalFixture is an obviously fake credential carried by the advanced configuration of the host below.
const managerRefusalFixture = "FAKE-mgr-0022"

// newManagerRefusingAdmin refuses every load whose body carries the fixture, quoting the value as Caddy does.
func newManagerRefusingAdmin(t *testing.T) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if bytes.Contains(body, []byte(managerRefusalFixture)) {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = fmt.Fprintf(w, `{"error":"loading new config: refused header value 'Bearer %s'"}`, managerRefusalFixture)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)
	return server
}

// refusedApplyManager returns a manager whose one enabled host carries the fixture in its advanced configuration.
func refusedApplyManager(t *testing.T, admin *httptest.Server) (*Manager, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(testutil.MemoryDSN(t)), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.ProxyHost{}, &models.Location{}, &models.Setting{}, &models.CaddyConfig{}, &models.SSLCertificate{}))
	host := models.ProxyHost{
		UUID:           "refused-host",
		DomainNames:    "refused.example.com",
		ForwardHost:    "10.0.0.5",
		ForwardPort:    8080,
		Enabled:        true,
		AdvancedConfig: `{"handler":"headers","request":{"set":{"Authorization":["Bearer ` + managerRefusalFixture + `"]}}}`,
	}
	require.NoError(t, db.Create(&host).Error)
	manager := NewManager(newTestClient(t, admin.URL), db, t.TempDir(), "", false, config.SecurityConfig{})
	return manager, db
}

// TestApplyConfigRefusalKeepsTheCredentialOutOfErrorHistoryAndLog covers the three sinks of an apply error: the
// returned error, the stored history and the log entries.
func TestApplyConfigRefusalKeepsTheCredentialOutOfErrorHistoryAndLog(t *testing.T) {
	admin := newManagerRefusingAdmin(t)
	manager, db := refusedApplyManager(t, admin)
	hook := logtest.NewLocal(logger.Log().Logger)
	t.Cleanup(hook.Reset)

	err := manager.ApplyConfig(context.Background())

	require.Error(t, err)
	require.NotContains(t, err.Error(), managerRefusalFixture)
	require.Contains(t, err.Error(), "refused header value")

	var rows []models.CaddyConfig
	require.NoError(t, db.Find(&rows).Error)
	require.NotEmpty(t, rows)
	for _, row := range rows {
		require.NotContains(t, row.ErrorMsg, managerRefusalFixture)
	}

	for _, entry := range hook.AllEntries() {
		require.NotContains(t, entry.Message, managerRefusalFixture)
		for _, v := range entry.Data {
			require.NotContains(t, fmt.Sprint(v), managerRefusalFixture)
		}
	}
}

// TestApplyConfigRefusalWithoutARollbackStillRedacts covers a refused apply that cannot roll back: the first
// apply has no earlier snapshot, so the joined error reports both parts.
func TestApplyConfigRefusalWithoutARollbackStillRedacts(t *testing.T) {
	admin := newManagerRefusingAdmin(t)
	manager, db := refusedApplyManager(t, admin)

	err := manager.ApplyConfig(context.Background())

	require.Error(t, err)
	require.NotContains(t, err.Error(), managerRefusalFixture)
	var rows []models.CaddyConfig
	require.NoError(t, db.Find(&rows).Error)
	require.NotEmpty(t, rows)
	require.Contains(t, rows[len(rows)-1].ErrorMsg, "[redacted]")
}
