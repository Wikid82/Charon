package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/gin-gonic/gin"

	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/services"
	"github.com/Wikid82/charon/backend/internal/util"
)

type PermissionChecker interface {
	Check(path, required string) util.PermissionCheck
}

type OSChecker struct{}

func (OSChecker) Check(path, required string) util.PermissionCheck {
	return util.CheckPathPermissions(path, required)
}

type SystemPermissionsHandler struct {
	cfg             config.Config
	checker         PermissionChecker
	securityService *services.SecurityService
}

type permissionsPathSpec struct {
	Path     string
	Required string
}

type permissionsRepairRequest struct {
	Paths     []string `json:"paths" binding:"required,min=1"`
	GroupMode bool     `json:"group_mode"`
}

type permissionsRepairResult struct {
	Path       string `json:"path"`
	Status     string `json:"status"`
	OwnerUID   int    `json:"owner_uid,omitempty"`
	OwnerGID   int    `json:"owner_gid,omitempty"`
	ModeBefore string `json:"mode_before,omitempty"`
	ModeAfter  string `json:"mode_after,omitempty"`
	Message    string `json:"message,omitempty"`
	ErrorCode  string `json:"error_code,omitempty"`
}

func NewSystemPermissionsHandler(cfg config.Config, securityService *services.SecurityService, checker PermissionChecker) *SystemPermissionsHandler {
	if checker == nil {
		checker = OSChecker{}
	}
	return &SystemPermissionsHandler{
		cfg:             cfg,
		checker:         checker,
		securityService: securityService,
	}
}

func (h *SystemPermissionsHandler) GetPermissions(c *gin.Context) {
	if !requireAdmin(c) {
		h.logAudit(c, "permissions_diagnostics", "blocked", "permissions_admin_only", 0)
		return
	}

	paths := h.defaultPaths()
	results := make([]util.PermissionCheck, 0, len(paths))
	for _, spec := range paths {
		results = append(results, h.checker.Check(spec.Path, spec.Required))
	}

	h.logAudit(c, "permissions_diagnostics", "ok", "", len(results))
	c.JSON(http.StatusOK, gin.H{"paths": results})
}

func (h *SystemPermissionsHandler) RepairPermissions(c *gin.Context) {
	if !requireAdmin(c) {
		h.logAudit(c, "permissions_repair", "blocked", "permissions_admin_only", 0)
		return
	}

	if !h.cfg.SingleContainer {
		h.logAudit(c, "permissions_repair", "blocked", "permissions_repair_disabled", 0)
		c.JSON(http.StatusForbidden, gin.H{
			"error":      "repair disabled",
			"error_code": "permissions_repair_disabled",
		})
		return
	}

	if os.Geteuid() != 0 {
		h.logAudit(c, "permissions_repair", "blocked", "permissions_non_root", 0)
		c.JSON(http.StatusForbidden, gin.H{
			"error":      "root privileges required",
			"error_code": "permissions_non_root",
		})
		return
	}

	var req permissionsRepairRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	results := make([]permissionsRepairResult, 0, len(req.Paths))
	allowlist := h.allowlistRoots()

	for _, rawPath := range req.Paths {
		result := h.repairPath(rawPath, req.GroupMode, allowlist)
		results = append(results, result)
	}

	h.logAudit(c, "permissions_repair", "ok", "", len(results))
	c.JSON(http.StatusOK, gin.H{"paths": results})
}

