import client from './client'

import type { AccessList } from './accessLists'

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
  } & SecurityEnforcement
  /** Whether forward target names are checked against the live resolver. Omitted by older backends. */
  upstream_name_checks?: { available: boolean }
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
  /** Ignored: CrowdSec readers always use the local engine address. */
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
  /** Omitted for standard users; `has_admin_whitelist` tells them whether one is set. */
  admin_whitelist?: string
  has_admin_whitelist?: boolean
  /** Omitted for standard users; `has_rate_limit_bypass_list` tells them whether one is set. */
  has_rate_limit_bypass_list?: boolean
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
  /** Omitted for standard users. */
  source_url?: string
  mode: string
  last_updated: string
  /** Omitted for standard users; `has_content` and `content_line_count` describe it instead. */
  content?: string
  has_content?: boolean
  /** Non-blank lines in the rule content. Sent to standard users in place of `content`. */
  content_line_count?: number
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

/** Where the proxy's GeoIP database comes from: the image, the data folder, a server-set path, or nothing. */
export type GeoIPSource = 'bundled' | 'data' | 'custom' | 'none'

/** Why a database cannot be replaced from the UI; empty when it can. */
export type GeoIPReplaceBlockedReason = '' | 'custom_path_set' | 'data_folder_read_only'

/** Response of `GET /security/geoip/status`. `reason` is present only while the database is unavailable. */
export interface GeoIPStatus {
  available: boolean
  loaded: boolean
  db_path: string
  message: string
  reason?: string
  source: GeoIPSource
  database_type: string
  build_date: string | null
  size_bytes: number | null
  loaded_at: string | null
  included_build_date: string | null
  data_copy_present: boolean
  can_replace: boolean
  replace_blocked_reason: GeoIPReplaceBlockedReason
  /** Country lists the proxy is not enforcing because no database is usable. */
  unenforced_lists_total: number
}

/** Response of the replace and remove routes: the status after the change. */
export interface GeoIPStatusChange {
  status: GeoIPStatus
  /** Set by the replace route when a data-folder copy was overwritten. */
  replaced_previous?: boolean
}

/** Response of `POST /security/geoip/lookup`. `country_code` is empty when `found` is false. */
export interface GeoIPLookupResult {
  ip_address: string
  country_code: string
  found: boolean
  message?: string
}

/**
 * Gets the GeoIP database status for the Security page card (admin).
 * @returns Promise resolving to GeoIPStatus
 * @throws {AxiosError} If the request fails
 */
export const getGeoIPStatus = async (): Promise<GeoIPStatus> => {
  const response = await client.get<GeoIPStatus>('/security/geoip/status')
  return response.data
}

/**
 * Reloads the GeoIP database from disk and applies the proxy configuration.
 * @returns Promise resolving to the server's reload message
 * @throws {AxiosError} If the reload fails; `error_code` names the cause
 */
export const reloadGeoIP = async (): Promise<unknown> => {
  const response = await client.post('/security/geoip/reload')
  return response.data
}

/**
 * Looks up the country of an IP address with the loaded database.
 * @param ipAddress - IPv4 or IPv6 address to look up
 * @returns Promise resolving to GeoIPLookupResult
 * @throws {AxiosError} If the request fails (400 for an invalid address, 503 when no database is loaded)
 */
export const lookupGeoIP = async (ipAddress: string): Promise<GeoIPLookupResult> => {
  const response = await client.post<GeoIPLookupResult>('/security/geoip/lookup', { ip_address: ipAddress })
  return response.data
}

/**
 * Replaces the data-folder copy of the GeoIP database with an uploaded file.
 * @param file - The uploaded database; sent as the multipart part `file`
 * @returns Promise resolving to the status after the replacement
 * @throws {AxiosError} If the upload is refused; `error_code` names the cause
 */
export const replaceGeoIPDatabase = async (file: File): Promise<GeoIPStatusChange> => {
  const body = new FormData()
  body.append('file', file)
  const response = await client.post<GeoIPStatusChange>('/security/geoip/database', body)
  return response.data
}

