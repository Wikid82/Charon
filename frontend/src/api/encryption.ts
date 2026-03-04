import client from './client'

/** Rotation status for key management */
export interface RotationStatus {
  current_version: number
  next_key_configured: boolean
  legacy_key_count: number
  providers_on_current_version: number
  providers_on_older_versions: number
}

/** Result of a key rotation operation */
export interface RotationResult {
  total_providers: number
  success_count: number
  failure_count: number
  failed_providers?: number[]
  duration: string
  new_key_version: number
}

/** Audit log entry for key rotation history */
export interface RotationHistoryEntry {
  id: number
  uuid: string
  actor: string
  action: string
  event_category: string
  details: string
  created_at: string
}

/** Response for history endpoint */
interface RotationHistoryResponse {
  history: RotationHistoryEntry[]
  total: number
}

/** Validation result for key configuration */
export interface KeyValidationResult {
  valid: boolean
  message?: string
  errors?: string[]
  warnings?: string[]
}

/**
 * Fetches current encryption key status and rotation information.
 * @returns Promise resolving to rotation status
 * @throws {AxiosError} If the request fails
 */
export async function getEncryptionStatus(): Promise<RotationStatus> {
  const response = await client.get<RotationStatus>('/admin/encryption/status')
  return response.data
}

/**
 * Triggers rotation of all DNS provider credentials to a new encryption key.
 * @returns Promise resolving to rotation result
 * @throws {AxiosError} If rotation fails or request fails
 */
export async function rotateEncryptionKey(): Promise<RotationResult> {
  const response = await client.post<RotationResult>('/admin/encryption/rotate')
  return response.data
}

/**
 * Fetches key rotation audit history.
 * @returns Promise resolving to array of rotation history entries
 * @throws {AxiosError} If the request fails
 */
export async function getRotationHistory(): Promise<RotationHistoryEntry[]> {
  const response = await client.get<RotationHistoryResponse>('/admin/encryption/history')
  return response.data.history
}

/**
 * Validates the current key configuration.
 * @returns Promise resolving to validation result
 * @throws {AxiosError} If the request fails
 */
export async function validateKeyConfiguration(): Promise<KeyValidationResult> {
  const response = await client.post<KeyValidationResult>('/admin/encryption/validate')
  return response.data
}