func (h *SystemPermissionsHandler) repairPath(rawPath string, groupMode bool, allowlist []string) permissionsRepairResult {
	cleanPath, invalidCode := normalizePath(rawPath)
	if invalidCode != "" {
		return permissionsRepairResult{
			Path:      rawPath,
			Status:    "error",
			ErrorCode: invalidCode,
			Message:   "invalid path",
		}
	}

	info, err := os.Lstat(cleanPath)
	if err != nil {
		if os.IsNotExist(err) {
			return permissionsRepairResult{
				Path:      cleanPath,
				Status:    "error",
				ErrorCode: "permissions_missing_path",
				Message:   "path does not exist",
			}
		}
		return permissionsRepairResult{
			Path:      cleanPath,
			Status:    "error",
			ErrorCode: "permissions_repair_failed",
			Message:   err.Error(),
		}
	}

	if info.Mode()&os.ModeSymlink != 0 {
		return permissionsRepairResult{
			Path:      cleanPath,
			Status:    "error",
			ErrorCode: "permissions_symlink_rejected",
			Message:   "symlink not allowed",
		}
	}

	hasSymlinkComponent, symlinkErr := pathHasSymlink(cleanPath)
	if symlinkErr != nil {
		if os.IsNotExist(symlinkErr) {
			return permissionsRepairResult{
				Path:      cleanPath,
				Status:    "error",
				ErrorCode: "permissions_missing_path",
				Message:   "path does not exist",
			}
		}
		return permissionsRepairResult{
			Path:      cleanPath,
			Status:    "error",
			ErrorCode: "permissions_repair_failed",
			Message:   symlinkErr.Error(),
		}
	}
	if hasSymlinkComponent {
		return permissionsRepairResult{
			Path:      cleanPath,
			Status:    "error",
			ErrorCode: "permissions_symlink_rejected",
			Message:   "symlink not allowed",
		}
	}

	resolved, err := filepath.EvalSymlinks(cleanPath)
	if err != nil {
		return permissionsRepairResult{
			Path:      cleanPath,
			Status:    "error",
			ErrorCode: "permissions_repair_failed",
			Message:   err.Error(),
		}
	}

	if !isWithinAllowlist(resolved, allowlist) {
		return permissionsRepairResult{
			Path:      cleanPath,
			Status:    "error",
			ErrorCode: "permissions_outside_allowlist",
			Message:   "path outside allowlist",
		}
	}

	if !info.IsDir() && !info.Mode().IsRegular() {
		return permissionsRepairResult{
			Path:      cleanPath,
			Status:    "error",
			ErrorCode: "permissions_unsupported_type",
			Message:   "unsupported path type",
		}
	}

	uid := os.Geteuid()
	gid := os.Getegid()
	modeBefore := fmt.Sprintf("%04o", info.Mode().Perm())
	modeAfter := targetMode(info.IsDir(), groupMode)

	alreadyOwned := isOwnedBy(info, uid, gid)
	alreadyMode := modeBefore == modeAfter
	if alreadyOwned && alreadyMode {
		return permissionsRepairResult{
			Path:       cleanPath,
			Status:     "skipped",
			OwnerUID:   uid,
			OwnerGID:   gid,
			ModeBefore: modeBefore,
			ModeAfter:  modeAfter,
			Message:    "ownership and mode already correct",
			ErrorCode:  "permissions_repair_skipped",
		}
	}

	if err := os.Chown(cleanPath, uid, gid); err != nil {
		return permissionsRepairResult{
			Path:      cleanPath,
			Status:    "error",
			ErrorCode: mapRepairErrorCode(err),
			Message:   err.Error(),
		}
	}

	parsedMode, parseErr := parseMode(modeAfter)
	if parseErr != nil {
		return permissionsRepairResult{
			Path:      cleanPath,
			Status:    "error",
			ErrorCode: "permissions_repair_failed",
			Message:   parseErr.Error(),
		}
	}
	if err := os.Chmod(cleanPath, parsedMode); err != nil {
		return permissionsRepairResult{
			Path:      cleanPath,
			Status:    "error",
			ErrorCode: mapRepairErrorCode(err),
			Message:   err.Error(),
		}
	}

	return permissionsRepairResult{
		Path:       cleanPath,
		Status:     "repaired",
		OwnerUID:   uid,
		OwnerGID:   gid,
		ModeBefore: modeBefore,
		ModeAfter:  modeAfter,
		Message:    "ownership and mode updated",
	}
}

func (h *SystemPermissionsHandler) defaultPaths() []permissionsPathSpec {
	dataRoot := filepath.Dir(h.cfg.DatabasePath)
	return []permissionsPathSpec{
		{Path: dataRoot, Required: "rwx"},
		{Path: h.cfg.DatabasePath, Required: "rw"},
		{Path: filepath.Join(dataRoot, "backups"), Required: "rwx"},
		{Path: filepath.Join(dataRoot, "imports"), Required: "rwx"},
		{Path: filepath.Join(dataRoot, "caddy"), Required: "rwx"},
		{Path: filepath.Join(dataRoot, "crowdsec"), Required: "rwx"},
		{Path: filepath.Join(dataRoot, "geoip"), Required: "rwx"},
		{Path: h.cfg.ConfigRoot, Required: "rwx"},
		{Path: h.cfg.CaddyLogDir, Required: "rwx"},
		{Path: h.cfg.CrowdSecLogDir, Required: "rwx"},
		{Path: h.cfg.PluginsDir, Required: "r-x"},
	}
}

