package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/changelog"
	"github.com/Wikid82/charon/backend/internal/models"
)

// ChangelogHandler serves the "What's New" changelog: the per-user
// unseen-entries status check, the full browsable history, and the two
// self-service dismiss/opt-in mutations. All four routes are
// self-service-on-your-own-user-row, registered under the `protected`
// group (see routes.go) — RolePassthrough users are rejected explicitly
// via rejectPassthrough, matching GetProfile/UpdateProfile/RegenerateAPIKey.
type ChangelogHandler struct {
	db  *gorm.DB
	svc *changelog.Service
}

// NewChangelogHandler constructs a ChangelogHandler.
func NewChangelogHandler(db *gorm.DB, svc *changelog.Service) *ChangelogHandler {
	return &ChangelogHandler{db: db, svc: svc}
}

// Status returns whether the current user has unseen changelog entries,
// and the entries themselves. Always show_changelog=false for dev builds,
// opted-out users, and when there are zero unseen entries — the modal
// must never render empty.
func (h *ChangelogHandler) Status(c *gin.Context) {
	if rejectPassthrough(c, "view the changelog") {
		return
	}
	userID, ok := requireUserID(c)
	if !ok {
		return
	}

	var user models.User
	if err := h.db.First(&user, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	if h.svc.IsDevBuild() || user.ChangelogOptOut {
		c.JSON(http.StatusOK, gin.H{"show_changelog": false, "versions": []changelog.Entry{}})
		return
	}

	entries := h.svc.GetEntriesSince(user.LastSeenVersion)
	c.JSON(http.StatusOK, gin.H{
		"show_changelog": len(entries) > 0,
		"versions":       entries,
	})
}

// All returns the full changelog history, independent of the caller's
// last-seen version — used by the Settings "revisit" browse mode.
func (h *ChangelogHandler) All(c *gin.Context) {
	if rejectPassthrough(c, "view the changelog") {
		return
	}
	c.JSON(http.StatusOK, gin.H{"versions": h.svc.GetAllEntries()})
}

// AckRequest is the body of POST /changelog/ack.
type AckRequest struct {
	Action string `json:"action" binding:"required,oneof=dismiss_temporary dismiss_permanent"`
	OptOut bool   `json:"opt_out"`
}

// Ack records the user's response to the changelog modal.
// dismiss_permanent advances LastSeenVersion to the running version so
// the modal won't reappear for these entries; dismiss_temporary leaves
// it untouched so the modal reappears next login. Either action may also
// set ChangelogOptOut via opt_out.
func (h *ChangelogHandler) Ack(c *gin.Context) {
	if rejectPassthrough(c, "update changelog preferences") {
		return
	}
	userID, ok := requireUserID(c)
	if !ok {
		return
	}

	var req AckRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	updates := map[string]any{}
	// Normal UI flow only reaches dismiss_permanent when show_changelog
	// was already true, which itself requires !IsDevBuild() (see
	// Status) — but a client can POST /changelog/ack directly, so guard
	// here too: never write an unversioned/non-semver CurrentVersion()
	// into LastSeenVersion, or the user would be permanently stuck once
	// the deployment upgrades to a real tagged release (same failure
	// mode as an unguarded seedLastSeenVersion; see
	// changelog.IsUnversionedBuild). Leave last_seen_version untouched
	// rather than writing a value we know is invalid.
	if req.Action == "dismiss_permanent" && !h.svc.IsDevBuild() {
		updates["last_seen_version"] = h.svc.CurrentVersion()
	}
	if req.OptOut {
		updates["changelog_opt_out"] = true
	}
	if len(updates) == 0 {
		c.JSON(http.StatusOK, gin.H{"message": "No change"})
		return
	}

	result := h.db.Model(&models.User{}).Where("id = ?", userID).Updates(updates)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update changelog state"})
		return
	}
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Acknowledged"})
}

// OptIn re-enables changelog notifications for the current user (clears
// ChangelogOptOut). Used by the Appearance Settings toggle.
func (h *ChangelogHandler) OptIn(c *gin.Context) {
	if rejectPassthrough(c, "update changelog preferences") {
		return
	}
	userID, ok := requireUserID(c)
	if !ok {
		return
	}

	result := h.db.Model(&models.User{}).Where("id = ?", userID).Update("changelog_opt_out", false)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to opt in"})
		return
	}
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Opted in"})
}
