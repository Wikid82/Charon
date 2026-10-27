package handlers

import (
	"github.com/gin-gonic/gin"

	"github.com/Wikid82/charon/backend/internal/caddy"
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
