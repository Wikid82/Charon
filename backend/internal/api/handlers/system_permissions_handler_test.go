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
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/services"
	"github.com/Wikid82/charon/backend/internal/util"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type stubPermissionChecker struct{}

type fakeNoStatFileInfo struct{}

func (fakeNoStatFileInfo) Name() string       { return "fake" }
func (fakeNoStatFileInfo) Size() int64        { return 0 }
func (fakeNoStatFileInfo) Mode() os.FileMode  { return 0 }
func (fakeNoStatFileInfo) ModTime() time.Time { return time.Time{} }
func (fakeNoStatFileInfo) IsDir() bool        { return false }
func (fakeNoStatFileInfo) Sys() any           { return nil }

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

func TestSystemPermissionsHandler_NewDefaultsCheckerToOSChecker(t *testing.T) {
	h := NewSystemPermissionsHandler(config.Config{}, nil, nil)
	require.NotNil(t, h)
	require.NotNil(t, h.checker)
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

func TestSystemPermissionsHandler_RepairPermissions_NonAdmin(t *testing.T) {
	gin.SetMode(gin.TestMode)

	h := NewSystemPermissionsHandler(config.Config{SingleContainer: true}, nil, stubPermissionChecker{})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("role", "user")
	c.Request = httptest.NewRequest(http.MethodPost, "/system/permissions/repair", bytes.NewBufferString(`{"paths":["/tmp"]}`))
	c.Request.Header.Set("Content-Type", "application/json")

	h.RepairPermissions(c)

	require.Equal(t, http.StatusForbidden, w.Code)
}

func TestSystemPermissionsHandler_RepairPermissions_InvalidJSONWhenRoot(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("test requires root execution")
	}

	gin.SetMode(gin.TestMode)
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
	c.Request = httptest.NewRequest(http.MethodPost, "/system/permissions/repair", bytes.NewBufferString(`{"paths":`))
	c.Request.Header.Set("Content-Type", "application/json")

	h.RepairPermissions(c)

	require.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSystemPermissionsHandler_DefaultPathsAndAllowlistRoots(t *testing.T) {
	h := NewSystemPermissionsHandler(config.Config{
		DatabasePath:   "/app/data/charon.db",
		ConfigRoot:     "/app/config",
		CaddyLogDir:    "/var/log/caddy",
		CrowdSecLogDir: "/var/log/crowdsec",
		PluginsDir:     "/app/plugins",
	}, nil, stubPermissionChecker{})

	paths := h.defaultPaths()
	require.Len(t, paths, 11)
	require.Equal(t, "/app/data", paths[0].Path)
	require.Equal(t, "/app/plugins", paths[len(paths)-1].Path)

	roots := h.allowlistRoots()
	require.Equal(t, []string{"/app/data", "/app/config", "/var/log/caddy", "/var/log/crowdsec"}, roots)
}

func TestSystemPermissionsHandler_IsOwnedByFalseWhenSysNotStat(t *testing.T) {
	owned := isOwnedBy(fakeNoStatFileInfo{}, os.Geteuid(), os.Getegid())
	require.False(t, owned)
}

func TestSystemPermissionsHandler_IsWithinAllowlist_RelErrorBranch(t *testing.T) {
	tmp := t.TempDir()
	inAllow := filepath.Join(tmp, "a", "b")
	require.NoError(t, os.MkdirAll(inAllow, 0o750))

	badRoot := string([]byte{'/', 0, 'x'})
	allowed := isWithinAllowlist(inAllow, []string{badRoot, tmp})
	require.True(t, allowed)
}

func TestSystemPermissionsHandler_IsWithinAllowlist_AllRelErrorsReturnFalse(t *testing.T) {
	badRoot1 := string([]byte{'/', 0, 'x'})
	badRoot2 := string([]byte{'/', 0, 'y'})
	allowed := isWithinAllowlist("/tmp/some/path", []string{badRoot1, badRoot2})
	require.False(t, allowed)
}

func TestSystemPermissionsHandler_LogAudit_PersistsAuditWithUserID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.SecurityAudit{}))

	securitySvc := services.NewSecurityService(db)
	h := NewSystemPermissionsHandler(config.Config{}, securitySvc, stubPermissionChecker{})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("role", "admin")
	c.Set("userID", 42)
	c.Request = httptest.NewRequest(http.MethodGet, "/system/permissions", http.NoBody)

	require.NotPanics(t, func() {
		h.logAudit(c, "permissions_diagnostics", "ok", "", 2)
	})
}

