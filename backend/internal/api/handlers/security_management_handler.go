package handlers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/services"
)

// codeManagementListUnknown answers a management set that names a list which does not exist.
const codeManagementListUnknown = "management_list_unknown"

// ManagementAccessHandler serves the access lists that protect Charon's own interface.
type ManagementAccessHandler struct {
	db      *gorm.DB
	aclMode string
	cerb    CacheInvalidator
}

// NewManagementAccessHandler creates the handler. cerb may be nil.
func NewManagementAccessHandler(db *gorm.DB, aclMode string, cerb CacheInvalidator) *ManagementAccessHandler {
	return &ManagementAccessHandler{db: db, aclMode: aclMode, cerb: cerb}
}

type managementAccessRequest struct {
	ListUUIDs []string `json:"list_uuids"`
}

type managementListSummary struct {
	UUID    string `json:"uuid"`
	Name    string `json:"name"`
	Type    string `json:"type"`
	Enabled bool   `json:"enabled"`
}

type managementAccessResponse struct {
	Lists []managementListSummary `json:"lists"`
	// Enforced reports whether the chosen lists are applied now: access control is on and at least one chosen list is
	// enabled.
	Enforced bool `json:"enforced"`
}

// Get returns the chosen lists. A chosen list that no longer exists is omitted.
func (h *ManagementAccessHandler) Get(c *gin.Context) {
	uuids, err := services.ManagementListUUIDs(h.db)
	if err != nil {
		logger.Log().WithError(err).Error("Failed to read the management access choice")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read management access"})
		return
	}
	h.respond(c, http.StatusOK, uuids)
}

// Put replaces the chosen lists. A choice that would refuse the caller is refused before the write.
func (h *ManagementAccessHandler) Put(c *gin.Context) {
	var req managementAccessRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	uuids, ok := h.knownUUIDs(c, req.ListUUIDs)
	if !ok {
		return
	}

	guard := newManagementGuard(h.db, h.aclMode)
	st, err := guard.current(callerAddress(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to check management access"})
		return
	}
	st.lists = uuids
	if guard.refuses(c, st) {
		return
	}

	err = services.WithWriteLock(h.db, &models.Setting{}, func(tx *gorm.DB) error {
		return services.WriteManagementLists(tx, uuids)
	})
	if err != nil {
		logger.Log().WithError(err).Error("Failed to save the management access choice")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save management access"})
		return
	}
	if h.cerb != nil {
		h.cerb.InvalidateCache()
	}
	h.respond(c, http.StatusOK, uuids)
}

// knownUUIDs returns the de-duplicated choice. A UUID that names no access list answers 400 and ok is false.
func (h *ManagementAccessHandler) knownUUIDs(c *gin.Context, requested []string) ([]string, bool) {
	accessSvc := services.NewAccessListService(h.db)
	seen := make(map[string]bool, len(requested))
	uuids := make([]string, 0, len(requested))
	for _, raw := range requested {
		uuid := strings.TrimSpace(raw)
		if uuid == "" || seen[uuid] {
			continue
		}
		if _, err := accessSvc.GetByUUID(uuid); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "access list not found", "error_code": codeManagementListUnknown, "uuid": uuid})
			return nil, false
		}
		seen[uuid] = true
		uuids = append(uuids, uuid)
	}
	return uuids, true
}

// respond writes the chosen lists in their stored order, with the enforcement state.
func (h *ManagementAccessHandler) respond(c *gin.Context, status int, uuids []string) {
	accessSvc := services.NewAccessListService(h.db)
	lists := make([]managementListSummary, 0, len(uuids))
	enabledChosen := false
	for _, uuid := range uuids {
		acl, err := accessSvc.GetByUUID(uuid)
		if err != nil {
			continue
		}
		lists = append(lists, managementListSummary{UUID: acl.UUID, Name: acl.Name, Type: acl.Type, Enabled: acl.Enabled})
		enabledChosen = enabledChosen || acl.Enabled
	}
	aclOn := services.ACLEnabled(services.SettingGetter(h.db), config.SecurityConfig{ACLMode: h.aclMode})
	c.JSON(status, managementAccessResponse{Lists: lists, Enforced: aclOn && enabledChosen})
}
