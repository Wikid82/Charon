package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"

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
	if _, exists := c.Get("userID"); !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "Authorization header required",
		})
		return false
	}

	return requireAdmin(c)
}

func isAdmin(c *gin.Context) bool {
	return c.GetString("role") == string(models.RoleAdmin)
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

	actor := "unknown"
	if userID, ok := c.Get("userID"); ok {
		actor = fmt.Sprintf("%v", userID)
	}

	_ = securityService.LogAudit(&models.SecurityAudit{
		Actor:         actor,
		Action:        action,
		EventCategory: "permissions",
		Details:       string(detailsJSON),
		IPAddress:     c.ClientIP(),
		UserAgent:     c.Request.UserAgent(),
	})
}
