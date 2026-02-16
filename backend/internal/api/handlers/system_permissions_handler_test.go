package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"syscall"
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

func TestSystemPermissionsHandler_HelperFunctions(t *testing.T) {
	t.Run("normalizePath", func(t *testing.T) {
		clean, code := normalizePath("/tmp/example")
		require.Equal(t, "/tmp/example", clean)
		require.Empty(t, code)

		clean, code = normalizePath("")
		require.Empty(t, clean)
		require.Equal(t, "permissions_invalid_path", code)

		clean, code = normalizePath("relative/path")
		require.Empty(t, clean)
		require.Equal(t, "permissions_invalid_path", code)
	})

	t.Run("containsParentReference", func(t *testing.T) {
		require.True(t, containsParentReference(".."))
		require.True(t, containsParentReference("../secrets"))
		require.True(t, containsParentReference("/var/../etc"))
		require.True(t, containsParentReference("/var/log/.."))
		require.False(t, containsParentReference("/var/log/charon"))
	})

	t.Run("isWithinAllowlist", func(t *testing.T) {
		allowlist := []string{"/app/data", "/config"}
		require.True(t, isWithinAllowlist("/app/data/charon.db", allowlist))
		require.True(t, isWithinAllowlist("/config/caddy", allowlist))
		require.False(t, isWithinAllowlist("/etc/passwd", allowlist))
	})

	t.Run("targetMode", func(t *testing.T) {
		require.Equal(t, "0700", targetMode(true, false))
		require.Equal(t, "0770", targetMode(true, true))
		require.Equal(t, "0600", targetMode(false, false))
		require.Equal(t, "0660", targetMode(false, true))
	})

	t.Run("parseMode", func(t *testing.T) {
		mode, err := parseMode("0640")
		require.NoError(t, err)
		require.Equal(t, os.FileMode(0640), mode)

		_, err = parseMode("")
		require.Error(t, err)

		_, err = parseMode("invalid")
		require.Error(t, err)
	})

	t.Run("mapRepairErrorCode", func(t *testing.T) {
		require.Equal(t, "", mapRepairErrorCode(nil))
		require.Equal(t, "permissions_readonly", mapRepairErrorCode(syscall.EROFS))
		require.Equal(t, "permissions_write_denied", mapRepairErrorCode(syscall.EACCES))
		require.Equal(t, "permissions_repair_failed", mapRepairErrorCode(syscall.EINVAL))
	})
}

func TestSystemPermissionsHandler_PathHasSymlink(t *testing.T) {
	root := t.TempDir()

	realDir := filepath.Join(root, "real")
	require.NoError(t, os.Mkdir(realDir, 0o750))

	plainPath := filepath.Join(realDir, "file.txt")
	require.NoError(t, os.WriteFile(plainPath, []byte("ok"), 0o600))

	hasSymlink, err := pathHasSymlink(plainPath)
	require.NoError(t, err)
	require.False(t, hasSymlink)

	linkDir := filepath.Join(root, "link")
	require.NoError(t, os.Symlink(realDir, linkDir))

	symlinkedPath := filepath.Join(linkDir, "file.txt")
	hasSymlink, err = pathHasSymlink(symlinkedPath)
	require.NoError(t, err)
	require.True(t, hasSymlink)

	_, err = pathHasSymlink(filepath.Join(root, "missing", "file.txt"))
	require.Error(t, err)
}

func TestSystemPermissionsHandler_RepairPermissions_DisabledWhenNotSingleContainer(t *testing.T) {
	gin.SetMode(gin.TestMode)

	h := NewSystemPermissionsHandler(config.Config{SingleContainer: false}, nil, stubPermissionChecker{})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("role", "admin")
	c.Request = httptest.NewRequest(http.MethodPost, "/system/permissions/repair", bytes.NewBufferString(`{"paths":["/tmp"]}`))
	c.Request.Header.Set("Content-Type", "application/json")

	h.RepairPermissions(c)

	require.Equal(t, http.StatusForbidden, w.Code)
	var payload map[string]string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &payload))
	require.Equal(t, "permissions_repair_disabled", payload["error_code"])
}

