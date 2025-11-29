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
