import client from './client'

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

export const getSecurityStatus = async (): Promise<SecurityStatus> => {
  const response = await client.get<SecurityStatus>('/security/status')
  return response.data
}

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

export const getSecurityConfig = async () => {
  const response = await client.get('/security/config')
  return response.data
}

export const updateSecurityConfig = async (payload: SecurityConfigPayload) => {
  const response = await client.post('/security/config', payload)
  return response.data
}

export const generateBreakGlassToken = async () => {
  const response = await client.post('/security/breakglass/generate')
  return response.data
}

export const enableCerberus = async (payload?: any) => {
  const response = await client.post('/security/enable', payload || {})
  return response.data
}

export const disableCerberus = async (payload?: any) => {
  const response = await client.post('/security/disable', payload || {})
  return response.data
}

export const getDecisions = async (limit = 50) => {
  const response = await client.get(`/security/decisions?limit=${limit}`)
  return response.data
}

export const createDecision = async (payload: any) => {
  const response = await client.post('/security/decisions', payload)
  return response.data
}

// WAF Ruleset types
export interface SecurityRuleSet {
  id: number
  uuid: string
  name: string
  source_url: string
  mode: string
  last_updated: string
  content: string
}

export interface RuleSetsResponse {
  rulesets: SecurityRuleSet[]
}

export interface UpsertRuleSetPayload {
  id?: number
  name: string
  content?: string
  source_url?: string
  mode?: 'blocking' | 'detection'
}

export const getRuleSets = async (): Promise<RuleSetsResponse> => {
  const response = await client.get<RuleSetsResponse>('/security/rulesets')
  return response.data
}

export const upsertRuleSet = async (payload: UpsertRuleSetPayload) => {
  const response = await client.post('/security/rulesets', payload)
  return response.data
}

export const deleteRuleSet = async (id: number) => {
  const response = await client.delete(`/security/rulesets/${id}`)
  return response.data
}
