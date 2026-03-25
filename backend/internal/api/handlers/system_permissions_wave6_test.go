package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestSystemPermissionsWave6_RepairPermissions_NonRootBranchViaSeteuid(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("test requires root execution")
	}

	if err := syscall.Seteuid(65534); err != nil {
		t.Skip("unable to drop euid for test")
	}
	defer func() {
		restoreErr := syscall.Seteuid(0)
		require.NoError(t, restoreErr)
	}()


	root := t.TempDir()
	dataDir := filepath.Join(root, "data")
	require.NoError(t, os.MkdirAll(dataDir, 0o750))

	h := NewSystemPermissionsHandler(config.Config{
		SingleContainer: true,
		DatabasePath:    filepath.Join(dataDir, "charon.db"),
		ConfigRoot:      dataDir,
		CaddyLogDir:     dataDir,
		CrowdSecLogDir:  dataDir,
	}, nil, stubPermissionChecker{})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("role", "admin")
	c.Request = httptest.NewRequest(http.MethodPost, "/system/permissions/repair", bytes.NewBufferString(`{"paths":["/tmp"]}`))
	c.Request.Header.Set("Content-Type", "application/json")

	h.RepairPermissions(c)

	require.Equal(t, http.StatusForbidden, w.Code)
	var payload map[string]string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &payload))
	require.Equal(t, "permissions_non_root", payload["error_code"])
}
