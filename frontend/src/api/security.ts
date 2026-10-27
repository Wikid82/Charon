import client from './client'

/**
 * Whether a module that is on is applied to the proxy. `enforced` is omitted when the report does not apply
 * (module off, no hosts, or a stale report); `reason` names the state when `enforced` is false.
 */
export interface SecurityEnforcement {
  enforced?: boolean
  reason?: string
}

/** Security module status information. */
export interface SecurityStatus {
  cerberus?: { enabled: boolean }
  crowdsec: {
    mode: 'disabled' | 'local'
    api_url: string
    enabled: boolean
  }
  waf: {
    mode: 'disabled' | 'enabled'
    enabled: boolean
    /** Hosts the WAF is not applied to while it is on. */
    unprotected_hosts?: number
  } & SecurityEnforcement
  rate_limit: {
    mode?: 'disabled' | 'enabled'
    enabled: boolean
  } & SecurityEnforcement
  acl: {
    enabled: boolean
  }
}

/**
 * Gets the current security status for all modules.
 * @returns Promise resolving to SecurityStatus
 * @throws {AxiosError} If the request fails
 */
export const getSecurityStatus = async (): Promise<SecurityStatus> => {
  const response = await client.get<SecurityStatus>('/security/status')
  return response.data
}

/** Address scope reported by the login-protection status. */
export type AddrScope = 'loopback' | 'private' | 'public'

/** One sign-in throttle budget: `requests` per `window_seconds`, per client. */
export interface LoginProtectionBudget {
  requests: number
  window_seconds: number
}

/**
 * Forwarded client-address headers received from peers that are not trusted proxies.
 * While `count` is 0, `last_seen` is null and `last_peer`/`last_peer_scope` are empty.
 */
export interface ForwardedHeaderObservation {
  count: number
  last_seen: string | null
  last_peer: string
  /** `loopback`/`private` for the local record, `public` for the public record. */
  last_peer_scope: AddrScope | ''
}

/** Admin-only login-protection (sign-in throttle) status. */
export interface LoginProtectionStatus {
  enabled: boolean
  login: LoginProtectionBudget
  session: LoginProtectionBudget
  /** Number of effective CHARON_TRUSTED_PROXIES entries. */
  trusted_proxy_count: number
  /** The throttle key Charon derives for the calling browser (IPv4, IPv6 /64, or `unknown`). */
  caller_client_key: string
  caller_client_scope: AddrScope
  untrusted_forwarded_headers: {
    local: ForwardedHeaderObservation
    public: ForwardedHeaderObservation
  }
}

/**
 * Gets the login-protection status (admin only).
 * @returns Promise resolving to LoginProtectionStatus
 * @throws {AxiosError} If the request fails (403 for non-admins)
 */
export const getLoginProtectionStatus = async (): Promise<LoginProtectionStatus> => {
  const response = await client.get<LoginProtectionStatus>('/security/login-protection')
  return response.data
}

/**
 * Standard rate-limit thresholds: the values the backend writes when the module is turned on without
 * thresholds, and the page placeholders and fallbacks. Keep in step with the backend constants.
 */
export const STANDARD_RATE_LIMIT = {
  requests: 100,
  windowSec: 60,
  burst: 20,
} as const

/**
 * Security configuration payload for a partial update. Every key is optional: a key that is absent (or null)
 * keeps the stored value, and an explicit empty string, 0 or false is applied.
 */
export interface SecurityConfigPayload {
  name?: string
  enabled?: boolean
  admin_whitelist?: string
  /** Managed by the CrowdSec switch and ignored by the security config save. */
  crowdsec_mode?: string
  crowdsec_api_url?: string
  waf_mode?: string
  waf_rules_source?: string
  waf_learning?: boolean
  waf_paranoia_level?: number
  waf_exclusions?: string
  rate_limit_mode?: string
  rate_limit_enable?: boolean
  rate_limit_burst?: number
  rate_limit_requests?: number
  rate_limit_window_sec?: number
  rate_limit_bypass_list?: string
}

/** Stored security configuration fields the UI reads. Other stored fields are not listed. */
export interface SecurityConfigRecord {
  name?: string
  admin_whitelist?: string
  waf_mode?: string
  waf_rules_source?: string
  rate_limit_requests?: number | null
  rate_limit_burst?: number | null
  rate_limit_window_sec?: number | null
}

/** Response of `GET /security/config`; `config` is null before the default row is seeded. */
export interface SecurityConfigResponse {
  config: SecurityConfigRecord | null
}

