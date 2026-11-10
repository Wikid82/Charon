package handlers

import (
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"

	"github.com/Wikid82/charon/backend/internal/caddy"
	"github.com/Wikid82/charon/backend/internal/geoip"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/services"
	"github.com/Wikid82/charon/backend/internal/util"
)

// geoipStatusResponse is the body of GET /security/geoip/status. The first four fields and reason are the original
// contract; the database card fields follow, flattened from services.GeoIPStatus. reason is present only when the
// database is unavailable, so the card can show why country lists cannot match.
type geoipStatusResponse struct {
	Available bool   `json:"available"`
	Loaded    bool   `json:"loaded"`
	DBPath    string `json:"db_path"`
	Message   string `json:"message"`
	Reason    string `json:"reason,omitempty"`
	services.GeoIPStatus
}

// GetGeoIPStatus returns the current status of the GeoIP service and the database card.
func (h *SecurityHandler) GetGeoIPStatus(c *gin.Context) {
	available := h.geoipSvc.Available()
	body := geoipStatusResponse{
		Available:   available,
		Message:     "GeoIP service not initialized",
		GeoIPStatus: h.geoipStatus(),
	}
	if h.geoipSvc != nil {
		body.Loaded = h.geoipSvc.IsLoaded()
		body.DBPath = h.geoipSvc.GetDatabasePath()
		body.Message = "GeoIP database unavailable"
	}
	if available {
		body.Message = "GeoIP service available"
	} else {
		body.Reason = caddy.ReasonGeoIPUnavailable
	}
	c.JSON(http.StatusOK, body)
}

// geoipStatus returns the database card status with the number of country lists the proxy is not enforcing. Without a
// store (no GeoIP service is wired) it reports no database.
func (h *SecurityHandler) geoipStatus() services.GeoIPStatus {
	if h.geoipStore == nil {
		return services.GeoIPStatus{Source: string(geoip.SourceNone)}
	}
	st := h.geoipStore.Status()
	st.UnenforcedListsTotal = h.unenforcedCountryListsTotal()
	return st
}

// unenforcedCountryListsTotal counts the country lists the proxy is not enforcing. Without a database there is no
// access-list state to read, so it reports zero.
func (h *SecurityHandler) unenforcedCountryListsTotal() int {
	if h.db == nil {
		return 0
	}
	return len(h.countryListsNotEnforced(h.aclEnabledForAlerts()))
}

// Stable error_code values and fixed messages of the GeoIP reload endpoint. The error text of the underlying failure
// (which can name filesystem paths) is logged server-side only and never returned to the client.
const (
	codeGeoIPReloadFailed       = "geoip_reload_failed"
	codeGeoIPServiceUnavailable = "geoip_service_unavailable"
	codeGeoIPBusy               = "geoip_busy"
	codeProxyApplyFailed        = "proxy_apply_failed"
	msgGeoIPReloadFailed        = "Failed to reload the GeoIP database"
	msgGeoIPBusy                = "Another GeoIP database change is in progress"
)

// ReloadGeoIP reloads the GeoIP database from disk.
func (h *SecurityHandler) ReloadGeoIP(c *gin.Context) {
	if !requireAdmin(c) {
		return
	}

	if h.geoipStore == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error":      "GeoIP service not initialized",
			"error_code": codeGeoIPServiceUnavailable,
		})
		return
	}

	// The store loads the file and applies the proxy config, so country lists follow the reload at once.
	if err := h.geoipStore.Reload(c.Request.Context()); err != nil {
		geoipErrorResponse(c, err)
		return
	}

	// Log audit event
	actor := auditActor(c)
	_ = h.svc.LogAudit(&models.SecurityAudit{Actor: actor, Action: "reload_geoip", Details: "GeoIP database reloaded successfully"})

	c.JSON(http.StatusOK, gin.H{
		"message": "GeoIP database reloaded successfully",
		"loaded":  h.geoipSvc.IsLoaded(),
		"db_path": h.geoipSvc.GetDatabasePath(),
		"status":  h.geoipStatus(),
	})
}