func TestSystemPermissionsHandler_LogAudit_PersistsAuditWithUnknownActor(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.SecurityAudit{}))

	securitySvc := services.NewSecurityService(db)
	h := NewSystemPermissionsHandler(config.Config{}, securitySvc, stubPermissionChecker{})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("role", "admin")
	c.Request = httptest.NewRequest(http.MethodGet, "/system/permissions", http.NoBody)

	require.NotPanics(t, func() {
		h.logAudit(c, "permissions_diagnostics", "ok", "", 1)
	})
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

	t.Run("outside allowlist rejected before stat for missing path", func(t *testing.T) {
		outsideMissing := filepath.Join(t.TempDir(), "missing.txt")

		result := h.repairPath(outsideMissing, false, allowlist)
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

func TestSystemPermissionsHandler_OSChecker_Check(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("test expects root-owned temp paths in CI")
	}

	tmp := t.TempDir()
	filePath := filepath.Join(tmp, "check.txt")
	require.NoError(t, os.WriteFile(filePath, []byte("ok"), 0o600))

	checker := OSChecker{}
	result := checker.Check(filePath, "rw")
	require.Equal(t, filePath, result.Path)
	require.Equal(t, "rw", result.Required)
	require.True(t, result.Exists)
}

func TestSystemPermissionsHandler_RepairPermissions_InvalidRequestBody_Root(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("test requires root execution")
	}

	gin.SetMode(gin.TestMode)

	tmp := t.TempDir()
	dataDir := filepath.Join(tmp, "data")
	require.NoError(t, os.MkdirAll(dataDir, 0o750))

	h := NewSystemPermissionsHandler(config.Config{
		SingleContainer: true,
		DatabasePath:    filepath.Join(dataDir, "charon.db"),
		ConfigRoot:      dataDir,
		CaddyLogDir:     dataDir,
		CrowdSecLogDir:  dataDir,
		PluginsDir:      filepath.Join(tmp, "plugins"),
	}, nil, stubPermissionChecker{})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("role", "admin")
	c.Request = httptest.NewRequest(http.MethodPost, "/system/permissions/repair", bytes.NewBufferString(`{"group_mode":true}`))
	c.Request.Header.Set("Content-Type", "application/json")

	h.RepairPermissions(c)
	require.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSystemPermissionsHandler_RepairPath_LstatInvalidArgument(t *testing.T) {
	h := NewSystemPermissionsHandler(config.Config{}, nil, stubPermissionChecker{})
	allowRoot := t.TempDir()

	result := h.repairPath("/tmp/\x00invalid", false, []string{allowRoot})
	require.Equal(t, "error", result.Status)
	require.Equal(t, "permissions_outside_allowlist", result.ErrorCode)
}

func TestSystemPermissionsHandler_RepairPath_RepairedBranch(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("test requires root execution")
	}

	h := NewSystemPermissionsHandler(config.Config{}, nil, stubPermissionChecker{})
	allowRoot := t.TempDir()
	targetFile := filepath.Join(allowRoot, "needs-repair.txt")
	require.NoError(t, os.WriteFile(targetFile, []byte("ok"), 0o600))

	result := h.repairPath(targetFile, true, []string{allowRoot})
	require.Equal(t, "repaired", result.Status)
	require.Equal(t, "0660", result.ModeAfter)

	info, err := os.Stat(targetFile)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o660), info.Mode().Perm())
}

func TestSystemPermissionsHandler_NormalizePath_ParentRefBranches(t *testing.T) {
	clean, code := normalizePath("/../etc")
	require.Equal(t, "/etc", clean)
	require.Empty(t, code)

	clean, code = normalizePath("/var/../etc")
	require.Equal(t, "/etc", clean)
	require.Empty(t, code)
}

func TestSystemPermissionsHandler_NormalizeAllowlist(t *testing.T) {
	allowlist := normalizeAllowlist([]string{"", "/tmp/data/..", "/var/log/charon"})
	require.Equal(t, []string{"/tmp", "/var/log/charon"}, allowlist)
}
