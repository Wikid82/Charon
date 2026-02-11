package util

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"syscall"
)

type PermissionCheck struct {
	Path      string `json:"path"`
	Required  string `json:"required"`
	Exists    bool   `json:"exists"`
	Writable  bool   `json:"writable"`
	OwnerUID  int    `json:"owner_uid"`
	OwnerGID  int    `json:"owner_gid"`
	Mode      string `json:"mode"`
	Error     string `json:"error,omitempty"`
	ErrorCode string `json:"error_code,omitempty"`
}

func CheckPathPermissions(path, required string) PermissionCheck {
	result := PermissionCheck{
		Path:     path,
		Required: required,
	}

	info, err := os.Stat(path)
	if err != nil {
		result.Writable = false
		result.Error = err.Error()
		result.ErrorCode = MapDiagnosticErrorCode(err)
		return result
	}

	result.Exists = true

	if stat, ok := info.Sys().(*syscall.Stat_t); ok {
		result.OwnerUID = int(stat.Uid)
		result.OwnerGID = int(stat.Gid)
	}
	result.Mode = fmt.Sprintf("%04o", info.Mode().Perm())

	if !info.IsDir() && !info.Mode().IsRegular() {
		result.Writable = false
		result.Error = "unsupported file type"
		result.ErrorCode = "permissions_unsupported_type"
		return result
	}

	if strings.Contains(required, "w") {
		if info.IsDir() {
			probeFile, probeErr := os.CreateTemp(path, "permcheck-*")
			if probeErr != nil {
				result.Writable = false
				result.Error = probeErr.Error()
				result.ErrorCode = MapDiagnosticErrorCode(probeErr)
				return result
			}
			if closeErr := probeFile.Close(); closeErr != nil {
				result.Writable = false
				result.Error = closeErr.Error()
				result.ErrorCode = MapDiagnosticErrorCode(closeErr)
				return result
			}
			if removeErr := os.Remove(probeFile.Name()); removeErr != nil {
				result.Writable = false
				result.Error = removeErr.Error()
				result.ErrorCode = MapDiagnosticErrorCode(removeErr)
				return result
			}
			result.Writable = true
			return result
		}

		file, openErr := os.OpenFile(path, os.O_WRONLY, 0)
		if openErr != nil {
			result.Writable = false
			result.Error = openErr.Error()
			result.ErrorCode = MapDiagnosticErrorCode(openErr)
			return result
		}
		if closeErr := file.Close(); closeErr != nil {
			result.Writable = false
			result.Error = closeErr.Error()
			result.ErrorCode = MapDiagnosticErrorCode(closeErr)
			return result
		}
		result.Writable = true
		return result
	}

	result.Writable = false
	return result
}

func MapDiagnosticErrorCode(err error) string {
	switch {
	case err == nil:
		return ""
	case os.IsNotExist(err):
		return "permissions_missing_path"
	case errors.Is(err, syscall.EROFS):
		return "permissions_readonly"
	case errors.Is(err, syscall.EACCES) || os.IsPermission(err):
		return "permissions_write_denied"
	default:
		return "permissions_write_failed"
	}
}

func MapSaveErrorCode(err error) (string, bool) {
	switch {
	case err == nil:
		return "", false
	case IsSQLiteReadOnlyError(err):
		return "permissions_db_readonly", true
	case IsSQLiteLockedError(err):
		return "permissions_db_locked", true
	case errors.Is(err, syscall.EROFS):
		return "permissions_readonly", true
	case errors.Is(err, syscall.EACCES) || os.IsPermission(err):
		return "permissions_write_denied", true
	case strings.Contains(strings.ToLower(err.Error()), "permission denied"):
		return "permissions_write_denied", true
	default:
		return "", false
	}
}

func IsSQLiteReadOnlyError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "readonly") ||
		strings.Contains(msg, "read-only") ||
		strings.Contains(msg, "attempt to write a readonly database") ||
		strings.Contains(msg, "sqlite_readonly")
}

func IsSQLiteLockedError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "database is locked") ||
		strings.Contains(msg, "sqlite_busy") ||
		strings.Contains(msg, "database locked")
}