// Stable error_code values and fixed messages of the GeoIP database endpoints. Replace and remove share them with the
// reload endpoint, so one failure has one code wherever it surfaces.
const (
	codeGeoIPFileRequired     = "geoip_file_required"
	codeGeoIPTooLarge         = "geoip_too_large"
	codeGeoIPNotMMDB          = "geoip_not_mmdb"
	codeGeoIPCorrupt          = "geoip_corrupt"
	codeGeoIPWrongType        = "geoip_wrong_type" //nolint:gosec // G101: stable error_code string, not a credential
	codeGeoIPOlderThanCurrent = "geoip_older_than_current"
	codeGeoIPBuildInFuture    = "geoip_build_date_in_future"
	codeGeoIPCustomPathSet    = "geoip_custom_path_set"
	codeGeoIPNoDataCopy       = "geoip_no_data_copy"
	codeGeoIPDataReadOnly     = "data_folder_read_only"
	codeGeoIPWriteFailed      = "geoip_write_failed" //nolint:gosec // G101: stable error_code string, not a credential

	msgGeoIPFileRequired  = "Upload the database as one file part named file"
	msgGeoIPTooLarge      = "The GeoIP database is larger than 32 MiB"
	msgGeoIPNotMMDB       = "The file is not a MaxMind database"
	msgGeoIPCorrupt       = "The file is a damaged MaxMind database or has no build date"
	msgGeoIPWrongType     = "The file is not a country database"
	msgGeoIPOlderCurrent  = "The file is older than the database in use or the bundled copy"
	msgGeoIPBuildInFuture = "The file's build date is in the future"
	msgGeoIPCustomPathSet = "The GeoIP database is set by CHARON_GEOIP_DB_PATH and cannot be changed here"
	msgGeoIPNoDataCopy    = "There is no uploaded GeoIP database to remove"
	msgGeoIPDataReadOnly  = "The GeoIP data folder is read-only"
	msgGeoIPWriteFailed   = "Failed to store the GeoIP database"
)

// geoipFailure is the HTTP answer for a failed GeoIP database change. refused marks an upload that was refused after its
// content was read; those are written to the audit log.
type geoipFailure struct {
	status  int
	code    string
	msg     string
	refused bool
}

// geoipFailureFor maps a failure of the store or the upload to its answer. Unknown failures map to a generic 500 that
// carries no detail.
func geoipFailureFor(err error) geoipFailure {
	var maxErr *http.MaxBytesError
	var older *services.OlderThanCurrentError
	switch {
	case errors.Is(err, services.ErrGeoIPBusy):
		return geoipFailure{status: http.StatusConflict, code: codeGeoIPBusy, msg: msgGeoIPBusy}
	case errors.Is(err, services.ErrGeoIPCustomPathSet):
		return geoipFailure{status: http.StatusConflict, code: codeGeoIPCustomPathSet, msg: msgGeoIPCustomPathSet}
	case errors.Is(err, services.ErrGeoIPNoDataCopy):
		return geoipFailure{status: http.StatusNotFound, code: codeGeoIPNoDataCopy, msg: msgGeoIPNoDataCopy}
	case errors.Is(err, services.ErrGeoIPTooLarge), errors.As(err, &maxErr):
		return geoipFailure{status: http.StatusRequestEntityTooLarge, code: codeGeoIPTooLarge, msg: msgGeoIPTooLarge, refused: true}
	case errors.Is(err, services.ErrGeoIPUploadRead):
		return geoipFailure{status: http.StatusBadRequest, code: codeGeoIPFileRequired, msg: msgGeoIPFileRequired, refused: true}
	case errors.Is(err, geoip.ErrNotMMDB):
		return geoipFailure{status: http.StatusBadRequest, code: codeGeoIPNotMMDB, msg: msgGeoIPNotMMDB, refused: true}
	case errors.Is(err, geoip.ErrCorruptDatabase), errors.Is(err, geoip.ErrNoBuildDate):
		return geoipFailure{status: http.StatusBadRequest, code: codeGeoIPCorrupt, msg: msgGeoIPCorrupt, refused: true}
	case errors.Is(err, geoip.ErrWrongDatabaseType):
		return geoipFailure{status: http.StatusBadRequest, code: codeGeoIPWrongType, msg: msgGeoIPWrongType, refused: true}
	case errors.As(err, &older):
		return geoipFailure{status: http.StatusConflict, code: codeGeoIPOlderThanCurrent, msg: msgGeoIPOlderCurrent, refused: true}
	case errors.Is(err, services.ErrGeoIPBuildDateInFuture):
		return geoipFailure{status: http.StatusBadRequest, code: codeGeoIPBuildInFuture, msg: msgGeoIPBuildInFuture, refused: true}
	case errors.Is(err, services.ErrGeoIPReloadFailed):
		return geoipFailure{status: http.StatusInternalServerError, code: codeGeoIPReloadFailed, msg: msgGeoIPReloadFailed}
	case errors.Is(err, services.ErrGeoIPApplyFailed):
		return geoipFailure{status: http.StatusInternalServerError, code: codeProxyApplyFailed, msg: msgModuleApplyFailed}
	case errors.Is(err, services.ErrGeoIPDataFolderReadOnly):
		return geoipFailure{status: http.StatusConflict, code: codeGeoIPDataReadOnly, msg: msgGeoIPDataReadOnly}
	default:
		return geoipFailure{status: http.StatusInternalServerError, code: codeGeoIPWriteFailed, msg: msgGeoIPWriteFailed}
	}
}

