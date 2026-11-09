package handlers

import (
	"github.com/gin-gonic/gin"

	"github.com/Wikid82/charon/backend/internal/caddy"
	"github.com/Wikid82/charon/backend/internal/geoip"
	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/services"
)

// enforcementReporter is implemented by the caddy manager. It returns the enforcement report of the last successful
// apply, read from memory.
type enforcementReporter interface {
	EnforcementReport() (caddy.EnforcementReport, bool)
}

// addAppliedState adds the applied-state fields to the WAF and rate-limit status objects. A module's fields are added
// only when the stored report was generated for the same on/off state the status resolved. A report from before a
// change, or from a failed apply, adds nothing, so the status never presents an old result as current.
func (h *SecurityHandler) addAppliedState(wafStatus, rateLimitStatus gin.H, wafOn, rateLimitOn bool) {
	reporter, ok := h.caddyManager.(enforcementReporter)
	if !ok {
		return
	}
	report, reported := reporter.EnforcementReport()
	if !reported {
		return
	}
	if report.WAFOn == wafOn {
		wafStatus["unprotected_hosts"] = report.WAF.Eligible - report.WAF.Protected
		addModuleAppliedState(wafStatus, report.WAF)
	}
	if report.RateLimitOn == rateLimitOn {
		addModuleAppliedState(rateLimitStatus, report.RateLimit)
	}
}

// addModuleAppliedState adds enforced and reason. enforced is omitted when no host is eligible, and reason is omitted
// when there is none to give.
func addModuleAppliedState(status gin.H, module caddy.ModuleEnforcement) {
	if module.Eligible > 0 {
		status["enforced"] = module.Enforced()
	}
	if module.Reason != "" {
		status["reason"] = module.Reason
	}
}

// countryListTypes are the access list types that match on the GeoIP country.
var countryListTypes = []string{"geo_whitelist", "geo_blacklist"}

// geoipDatabaseAvailable reports whether the GeoIP service has a usable database loaded.
func geoipDatabaseAvailable(svc *services.GeoIPService) bool {
	if svc == nil {
		return false
	}
	return geoip.DatabaseAvailable(svc.GetDatabasePath(), svc.IsLoaded())
}

// addACLGeoIPReason adds the geoip_unavailable reason to the ACL status when the ACL is on, an enabled country list
// is in use, and no GeoIP database is available. Such a list cannot match, so the status says why.
func (h *SecurityHandler) addACLGeoIPReason(aclStatus gin.H, aclEnabled bool) {
	if !aclEnabled || geoipDatabaseAvailable(h.geoipSvc) || !h.hasEnabledCountryList() {
		return
	}
	aclStatus["reason"] = caddy.ReasonGeoIPUnavailable
}

// hasEnabledCountryList reports whether at least one enabled access list matches on country. A failed read counts as
// none, so the status never reports a reason it cannot confirm.
func (h *SecurityHandler) hasEnabledCountryList() bool {
	if h.db == nil {
		return false
	}
	var count int64
	err := h.db.Model(&models.AccessList{}).Where("enabled = ? AND type IN ?", true, countryListTypes).Count(&count).Error
	if err != nil {
		logger.Log().WithError(err).Warn("Failed to count country access lists for status")
		return false
	}
	return count > 0
}
