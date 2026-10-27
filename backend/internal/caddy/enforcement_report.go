package caddy

// Reasons a security module that is switched on is not applied to every eligible host. The security status reports
// them so the admin can see the applied state.
const (
	// ReasonNoRuleset means the WAF is on and no ruleset exists.
	ReasonNoRuleset = "no_ruleset"
	// ReasonRulesetNotSelected means rulesets exist, but none is chosen for the host.
	ReasonRulesetNotSelected = "ruleset_not_selected"
	// ReasonRulesetUnavailable means the chosen ruleset has no file on disk.
	ReasonRulesetUnavailable = "ruleset_unavailable"
	// ReasonModeDisabled means the default row holds the disabled WAF mode while the WAF key is on.
	// It is an interim reason; the veto that produces it is removed with the mode reconciliation.
	ReasonModeDisabled = "mode_disabled"
	// ReasonThresholdsUnset means the rate limit is on without a request count or a window.
	ReasonThresholdsUnset = "thresholds_unset"
)

// ModuleEnforcement counts the hosts one security module applies to.
type ModuleEnforcement struct {
	// Eligible is the number of enabled hosts the module applies to.
	Eligible int
	// Protected is the number of eligible hosts that got the module's handler.
	Protected int
	// Reason explains the first eligible host that did not get the handler. It is empty when none was left out.
	Reason string
}

// Enforced reports whether every eligible host got the handler. It is false when no host is eligible.
func (m ModuleEnforcement) Enforced() bool {
	return m.Eligible > 0 && m.Protected == m.Eligible
}

// record counts one eligible host. A host without the handler keeps the first reason recorded.
func (m *ModuleEnforcement) record(hasHandler bool, reason string) {
	m.Eligible++
	if hasHandler {
		m.Protected++
		return
	}
	if m.Reason == "" {
		m.Reason = reason
	}
}

// EnforcementReport describes what GenerateConfig built for the security modules. It is stamped with the flags it was
// generated for, so a reader can tell whether it still matches the current module state.
type EnforcementReport struct {
	WAFOn       bool
	RateLimitOn bool
	WAF         ModuleEnforcement
	RateLimit   ModuleEnforcement
}
