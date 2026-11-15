package handlers

import (
	"context"
	"errors"
	"net/http"
	"net/netip"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/api/middleware"
	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/network"
	"github.com/Wikid82/charon/backend/internal/security"
	"github.com/Wikid82/charon/backend/internal/services"
	"github.com/Wikid82/charon/backend/internal/util"
)

// codeManagementLockout and codeAccessListInvalid are the coded answers of the lock-out guards.
const (
	codeManagementLockout = "management_access_would_lock_out"
	codeAccessListInvalid = "access_list_invalid"
	msgManagementLockout  = "This change would lock you out of Charon. Add your address to the admin allow-list, or choose a different access list."
)

// managementGuard judges changes to the access lists that protect Charon's own interface. A change that would refuse
// the calling administrator is refused before any write. The emergency reset is never refused, and an emergency
// bypass request skips the check.
type managementGuard struct {
	db      *gorm.DB
	aclMode string
}

// newManagementGuard returns the guard for db. aclMode is the static access-control mode of the configuration.
func newManagementGuard(db *gorm.DB, aclMode string) managementGuard {
	return managementGuard{db: db, aclMode: aclMode}
}

// managementState is the access state the guard judges: the stored state with the part under review replaced.
type managementState struct {
	clientAddr string
	aclOn      bool
	lists      []string
	admin      []string
	// override is the proposed state of one list in the management set, when a list is being edited.
	override *models.AccessList
}

// current reads the stored state for a caller at clientAddr.
func (g managementGuard) current(clientAddr string) (managementState, error) {
	lists, err := services.ManagementListUUIDs(g.db)
	if err != nil {
		return managementState{}, err
	}
	return managementState{
		clientAddr: clientAddr,
		aclOn:      services.ACLEnabled(services.SettingGetter(g.db), config.SecurityConfig{ACLMode: g.aclMode}),
		lists:      lists,
		admin:      services.NewAccessListService(g.db).AdminWhitelistRanges(),
	}, nil
}

// refuses reports whether the proposed state would refuse the caller. When it would, the response is written and
// refuses returns true. A stored list that cannot be used is answered 400 with the list named; an unusable caller
// address counts as a lock-out.
func (g managementGuard) refuses(c *gin.Context, st managementState) bool {
	if middleware.IsEmergencyBypass(c) || !st.aclOn || len(st.lists) == 0 {
		return false
	}
	verdict, err := services.NewAccessListService(g.db).EvaluateManagement(st.lists, st.clientAddr, st.admin, st.override)
	var listErr *services.ManagementListError
	switch {
	case err == nil && verdict.Allowed:
		return false
	case errors.As(err, &listErr):
		c.JSON(http.StatusBadRequest, gin.H{"error": "an access list chosen for Charon's own interface cannot be used", "error_code": codeAccessListInvalid, "list": listErr.List})
	case err != nil && !errors.Is(err, services.ErrUnusableClientAddress):
		logger.Log().WithField("error", util.SanitizeForLog(err.Error())).Error("Management access check failed")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to check management access"})
	default:
		respondCoded(c, http.StatusForbidden, codeManagementLockout, msgManagementLockout, nil)
	}
	return true
}

// callerAddress is the address the proxy decided on for this request, as the management checks read it.
func callerAddress(c *gin.Context) string {
	return util.CanonicalizeIPForSecurity(c.ClientIP())
}

// callerAddressUnder recomputes the caller address the proxy would decide on under the trusted list. A request that
// came through Charon's proxy is judged from the proxy's peer and the client's forwarded chain; any other request keeps
// its connection address, which trust does not change.
func callerAddressUnder(c *gin.Context, trusted []netip.Prefix) string {
	origin, ok := middleware.RequestOriginFrom(c)
	if !ok || origin.ProxyPeer == "" {
		return callerAddress(c)
	}
	peer, err := netip.ParseAddr(origin.ProxyPeer)
	if err != nil {
		return callerAddress(c)
	}
	return security.StrictClientAddr(peer, origin.ForwardedFor, trusted).String()
}

// refuseAdminAllowList judges a proposed admin allow-list before it is saved. It returns true when the change was
// refused and the response written.
func refuseAdminAllowList(c *gin.Context, db *gorm.DB, aclMode, proposed string) bool {
	guard := newManagementGuard(db, aclMode)
	st, err := guard.current(callerAddress(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to check management access"})
		return true
	}
	st.admin = network.SplitAddressList(proposed)
	return guard.refuses(c, st)
}

// geoipChangeContext returns the request context for a GeoIP database change. It carries the management lock-out
// check, which runs against the candidate database before the live one is replaced. An emergency bypass request is not
// checked, and neither is a handler without a database, which holds no management set.
func (h *SecurityHandler) geoipChangeContext(c *gin.Context) context.Context {
	ctx := c.Request.Context()
	if middleware.IsEmergencyBypass(c) || h.db == nil {
		return ctx
	}
	guard := newManagementGuard(h.db, h.cfg.ACLMode)
	st, err := guard.current(callerAddress(c))
	if err != nil {
		return services.WithCandidateGuard(ctx, func(string) error { return err })
	}
	return services.WithCandidateGuard(ctx, services.ManagementCandidateCheck(h.db, st.lists, st.clientAddr, st.admin, st.aclOn))
}
