import client from './client'

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
  }
  rate_limit: {
    mode?: 'disabled' | 'enabled'
    enabled: boolean
  }
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

/** Security configuration payload. */
export interface SecurityConfigPayload {
  name?: string
  enabled?: boolean
  admin_whitelist?: string
  crowdsec_mode?: string
  crowdsec_api_url?: string
  waf_mode?: string
  waf_rules_source?: string
  waf_learning?: boolean
  rate_limit_enable?: boolean
  rate_limit_burst?: number
  rate_limit_requests?: number
  rate_limit_window_sec?: number
}

/**
 * Gets the current security configuration.
 * @returns Promise resolving to the security configuration
 * @throws {AxiosError} If the request fails
 */
export const getSecurityConfig = async () => {
  const response = await client.get('/security/config')
  return response.data
}

/**
 * Updates security configuration.
 * @param payload - SecurityConfigPayload with settings to update
 * @returns Promise resolving to the updated configuration
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
/** WAF security ruleset configuration. */
export interface SecurityRuleSet {
  id: number
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
  id?: number
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
 * Deletes a WAF ruleset.
 * @param id - The ruleset ID to delete
 * @returns Promise resolving to delete result
 * @throws {AxiosError} If deletion fails or ruleset not found
 */
export const deleteRuleSet = async (id: number) => {
  const response = await client.delete(`/security/rulesets/${id}`)
  return response.data
}
