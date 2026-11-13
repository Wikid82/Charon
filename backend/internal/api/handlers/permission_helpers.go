package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"

	"github.com/Wikid82/charon/backend/internal/api/middleware"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/services"
	"github.com/Wikid82/charon/backend/internal/util"
)

func requireAdmin(c *gin.Context) bool {
	if isAdmin(c) {
		return true
	}
	c.JSON(http.StatusForbidden, gin.H{
		"error":      "admin privileges required",
		"error_code": "permissions_admin_only",
	})
	return false
}

func requireAuthenticatedAdmin(c *gin.Context) bool {
	if _, ok := middleware.CallerID(c); !ok {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "Authorization header required",
		})
		return false
	}

	return requireAdmin(c)
}

func isAdmin(c *gin.Context) bool {
	return middleware.CallerRole(c) == string(models.RoleAdmin)
}

// redactForRole returns item unchanged for administrators. For any other caller
// it returns the item as a JSON object without the hidden top-level keys, plus
// the flags derived from the original item (for example has_url). Clients keep
// the snake_case names they already read; only the sensitive values are removed.
func redactForRole[T any](c *gin.Context, item T, hidden []string, flags func(T) map[string]any) (any, error) {
	if isAdmin(c) {
		return item, nil
	}
	return redactItem(item, hidden, flags)
}

// redactListForRole applies redactForRole to every element of items. Administrators
// receive the original slice. Other callers receive a non-nil slice, so an empty
// result still encodes as [].
func redactListForRole[T any](c *gin.Context, items []T, hidden []string, flags func(T) map[string]any) (any, error) {
	if isAdmin(c) {
		return items, nil
	}
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		redacted, err := redactItem(item, hidden, flags)
		if err != nil {
			return nil, err
		}
		out = append(out, redacted)
	}
	return out, nil
}

// redactItem converts item to a JSON object, removes the hidden keys and adds
// the derived flags. Numbers are kept as json.Number so ids and counters keep
// their exact values.
func redactItem[T any](item T, hidden []string, flags func(T) map[string]any) (map[string]any, error) {
	raw, err := json.Marshal(item)
	if err != nil {
		return nil, fmt.Errorf("encode response for redaction: %w", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var fields map[string]any
	if err := decoder.Decode(&fields); err != nil {
		return nil, fmt.Errorf("decode response for redaction: %w", err)
	}
	for _, key := range hidden {
		delete(fields, key)
	}
	if flags != nil {
		for key, value := range flags(item) {
			fields[key] = value
		}
	}
	return fields, nil
}

func respondPermissionError(c *gin.Context, securityService *services.SecurityService, action string, err error, path string) bool {
	code, ok := util.MapSaveErrorCode(err)
	if !ok {
		return false
	}

	admin := isAdmin(c)
	response := gin.H{
		"error":      permissionErrorMessage(code),
		"error_code": code,
	}

	if admin {
		if path != "" {
			response["path"] = path
		}
		response["help"] = buildPermissionHelp(path)
	} else {
		response["help"] = "Check volume permissions or contact an administrator."
	}

	logPermissionAudit(securityService, c, action, code, path, admin)
	c.JSON(http.StatusInternalServerError, response)
	return true
}

func permissionErrorMessage(code string) string {
	switch code {
	case "permissions_db_readonly":
		return "database is read-only"
	case "permissions_db_locked":
		return "database is locked"
	case "permissions_readonly":
		return "filesystem is read-only"
	case "permissions_write_denied":
		return "permission denied"
	default:
		return "permission error"
	}
}

func buildPermissionHelp(path string) string {
	uid := os.Geteuid()
	gid := os.Getegid()
	if path == "" {
		return fmt.Sprintf("chown -R %d:%d <path-to-volume>", uid, gid)
	}
	return fmt.Sprintf("chown -R %d:%d %s", uid, gid, path)
}

func logPermissionAudit(securityService *services.SecurityService, c *gin.Context, action, code, path string, admin bool) {
	if securityService == nil {
		return
	}

	details := map[string]any{
		"error_code": code,
		"admin":      admin,
	}
	if admin && path != "" {
		details["path"] = path
	}
	detailsJSON, _ := json.Marshal(details)

	_ = securityService.LogAudit(&models.SecurityAudit{
		Actor:         auditActor(c),
		Action:        action,
		EventCategory: "permissions",
		Details:       string(detailsJSON),
		IPAddress:     c.ClientIP(),
		UserAgent:     c.Request.UserAgent(),
	})
}
