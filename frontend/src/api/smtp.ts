import client from './client'

/** SMTP server configuration. */
export interface SMTPConfig {
  host: string
  port: number
  username: string
  password: string
  from_address: string
  encryption: 'none' | 'ssl' | 'starttls'
  configured: boolean
}

/** Request payload for SMTP configuration. */
export interface SMTPConfigRequest {
  host: string
  port: number
  username: string
  password: string
  from_address: string
  encryption: 'none' | 'ssl' | 'starttls'
}

/** Request payload for sending a test email. */
export interface TestEmailRequest {
  to: string
}

/** Result of an SMTP test operation. */
export interface SMTPTestResult {
  success: boolean
  message?: string
  error?: string
}

/**
 * Fetches the current SMTP configuration.
 * @returns Promise resolving to SMTPConfig
 * @throws {AxiosError} If the request fails
 */
export const getSMTPConfig = async (): Promise<SMTPConfig> => {
  const response = await client.get<SMTPConfig>('/settings/smtp')
  return response.data
}

/**
 * Updates the SMTP configuration.
 * @param config - SMTPConfigRequest with new settings
 * @returns Promise resolving to success message
 * @throws {AxiosError} If update fails
 */
export const updateSMTPConfig = async (config: SMTPConfigRequest): Promise<{ message: string }> => {
  const response = await client.post<{ message: string }>('/settings/smtp', config)
  return response.data
}

/**
 * Tests the SMTP connection with current settings.
 * @returns Promise resolving to SMTPTestResult
 * @throws {AxiosError} If test request fails
 */
export const testSMTPConnection = async (): Promise<SMTPTestResult> => {
  const response = await client.post<SMTPTestResult>('/settings/smtp/test')
  return response.data
}

/**
 * Sends a test email to verify SMTP configuration.
 * @param request - TestEmailRequest with recipient address
 * @returns Promise resolving to SMTPTestResult
 * @throws {AxiosError} If sending fails
 */
export const sendTestEmail = async (request: TestEmailRequest): Promise<SMTPTestResult> => {
  const response = await client.post<SMTPTestResult>('/settings/smtp/test-email', request)
  return response.data
}