func (h *SystemPermissionsHandler) allowlistRoots() []string {
	dataRoot := filepath.Dir(h.cfg.DatabasePath)
	return []string{
		dataRoot,
		h.cfg.ConfigRoot,
		h.cfg.CaddyLogDir,
		h.cfg.CrowdSecLogDir,
	}
}

func (h *SystemPermissionsHandler) logAudit(c *gin.Context, action, result, code string, pathCount int) {
	if h.securityService == nil {
		return
	}
	payload := map[string]any{
		"result":     result,
		"error_code": code,
		"path_count": pathCount,
		"admin":      isAdmin(c),
	}
	payloadJSON, _ := json.Marshal(payload)

	actor := "unknown"
	if userID, ok := c.Get("userID"); ok {
		actor = fmt.Sprintf("%v", userID)
	}

	_ = h.securityService.LogAudit(&models.SecurityAudit{
		Actor:         actor,
		Action:        action,
		EventCategory: "permissions",
		Details:       string(payloadJSON),
		IPAddress:     c.ClientIP(),
		UserAgent:     c.Request.UserAgent(),
	})
}

func normalizePath(rawPath string) (string, string) {
	if rawPath == "" {
		return "", "permissions_invalid_path"
	}
	if !filepath.IsAbs(rawPath) {
		return "", "permissions_invalid_path"
	}
	clean := filepath.Clean(rawPath)
	if clean == "." || clean == ".." {
		return "", "permissions_invalid_path"
	}
	if containsParentReference(clean) {
		return "", "permissions_invalid_path"
	}
	return clean, ""
}

func containsParentReference(clean string) bool {
	if clean == ".." {
		return true
	}
	if strings.HasPrefix(clean, ".."+string(os.PathSeparator)) {
		return true
	}
	if strings.Contains(clean, string(os.PathSeparator)+".."+string(os.PathSeparator)) {
		return true
	}
	return strings.HasSuffix(clean, string(os.PathSeparator)+"..")
}

func pathHasSymlink(path string) (bool, error) {
	clean := filepath.Clean(path)
	parts := strings.Split(clean, string(os.PathSeparator))
	current := string(os.PathSeparator)
	for _, part := range parts {
		if part == "" {
			continue
		}
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if err != nil {
			return false, err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return true, nil
		}
	}
	return false, nil
}

func isWithinAllowlist(path string, allowlist []string) bool {
	for _, root := range allowlist {
		rel, err := filepath.Rel(root, path)
		if err != nil {
			continue
		}
		if rel == "." || (!strings.HasPrefix(rel, ".."+string(os.PathSeparator)) && rel != "..") {
			return true
		}
	}
	return false
}

func targetMode(isDir, groupMode bool) string {
	if isDir {
		if groupMode {
			return "0770"
		}
		return "0700"
	}
	if groupMode {
		return "0660"
	}
	return "0600"
}

func parseMode(mode string) (os.FileMode, error) {
	if mode == "" {
		return 0, fmt.Errorf("mode required")
	}
	var parsed uint32
	if _, err := fmt.Sscanf(mode, "%o", &parsed); err != nil {
		return 0, fmt.Errorf("parse mode: %w", err)
	}
	return os.FileMode(parsed), nil
}

func isOwnedBy(info os.FileInfo, uid, gid int) bool {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return false
	}
	return int(stat.Uid) == uid && int(stat.Gid) == gid
}

func mapRepairErrorCode(err error) string {
	switch {
	case err == nil:
		return ""
	case errors.Is(err, syscall.EROFS):
		return "permissions_readonly"
	case errors.Is(err, syscall.EACCES) || os.IsPermission(err):
		return "permissions_write_denied"
	default:
		return "permissions_repair_failed"
	}
}
