import client from './client'

/** A single remote copy of a backup, reported inline on `BackupFile` (spec §3.3.1). */
export interface BackupRemoteCopyEntry {
  target_uuid: string
  target_name: string
  status: string
  uploaded_at?: string
}

/** Represents a backup file stored on the server. */
export interface BackupFile {
  filename: string
  size: number
  time: string
  /** Additive v2 fields (spec §3.3.1) — optional so v1-only consumers/tests keep working. */
  uuid?: string
  type?: 'manual' | 'scheduled' | 'pre_restore' | 'uploaded'
  encrypted?: boolean
  format_version?: number
  sha256?: string
  status?: string
  app_version?: string
  remote_copies?: BackupRemoteCopyEntry[]
}

/**
 * Fetches all available backup files.
 * @returns Promise resolving to array of BackupFile objects
 * @throws {AxiosError} If the request fails
 */
export const getBackups = async (): Promise<BackupFile[]> => {
  const response = await client.get<BackupFile[]>('/backups')
  return response.data
}

/** Optional body for POST /backups (spec §3.3.1) — both fields optional. */
export interface CreateBackupOptions {
  encrypt?: boolean
  passphrase?: string
}

/** Response of POST /backups (spec §3.3.1). */
export interface CreateBackupResponse {
  filename: string
  uuid?: string
  message?: string
}

/**
 * Creates a new backup of the current configuration.
 * @param options - Optional encryption request (`encrypt`/`passphrase`)
 * @returns Promise resolving to object containing the new backup filename
 * @throws {AxiosError} If backup creation fails
 */
export const createBackup = async (options?: CreateBackupOptions): Promise<CreateBackupResponse> => {
  const response = await (options
    ? client.post<CreateBackupResponse>('/backups', options)
    : client.post<CreateBackupResponse>('/backups'))
  return response.data
}

/** Response of POST /backups/:filename/restore (spec §3.3.1). */
export interface RestoreResult {
  message: string
  restart_required: boolean
  database_swap_pending: boolean
  live_rehydrate_applied: boolean
  caddy_reloaded: boolean
  pre_restore_backup?: string
  legacy_format: boolean
}

/**
 * Restores configuration from a backup file.
 * @param filename - The name of the backup file to restore
 * @param passphrase - Required only when `filename` ends in `.age`
 * @throws {AxiosError} If restoration fails or file not found
 */
export const restoreBackup = async (filename: string, passphrase?: string): Promise<RestoreResult> => {
  const response = await (passphrase
    ? client.post<RestoreResult>(`/backups/${filename}/restore`, { passphrase })
    : client.post<RestoreResult>(`/backups/${filename}/restore`))
  return response.data
}

/**
 * Deletes a backup file.
 * @param filename - The name of the backup file to delete
 * @throws {AxiosError} If deletion fails or file not found
 */
export const deleteBackup = async (filename: string): Promise<void> => {
  await client.delete(`/backups/${filename}`)
}

/** Response of POST /backups/upload and POST /backups/:filename/validate (spec §3.3.2). */
export interface UploadBackupResponse {
  filename: string
  uuid?: string
  legacy_format: boolean
  encryption_key_required?: boolean
  message: string
}

/**
 * Uploads a backup archive (.zip, .zip.age, or raw .db) for validation and
 * storage. The server discards the client-supplied filename and generates
 * its own (spec §3.3.2, §3.9).
 * @param file - The backup file to upload
 * @param passphrase - Optional passphrase to validate an encrypted upload immediately
 * @throws {AxiosError} If the upload or validation fails
 */
export const uploadBackup = async (file: File, passphrase?: string): Promise<UploadBackupResponse> => {
  const formData = new FormData()
  formData.append('file', file)
  if (passphrase) {
    formData.append('passphrase', passphrase)
  }
  const response = await client.post<UploadBackupResponse>('/backups/upload', formData, {
    headers: { 'Content-Type': 'multipart/form-data' },
  })
  return response.data
}

/** Response of POST /backups/:filename/validate (spec §3.3.2). */
export interface ValidateBackupResponse {
  valid: boolean
  format_version: number
  legacy_format: boolean
  app_version?: string
  created_at?: string
  database_integrity: string
  encryption_key_required: boolean
  warnings?: string[]
}

/**
 * Dry-run validates a backup archive without restoring it.
 * @param filename - The name of the backup file to validate
 * @param passphrase - Required only when `filename` ends in `.age`
 * @throws {AxiosError} If validation fails
 */
export const validateBackup = async (filename: string, passphrase?: string): Promise<ValidateBackupResponse> => {
  const response = await (passphrase
    ? client.post<ValidateBackupResponse>(`/backups/${filename}/validate`, { passphrase })
    : client.post<ValidateBackupResponse>(`/backups/${filename}/validate`))
  return response.data
}

/** GET/PUT response shape for /backups/settings (spec §3.3.2). */
export interface BackupSettings {
  schedule_enabled: boolean
  schedule_cron: string
  retention_count: number
  remote_retention_count: number
  encryption_enabled: boolean
  encryption_passphrase_set: boolean
}

