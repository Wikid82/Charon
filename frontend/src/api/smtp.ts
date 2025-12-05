import client from './client'

export interface SMTPConfig {
  host: string
  port: number
  username: string
  password: string
  from_address: string
  encryption: 'none' | 'ssl' | 'starttls'
  configured: boolean
}

export interface SMTPConfigRequest {
  host: string
  port: number
  username: string
  password: string
  from_address: string
  encryption: 'none' | 'ssl' | 'starttls'
}

export interface TestEmailRequest {
  to: string
}

export interface SMTPTestResult {
  success: boolean
  message?: string
  error?: string
}

export const getSMTPConfig = async (): Promise<SMTPConfig> => {
  const response = await client.get<SMTPConfig>('/settings/smtp')
  return response.data
}

export const updateSMTPConfig = async (config: SMTPConfigRequest): Promise<{ message: string }> => {
  const response = await client.post<{ message: string }>('/settings/smtp', config)
  return response.data
}

export const testSMTPConnection = async (): Promise<SMTPTestResult> => {
  const response = await client.post<SMTPTestResult>('/settings/smtp/test')
  return response.data
}

export const sendTestEmail = async (request: TestEmailRequest): Promise<SMTPTestResult> => {
  const response = await client.post<SMTPTestResult>('/settings/smtp/test-email', request)
  return response.data
}