func TestSystemPermissionsHandler_RepairPermissions_InvalidJSON(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("test requires root execution")
	}

	gin.SetMode(gin.TestMode)

	root := t.TempDir()
	dataDir := filepath.Join(root, "data")
	require.NoError(t, os.MkdirAll(dataDir, 0o750))

	cfg := config.Config{
		SingleContainer: true,
		DatabasePath:    filepath.Join(dataDir, "charon.db"),
		ConfigRoot:      dataDir,
		CaddyLogDir:     dataDir,
		CrowdSecLogDir:  dataDir,
		PluginsDir:      filepath.Join(root, "plugins"),
	}

	h := NewSystemPermissionsHandler(cfg, nil, stubPermissionChecker{})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("role", "admin")
	c.Request = httptest.NewRequest(http.MethodPost, "/system/permissions/repair", bytes.NewBufferString(`{"paths":`))
	c.Request.Header.Set("Content-Type", "application/json")

	h.RepairPermissions(c)

	require.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSystemPermissionsHandler_RepairPermissions_Success(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("test requires root execution")
	}

	gin.SetMode(gin.TestMode)

	root := t.TempDir()
	dataDir := filepath.Join(root, "data")
	require.NoError(t, os.MkdirAll(dataDir, 0o750))

	targetFile := filepath.Join(dataDir, "repair-target.txt")
	require.NoError(t, os.WriteFile(targetFile, []byte("repair"), 0o600))

	cfg := config.Config{
		SingleContainer: true,
		DatabasePath:    filepath.Join(dataDir, "charon.db"),
		ConfigRoot:      dataDir,
		CaddyLogDir:     dataDir,
		CrowdSecLogDir:  dataDir,
		PluginsDir:      filepath.Join(root, "plugins"),
	}

	h := NewSystemPermissionsHandler(cfg, nil, stubPermissionChecker{})

	body := fmt.Sprintf(`{"paths":[%q],"group_mode":false}`, targetFile)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("role", "admin")
	c.Request = httptest.NewRequest(http.MethodPost, "/system/permissions/repair", bytes.NewBufferString(body))
	c.Request.Header.Set("Content-Type", "application/json")

	h.RepairPermissions(c)

	require.Equal(t, http.StatusOK, w.Code)

	var payload struct {
		Paths []permissionsRepairResult `json:"paths"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &payload))
	require.Len(t, payload.Paths, 1)
	require.Equal(t, targetFile, payload.Paths[0].Path)
	require.NotEqual(t, "error", payload.Paths[0].Status)
}

func TestSystemPermissionsHandler_RepairPath_Branches(t *testing.T) {
	h := NewSystemPermissionsHandler(config.Config{}, nil, stubPermissionChecker{})
	allowRoot := t.TempDir()
	allowlist := []string{allowRoot}

	t.Run("invalid path", func(t *testing.T) {
		result := h.repairPath("", false, allowlist)
		require.Equal(t, "error", result.Status)
		require.Equal(t, "permissions_invalid_path", result.ErrorCode)
	})

	t.Run("missing path", func(t *testing.T) {
		missingPath := filepath.Join(allowRoot, "missing-file.txt")
		result := h.repairPath(missingPath, false, allowlist)
		require.Equal(t, "error", result.Status)
		require.Equal(t, "permissions_missing_path", result.ErrorCode)
	})

	t.Run("symlink leaf rejected", func(t *testing.T) {
		target := filepath.Join(allowRoot, "target.txt")
		require.NoError(t, os.WriteFile(target, []byte("ok"), 0o600))
		link := filepath.Join(allowRoot, "link.txt")
		require.NoError(t, os.Symlink(target, link))

		result := h.repairPath(link, false, allowlist)
		require.Equal(t, "error", result.Status)
		require.Equal(t, "permissions_symlink_rejected", result.ErrorCode)
	})

	t.Run("symlink component rejected", func(t *testing.T) {
		realDir := filepath.Join(allowRoot, "real")
		require.NoError(t, os.MkdirAll(realDir, 0o750))
		realFile := filepath.Join(realDir, "file.txt")
		require.NoError(t, os.WriteFile(realFile, []byte("ok"), 0o600))

		linkDir := filepath.Join(allowRoot, "linkdir")
		require.NoError(t, os.Symlink(realDir, linkDir))

		result := h.repairPath(filepath.Join(linkDir, "file.txt"), false, allowlist)
		require.Equal(t, "error", result.Status)
		require.Equal(t, "permissions_symlink_rejected", result.ErrorCode)
	})

	t.Run("outside allowlist rejected", func(t *testing.T) {
		outsideFile := filepath.Join(t.TempDir(), "outside.txt")
		require.NoError(t, os.WriteFile(outsideFile, []byte("x"), 0o600))

		result := h.repairPath(outsideFile, false, allowlist)
		require.Equal(t, "error", result.Status)
		require.Equal(t, "permissions_outside_allowlist", result.ErrorCode)
	})

	t.Run("unsupported type rejected", func(t *testing.T) {
		fifoPath := filepath.Join(allowRoot, "fifo")
		require.NoError(t, syscall.Mkfifo(fifoPath, 0o600))

		result := h.repairPath(fifoPath, false, allowlist)
		require.Equal(t, "error", result.Status)
		require.Equal(t, "permissions_unsupported_type", result.ErrorCode)
	})

	t.Run("already correct skipped", func(t *testing.T) {
		filePath := filepath.Join(allowRoot, "already-correct.txt")
		require.NoError(t, os.WriteFile(filePath, []byte("ok"), 0o600))

		result := h.repairPath(filePath, false, allowlist)
		require.Equal(t, "skipped", result.Status)
		require.Equal(t, "permissions_repair_skipped", result.ErrorCode)
		require.Equal(t, "0600", result.ModeAfter)
	})
}