// geoipErrorResponse writes the answer for a failed GeoIP database change. The body carries a fixed message, the
// error_code, and for an older upload the two build dates. The detail of err is logged, sanitized, and never returned.
func geoipErrorResponse(c *gin.Context, err error) {
	f := geoipFailureFor(err)
	body := gin.H{"error": f.msg, "error_code": f.code}
	var older *services.OlderThanCurrentError
	if errors.As(err, &older) {
		body["uploaded_build_date"] = formatBuildEpoch(older.Uploaded)
		body["current_build_date"] = formatBuildEpoch(older.Current)
	}
	detail := util.SanitizeForLog(err.Error())
	if f.status >= http.StatusInternalServerError {
		log.WithField("detail", detail).Error("GeoIP database change failed")
	} else {
		log.WithField("detail", detail).Warn("GeoIP database change refused")
	}
	c.JSON(f.status, body)
}

// formatBuildEpoch renders a MaxMind build epoch as an RFC 3339 UTC time, or "unknown" for a zero epoch.
func formatBuildEpoch(epoch uint) string {
	if epoch == 0 {
		return "unknown"
	}
	return geoip.BuildTime(epoch).UTC().Format(time.RFC3339)
}

// geoipUploadField is the only multipart part name the upload accepts.
const geoipUploadField = "file"

// geoipMultipartOverhead is the room left in the request limit for multipart framing around the file.
const geoipMultipartOverhead = 64 << 10

// errUnexpectedPart reports a request with a part after the file, or a part that is not the file.
var errUnexpectedPart = fmt.Errorf("%w: unexpected form part", services.ErrGeoIPUploadRead)

// singlePartReader reads the file part and, at its end, requires that no further part follows. A trailing part is an
// error, so the store never installs a file whose request carried more than the one part.
type singlePartReader struct {
	part *multipart.Part
	mr   *multipart.Reader
}

// Read reads the file part; at its end it checks that the multipart stream has no further part.
func (r singlePartReader) Read(p []byte) (int, error) {
	n, err := r.part.Read(p)
	if !errors.Is(err, io.EOF) {
		return n, err
	}
	next, nextErr := r.mr.NextPart()
	switch {
	case nextErr != nil && errors.Is(nextErr, io.EOF):
		return n, io.EOF
	case nextErr != nil:
		return n, nextErr
	default:
		_ = next.Close()
		return n, errUnexpectedPart
	}
}

