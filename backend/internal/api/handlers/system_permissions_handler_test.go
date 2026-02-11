package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/util"
)

type stubPermissionChecker struct{}

func (stubPermissionChecker) Check(path, required string) util.PermissionCheck {
	return util.PermissionCheck{
		Path:     path,
		Required: required,
		Exists:   true,
		Writable: true,
		OwnerUID: 1000,
		OwnerGID: 1000,
		Mode:     "0755",
	}
}

func TestSystemPermissionsHandler_GetPermissions_Admin(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := config.Config{
		DatabasePath:   "/app/data/charon.db",
		ConfigRoot:     "/config",
		CaddyLogDir:    "/var/log/caddy",
		CrowdSecLogDir: "/var/log/crowdsec",
		PluginsDir:     "/app/plugins",
	}

	h := NewSystemPermissionsHandler(cfg, nil, stubPermissionChecker{})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("role", "admin")
	c.Request = httptest.NewRequest(http.MethodGet, "/system/permissions", http.NoBody)

	h.GetPermissions(c)

	require.Equal(t, http.StatusOK, w.Code)

	var payload struct {
		Paths []map[string]any `json:"paths"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &payload))
	require.NotEmpty(t, payload.Paths)

	first := payload.Paths[0]
	require.NotEmpty(t, first["path"])
	require.NotEmpty(t, first["required"])
	require.NotEmpty(t, first["mode"])
}

func TestSystemPermissionsHandler_GetPermissions_NonAdmin(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := config.Config{}
	h := NewSystemPermissionsHandler(cfg, nil, stubPermissionChecker{})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("role", "user")
	c.Request = httptest.NewRequest(http.MethodGet, "/system/permissions", http.NoBody)

	h.GetPermissions(c)

	require.Equal(t, http.StatusForbidden, w.Code)

	var payload map[string]string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &payload))
	require.Equal(t, "permissions_admin_only", payload["error_code"])
}

func TestSystemPermissionsHandler_RepairPermissions_NonRoot(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("test requires non-root execution")
	}

	gin.SetMode(gin.TestMode)

	cfg := config.Config{SingleContainer: true}
	h := NewSystemPermissionsHandler(cfg, nil, stubPermissionChecker{})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("role", "admin")
	c.Request = httptest.NewRequest(http.MethodPost, "/system/permissions/repair", http.NoBody)

	h.RepairPermissions(c)

	require.Equal(t, http.StatusForbidden, w.Code)

	var payload map[string]string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &payload))
	require.Equal(t, "permissions_non_root", payload["error_code"])
}
