import client from './client'

export interface SecurityStatus {
  cerberus?: { enabled: boolean }
  crowdsec: {
    mode: 'disabled' | 'local' | 'external'
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
  waf_mode?: string
  rate_limit_enable?: boolean
  rate_limit_burst?: number
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