/**
 * Gets the current security configuration.
 * @returns Promise resolving to the security configuration
 * @throws {AxiosError} If the request fails
 */
export const getSecurityConfig = async (): Promise<SecurityConfigResponse> => {
  const response = await client.get<SecurityConfigResponse>('/security/config')
  return response.data
}

/**
 * Updates security configuration.
 *
 * Only the fields present in the payload are changed; omitted or null fields keep their stored value. Send an
 * empty string, 0 or false to clear or turn a field off. `crowdsec_mode` is managed by the CrowdSec switch and
 * is ignored here.
 *
 * @param payload - SecurityConfigPayload with the fields to change
 * @returns Promise resolving to the stored configuration
 * @throws {AxiosError} If update fails
 */
export const updateSecurityConfig = async (payload: SecurityConfigPayload) => {
  const response = await client.post('/security/config', payload)
  return response.data
}

/**
 * Generates a break-glass token for emergency access.
 * @returns Promise resolving to object containing the token
 * @throws {AxiosError} If generation fails
 */
export const generateBreakGlassToken = async () => {
  const response = await client.post('/security/breakglass/generate')
  return response.data
}

/**
 * Enables the Cerberus security module.
 * @param payload - Optional configuration for enabling
 * @returns Promise resolving to enable result
 * @throws {AxiosError} If enabling fails
 */
export const enableCerberus = async (payload?: Record<string, unknown>) => {
  const response = await client.post('/security/enable', payload || {})
  return response.data
}

/**
 * Disables the Cerberus security module.
 * @param payload - Optional configuration for disabling
 * @returns Promise resolving to disable result
 * @throws {AxiosError} If disabling fails
 */
export const disableCerberus = async (payload?: Record<string, unknown>) => {
  const response = await client.post('/security/disable', payload || {})
  return response.data
}

/**
 * Gets security decisions (bans, captchas) with optional limit.
 * @param limit - Maximum number of decisions to return (default: 50)
 * @returns Promise resolving to decisions list
 * @throws {AxiosError} If the request fails
 */
export const getDecisions = async (limit = 50) => {
  const response = await client.get(`/security/decisions?limit=${limit}`)
  return response.data
}

/** Payload for creating a security decision. */
export interface CreateDecisionPayload {
  type: string
  value: string
  duration: string
  reason?: string
}

/**
 * Creates a new security decision (e.g., ban an IP).
 * @param payload - Decision configuration
 * @returns Promise resolving to the created decision
 * @throws {AxiosError} If creation fails
 */
export const createDecision = async (payload: CreateDecisionPayload) => {
  const response = await client.post('/security/decisions', payload)
  return response.data
}

// WAF Ruleset types
/**
 * WAF security ruleset configuration. The numeric database id is never sent to the client; `uuid` identifies
 * a rule set in every route that addresses one.
 */
export interface SecurityRuleSet {
  uuid: string
  name: string
  source_url: string
  mode: string
  last_updated: string
  content: string
}

/** Response containing WAF rulesets. */
export interface RuleSetsResponse {
  rulesets: SecurityRuleSet[]
}

/** Payload for creating/updating a WAF ruleset. */
export interface UpsertRuleSetPayload {
  name: string
  content?: string
  source_url?: string
  mode?: 'blocking' | 'detection'
}

/**
 * Gets all WAF rulesets.
 * @returns Promise resolving to RuleSetsResponse
 * @throws {AxiosError} If the request fails
 */
export const getRuleSets = async (): Promise<RuleSetsResponse> => {
  const response = await client.get<RuleSetsResponse>('/security/rulesets')
  return response.data
}

/**
 * Creates or updates a WAF ruleset.
 * @param payload - Ruleset configuration
 * @returns Promise resolving to the upserted ruleset
 * @throws {AxiosError} If upsert fails
 */
export const upsertRuleSet = async (payload: UpsertRuleSetPayload) => {
  const response = await client.post('/security/rulesets', payload)
  return response.data
}

/**
 * Deletes a WAF ruleset. The server refuses (409 `waf_ruleset_in_use`) the rule set the WAF uses as its source.
 * @param uuid - The ruleset UUID to delete
 * @returns Promise resolving to delete result
 * @throws {AxiosError} If deletion fails or ruleset not found
 */
export const deleteRuleSet = async (uuid: string) => {
  const response = await client.delete(`/security/rulesets/${encodeURIComponent(uuid)}`)
  return response.data
}
