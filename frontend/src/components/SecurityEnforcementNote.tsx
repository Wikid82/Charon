import { useTranslation } from 'react-i18next'

import type { SecurityEnforcement } from '../api/security'

/** Modules that report whether the proxy applies them. */
export type EnforcementModuleName = 'waf' | 'rate_limit'

/** Reasons the backend reports per module. Any other reason shows the generic line. */
const KNOWN_REASONS: Record<EnforcementModuleName, readonly string[]> = {
  waf: ['no_ruleset', 'ruleset_not_selected', 'ruleset_unavailable'],
  rate_limit: ['thresholds_unset'],
}

/** Translation key for a module's enforcement reason, or the generic line for an unknown reason. */
function enforcementMessageKey(moduleName: EnforcementModuleName, reason: string | undefined): string {
  const known = reason !== undefined && KNOWN_REASONS[moduleName].includes(reason)
  return known ? `security.enforcement.${moduleName}.${reason}` : 'security.enforcement.unknown'
}

/**
 * Passive state line for a module that is on but not applied to every host. Renders nothing when the
 * report omits `enforced` (module off, no hosts, or stale) or when the module is applied.
 */
export function SecurityEnforcementNote({
  moduleName,
  state,
}: {
  moduleName: EnforcementModuleName
  state?: SecurityEnforcement
}) {
  const { t } = useTranslation()
  if (!state || state.enforced !== false) return null
  return (
    <p className="mt-2 text-sm text-content-muted" data-testid={`${moduleName}-enforcement-note`}>
      {t(enforcementMessageKey(moduleName, state.reason))}
    </p>
  )
}
