package handlers

import (
	"github.com/gin-gonic/gin"

	"github.com/Wikid82/charon/backend/internal/caddy"
	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/models"
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

// appliedReporter is implemented by the caddy manager. It returns the report of the last successful apply.
type appliedReporter interface {
	LastAppliedReport() (caddy.EnforcementReport, bool)
}

// countryListsNotEnforced returns the sorted names of the country lists the proxy is not enforcing because no GeoIP
// database is usable. The alert and the ACL status reason both come from it, so they never disagree.
//
// After a successful apply the last applied report decides, and live database availability is not consulted: the
// report describes the running config until a newer apply succeeds. Before any successful apply there is no report, so
// the same rule the config generator uses is evaluated against the database. aclEnabled is the current ACL state and
// is used only for that fallback.
func (h *SecurityHandler) countryListsNotEnforced(aclEnabled bool) []string {
	if reporter, ok := h.caddyManager.(appliedReporter); ok {
		if report, applied := reporter.LastAppliedReport(); applied {
			if !report.ACLOn {
				return nil
			}
			return report.CountryListsWithoutDatabase
		}
	}
	return h.pendingCountryLists(aclEnabled)
}

// pendingCountryLists evaluates, before any apply, the rule the config generator uses: access control is on, no
// database is available, and an enabled country list is attached to an enabled host with a domain. A failed read
// counts as none, so the status never reports a reason it cannot confirm.
func (h *SecurityHandler) pendingCountryLists(aclEnabled bool) []string {
	if h.db == nil || !aclEnabled || h.geoipSvc.Available() {
		return nil
	}
	var names []string
	err := h.db.Model(&models.ProxyHost{}).
		Joins("JOIN access_lists ON access_lists.id = proxy_hosts.access_list_id").
		Where("proxy_hosts.enabled = ? AND proxy_hosts.domain_names <> ? AND access_lists.enabled = ? AND access_lists.type IN ?",
			true, "", true, countryListTypes).
		Distinct("access_lists.name").
		Order("access_lists.name").
		Pluck("access_lists.name", &names).Error
	if err != nil {
		logger.Log().WithError(err).Warn("Failed to read country access lists for status")
		return nil
	}
	return names
}

// addACLGeoIPReason adds the geoip_unavailable reason to the ACL status when a country list is not being enforced.
func (h *SecurityHandler) addACLGeoIPReason(aclStatus gin.H, aclEnabled bool) {
	if len(h.countryListsNotEnforced(aclEnabled)) > 0 {
		aclStatus["reason"] = caddy.ReasonGeoIPUnavailable
	}
}
