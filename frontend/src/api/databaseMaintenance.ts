import client from './client'

/** Notice codes emitted by `GET /system/database` (GH #1422, plan section 3.7). */
export type DatabaseNoticeCode =
  | 'restart_to_optimize'
  | 'insufficient_disk'
  | 'disabled_by_env'
  | 'too_many_failures'
  | 'database_busy'

export interface DatabaseNotice {
  code: DatabaseNoticeCode
  severity: 'info' | 'warning'
  reclaimable_bytes: number
  required_bytes?: number
  available_bytes?: number
}

export interface DatabaseLastResult {
  at: string
  outcome: string
  reason: string
  bytes_before: number
  bytes_after: number
}

/** Response of `GET /api/v1/system/database` (admin only). */
export interface DatabaseStatus {
  size_bytes: number
  wal_bytes: number
  reclaimable_bytes: number
  auto_vacuum: 'none' | 'full' | 'incremental'
  env_mode: 'auto' | 'off'
  compact_requested: boolean
  can_request_optimize: boolean
  disk_free_bytes: number
  last_result: DatabaseLastResult | null
  notice: DatabaseNotice | null
}

export interface OptimizeRequestResult {
  requested: boolean
}

export type MaintenancePhase =
  | 'idle'
  | 'planned'
  | 'checking'
  | 'converting'
  | 'done'
  | 'skipped'
  | 'failed'

/** Response of the unauthenticated `GET /api/v1/maintenance/status`. */
export interface MaintenanceStatus {
  active: boolean
  phase: MaintenancePhase
  elapsed_seconds: number
}

/**
 * Fetches the database card data and the notice that currently applies.
 * @throws {AxiosError} If the request fails (403 for non-admin users)
 */
export const getDatabaseStatus = async (): Promise<DatabaseStatus> => {
  const response = await client.get<DatabaseStatus>('/system/database')
  return response.data
}

/**
 * Asks Charon to reclaim space at the next start. Idempotent: repeating it
 * while the request is already set still returns `{requested: true}`.
 * @throws {AxiosError} 409 with `code` `nothing_to_optimize` or `disabled_by_env`
 */
export const requestOptimizeOnRestart = async (): Promise<OptimizeRequestResult> => {
  const response = await client.post<OptimizeRequestResult>('/system/database/optimize-on-restart')
  return response.data
}

/** Withdraws the request made by {@link requestOptimizeOnRestart}. Idempotent. */
export const cancelOptimizeOnRestart = async (): Promise<OptimizeRequestResult> => {
  const response = await client.delete<OptimizeRequestResult>('/system/database/optimize-on-restart')
  return response.data
}

/** Polls the maintenance gate. Answered in every phase without touching the database. */
export const getMaintenanceStatus = async (): Promise<MaintenanceStatus> => {
  const response = await client.get<MaintenanceStatus>('/maintenance/status')
  return response.data
}