// ReplaceGeoIPDatabase installs an uploaded country database as the data-folder copy. The request is one multipart
// part named file. The client's file name and content type are ignored and never stored.
func (h *SecurityHandler) ReplaceGeoIPDatabase(c *gin.Context) {
	if !requireAdmin(c) {
		return
	}
	if h.geoipStore == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error":      "GeoIP service not initialized",
			"error_code": codeGeoIPServiceUnavailable,
		})
		return
	}

	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, services.MaxGeoIPDatabaseBytes+geoipMultipartOverhead)
	mr, err := c.Request.MultipartReader()
	if err != nil {
		geoipErrorResponse(c, services.ClassifyUploadRead(err))
		return
	}
	part, err := mr.NextPart()
	if err == nil && part.FormName() != geoipUploadField {
		err = errUnexpectedPart
	}
	if err != nil {
		geoipErrorResponse(c, services.ClassifyUploadRead(err))
		return
	}

	result, err := h.geoipStore.Replace(c.Request.Context(), singlePartReader{part: part, mr: mr})
	if err != nil {
		h.auditRefusedUpload(c, err, result)
		geoipErrorResponse(c, err)
		return
	}

	h.auditGeoIP(c, "replace_geoip_database", fmt.Sprintf("type=%s build=%s size=%d sha256=%s",
		util.SanitizeForLog(result.Info.DatabaseType), formatBuildEpoch(result.Info.BuildEpoch), result.SizeBytes, result.SHA256))
	c.JSON(http.StatusOK, gin.H{
		"status":            h.geoipStatus(),
		"replaced_previous": result.ReplacedPrevious,
	})
}

// RemoveGeoIPDatabase deletes the data-folder copy so the image's bundled copy is used again.
func (h *SecurityHandler) RemoveGeoIPDatabase(c *gin.Context) {
	if !requireAdmin(c) {
		return
	}
	if h.geoipStore == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error":      "GeoIP service not initialized",
			"error_code": codeGeoIPServiceUnavailable,
		})
		return
	}

	removed := h.geoipStore.DataCopyInfo()
	if err := h.geoipStore.RemoveDataCopy(c.Request.Context()); err != nil {
		geoipErrorResponse(c, err)
		return
	}

	h.auditGeoIP(c, "remove_geoip_database", "build="+formatBuildEpoch(removed.BuildEpoch))
	c.JSON(http.StatusOK, gin.H{"status": h.geoipStatus()})
}

// auditRefusedUpload records an upload refused after its content was read, with its size and digest when known. The
// reason is the error_code the client received.
func (h *SecurityHandler) auditRefusedUpload(c *gin.Context, err error, result services.ReplaceResult) {
	f := geoipFailureFor(err)
	if !f.refused {
		return
	}
	h.auditGeoIP(c, "reject_geoip_database", fmt.Sprintf("reason=%s size=%d sha256=%s", f.code, result.SizeBytes, result.SHA256))
}

// auditGeoIP writes one audit row for a GeoIP database change. The details never carry a path or a client file name.
func (h *SecurityHandler) auditGeoIP(c *gin.Context, action, details string) {
	_ = h.svc.LogAudit(&models.SecurityAudit{
		Actor:   auditActor(c),
		Action:  action,
		Details: util.SanitizeForLog(details),
	})
}

// LookupGeoIP performs a GeoIP lookup for a given IP address.
func (h *SecurityHandler) LookupGeoIP(c *gin.Context) {
	if !requireAdmin(c) {
		return
	}

	var req struct {
		IPAddress string `json:"ip_address" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ip_address is required"})
		return
	}

	if h.geoipSvc == nil || !h.geoipSvc.IsLoaded() {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "GeoIP service not available",
		})
		return
	}

	country, err := h.geoipSvc.LookupCountry(req.IPAddress)
	if err != nil {
		if errors.Is(err, services.ErrInvalidGeoIP) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid IP address"})
			return
		}
		if errors.Is(err, services.ErrCountryNotFound) {
			c.JSON(http.StatusOK, gin.H{
				"ip_address":   req.IPAddress,
				"country_code": "",
				"found":        false,
				"message":      "No country found for this IP address",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "GeoIP lookup failed: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"ip_address":   req.IPAddress,
		"country_code": country,
		"found":        true,
	})
}
