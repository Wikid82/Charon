import client from './client'

/** Usage of one configured encryption key slot (CHARON_ENCRYPTION_KEY, _NEXT, _V<n>) */
export interface KeyUsage {
  slot: string
  /** Zero when the slot could not be loaded */
  version: number
  /** First 8 hex chars of the key id; display only. Empty when not loaded. */
  key_id: string
  in_use: number
  /** True when the key is not the current key and no stored secret uses it */
  removable: boolean
  loaded: boolean
}

/** How many stored secrets of one store are protected by each key version */
export interface StoreUsage {
  store: string
  total: number
  /** Keyed by version as a string (JSON object keys) */
  by_version: Record<string, number>
  unreadable: number
}

/** Rotation status for key management */
export interface RotationStatus {
  current_version: number
  next_key_configured: boolean
  next_version: number
  legacy_key_count: number
  legacy_key_versions: number[]
  keys: KeyUsage[]
  stores: StoreUsage[]
  unreadable_total: number
  /** True when some stored secret is on a key other than the rotation target */
  rotation_needed: boolean
  /** DNS providers only; kept for existing scripts */
  providers_on_current_version: number
  /** DNS providers only; kept for existing scripts */
  providers_on_older_versions: number
  /** DNS providers only; kept for existing scripts */
  providers_by_version: Record<string, number>
}

/** Per-store outcome of a rotation run */
export interface StoreRotation {
  store: string
  total: number
  rotated: number
  already_current: number
  unreadable: number
}

/** Result of a key rotation operation */
export interface RotationResult {
  target_version: number
  stores: StoreRotation[]
  total_records: number
  rotated_count: number
  already_current_count: number
  unreadable_count: number
  new_key_version: number
  duration: string
  started_at: string
  completed_at: string
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
 * Re-encrypts every stored secret to the rotation target key.
 * @returns Promise resolving to the per-store rotation result
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