/**
 * Removes the data-folder copy so Charon uses the database included with the image.
 * @returns Promise resolving to the status after the removal
 * @throws {AxiosError} If the removal fails; 404 when there is no data-folder copy
 */
export const removeGeoIPDatabase = async (): Promise<GeoIPStatusChange> => {
  const response = await client.delete<GeoIPStatusChange>('/security/geoip/database')
  return response.data
}

/** Where the proxy layer's trusted list comes from: the saved setting, the environment, or the loopback default. */
export type TrustedProxySource = 'setting' | 'environment' | 'default'

/**
 * Proxy-layer trust list (`/security/trusted-proxies`). Administrators receive every field. Other users receive
 * `has_proxies` in place of the list fields, so `proxies` and the other list fields are absent for them.
 */
export interface TrustedProxiesResponse {
  source: TrustedProxySource
  /** Loopback addresses that are always trusted. */
  always_trusted: string[]
  /** Administrators only: the list in force, canonical CIDR text. */
  proxies?: string[]
  /** Administrators only: environment entries ignored because they are invalid or too broad. */
  dropped_entries?: string[]
  /** Administrators only: the list set by CHARON_TRUSTED_PROXIES for Charon's own port (read-only). */
  api_layer?: string[]
  /** Administrators only: passive notes about entries that are accepted but wide. */
  warnings?: string[]
  /** Non-administrators only: whether any proxy is configured. */
  has_proxies?: boolean
}

/**
 * Gets the proxy-layer trust list in force.
 * @returns Promise resolving to TrustedProxiesResponse
 * @throws {AxiosError} If the request fails
 */
export const getTrustedProxies = async (): Promise<TrustedProxiesResponse> => {
  const response = await client.get<TrustedProxiesResponse>('/security/trusted-proxies')
  return response.data
}

/**
 * Replaces the saved trusted-proxy list (administrators only). An empty list trusts loopback only.
 * @param proxies - Entries as typed; the server validates and canonicalises them
 * @returns Promise resolving to the list now in force
 * @throws {AxiosError} If the entries are refused (400) or the proxy cannot be reloaded (500)
 */
export const saveTrustedProxies = async (proxies: string[]): Promise<TrustedProxiesResponse> => {
  const response = await client.put<TrustedProxiesResponse>('/security/trusted-proxies', { proxies })
  return response.data
}

/**
 * Clears the saved trusted-proxy list so the environment list or the loopback default applies again
 * (administrators only).
 * @returns Promise resolving to the list now in force
 * @throws {AxiosError} If the proxy cannot be reloaded
 */
export const resetTrustedProxies = async (): Promise<TrustedProxiesResponse> => {
  const response = await client.delete<TrustedProxiesResponse>('/security/trusted-proxies')
  return response.data
}

/** One access list chosen to protect Charon's own interface (a row of the management set). */
export type ManagementAccessList = Pick<AccessList, 'uuid' | 'name' | 'type' | 'enabled'>

/**
 * Access lists that protect Charon's own interface (`/security/management-access`). `lists` is the chosen set;
 * `enforced` is false while the set is not applied (access control off, or no chosen list enabled).
 */
export interface ManagementAccessResponse {
  lists: ManagementAccessList[]
  enforced: boolean
}

/**
 * Gets the access lists chosen for Charon's own interface.
 * @returns Promise resolving to ManagementAccessResponse
 * @throws {AxiosError} If the request fails
 */
export const getManagementAccess = async (): Promise<ManagementAccessResponse> => {
  const response = await client.get<ManagementAccessResponse>('/security/management-access')
  return response.data
}

/**
 * Replaces the set of access lists that protect Charon's own interface (administrators only).
 * @param listUuids - UUIDs of the chosen lists; an empty array means not restricted
 * @returns Promise resolving to the stored set
 * @throws {AxiosError} If the choice is refused (400, or 403 when it would lock the caller out)
 */
export const saveManagementAccess = async (listUuids: string[]): Promise<ManagementAccessResponse> => {
  const response = await client.put<ManagementAccessResponse>('/security/management-access', { list_uuids: listUuids })
  return response.data
}