/** Partial-update request body for PUT /backups/settings (spec §3.3.2). */
export interface UpdateBackupSettingsPayload {
  schedule_enabled?: boolean
  schedule_cron?: string
  retention_count?: number
  remote_retention_count?: number
  encryption_enabled?: boolean
  /** Write-only; never echoed back. Empty string clears the stored passphrase. */
  encryption_passphrase?: string
}

/**
 * Fetches the current backup schedule/retention/encryption settings.
 * @throws {AxiosError} If the request fails
 */
export const getBackupSettings = async (): Promise<BackupSettings> => {
  const response = await client.get<BackupSettings>('/backups/settings')
  return response.data
}

/**
 * Updates the backup schedule/retention/encryption settings (partial update).
 * @param payload - Fields to update; omitted fields are left unchanged
 * @throws {AxiosError} If validation fails (invalid cron, retention count, etc.)
 */
export const updateBackupSettings = async (payload: UpdateBackupSettingsPayload): Promise<BackupSettings> => {
  const response = await client.put<BackupSettings>('/backups/settings', payload)
  return response.data
}

/** Non-secret remote-target configuration (spec §3.3.2) — fields not relevant to `type` are omitted. */
export interface RemoteTargetConfig {
  // s3
  endpoint?: string
  region?: string
  bucket?: string
  path_prefix?: string
  use_ssl?: boolean
  force_path_style?: boolean
  // sftp
  host?: string
  port?: number
  path?: string
  username?: string
  host_key_fingerprint?: string
}

/** Secret blob accepted by the remote-targets API — never returned by the server. */
export interface RemoteTargetSecrets {
  access_key_id?: string
  secret_access_key?: string
  password?: string
  private_key_pem?: string
  passphrase?: string
}

/** Remote storage target as returned by the API — `secrets` are never included (spec §3.3.2). */
export interface RemoteTarget {
  uuid: string
  name: string
  type: 's3' | 'sftp'
  enabled: boolean
  config: RemoteTargetConfig
  secrets_set: boolean
  last_test_at: string | null
  last_test_status: 'ok' | 'failed' | 'never'
  last_error: string
  created_at: string
  updated_at: string
}

/** Create/update request body for remote targets (spec §3.3.2). */
export interface RemoteTargetPayload {
  name: string
  type: 's3' | 'sftp'
  enabled?: boolean
  config: RemoteTargetConfig
  secrets?: RemoteTargetSecrets
}

/**
 * Fetches all configured remote storage targets. Secrets are never included.
 * @throws {AxiosError} If the request fails
 */
export const getRemoteTargets = async (): Promise<RemoteTarget[]> => {
  const response = await client.get<RemoteTarget[]>('/backups/remote-targets')
  return response.data
}

/**
 * Creates a new remote storage target.
 * @param payload - Target name/type/config/secrets
 * @throws {AxiosError} If creation fails (validation, SSRF rejection, etc.)
 */
export const createRemoteTarget = async (payload: RemoteTargetPayload): Promise<RemoteTarget> => {
  const response = await client.post<RemoteTarget>('/backups/remote-targets', payload)
  return response.data
}

/**
 * Updates an existing remote storage target. Omitted secret fields keep the
 * existing stored value (spec §3.3.2).
 * @param uuid - Target UUID
 * @param payload - Partial update
 * @throws {AxiosError} If the update fails
 */
export const updateRemoteTarget = async (
  uuid: string,
  payload: Partial<RemoteTargetPayload>
): Promise<RemoteTarget> => {
  const response = await client.put<RemoteTarget>(`/backups/remote-targets/${uuid}`, payload)
  return response.data
}

/**
 * Deletes a remote storage target.
 * @param uuid - Target UUID
 * @throws {AxiosError} If deletion fails
 */
export const deleteRemoteTarget = async (uuid: string): Promise<void> => {
  await client.delete(`/backups/remote-targets/${uuid}`)
}

/** Response of POST /backups/remote-targets/:uuid/test (spec §3.3.2, §3.7). */
export interface TestRemoteTargetResponse {
  success: boolean
  message: string
  latency_ms?: number
  /** Present only during SFTP host-key discovery (no fingerprint pinned yet). */
  discovered_fingerprint?: string
}

/**
 * Tests connectivity/auth for a remote storage target. For SFTP targets with
 * no pinned host key yet, this runs the discovery flow and returns a
 * `discovered_fingerprint` without ever authenticating (spec §3.7).
 * @param uuid - Target UUID
 * @throws {AxiosError} If the request itself fails (a failed connection test
 *   is still reported as a 200 with `success: false` by some paths, or as a
 *   4xx/5xx `error` body by others — callers should handle both)
 */
export const testRemoteTarget = async (uuid: string): Promise<TestRemoteTargetResponse> => {
  const response = await client.post<TestRemoteTargetResponse>(`/backups/remote-targets/${uuid}/test`)
  return response.data
}
