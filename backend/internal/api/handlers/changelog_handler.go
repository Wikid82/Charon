package handlers

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/changelog"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/version"
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
// dismiss_permanent advances LastSeenVersion to the bare SemVer form of the
// running version so the modal won't reappear for these entries. It never
// moves LastSeenVersion backwards, and an unversioned build writes nothing.
// dismiss_temporary leaves
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
		// Fixed message on purpose: binder errors expose field names and
		// validator internals. The details are not useful to the client.
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	updates := map[string]any{}
	if req.Action == "dismiss_permanent" {
		if anchor := h.svc.AnchorVersion(); anchor != "" {
			var user models.User
			if err := h.db.Select("id", "last_seen_version").First(&user, userID).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
					return
				}
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update changelog state"})
				return
			}
			// last_seen_version never moves backwards. After a downgrade, a
			// stored higher version stays put (F15). An empty or invalid
			// stored value is always replaced by the anchor.
			if advancesLastSeen(user.LastSeenVersion, anchor) {
				updates["last_seen_version"] = anchor
			}
		}
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

// advancesLastSeen reports whether anchor should replace the stored
// last-seen value. Compare is false when stored is empty or not valid SemVer,
// and in that case the anchor always wins.
func advancesLastSeen(stored, anchor string) bool {
	cmp, ok := version.Compare(anchor, stored)
	return !ok || cmp > 0
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
