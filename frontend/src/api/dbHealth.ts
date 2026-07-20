import client from './client'

/**
 * Response of `GET /api/v1/health/db` (plan §3.9/§3.10) — surfaces the
 * result of a dedicated-connection `PRAGMA quick_check` integrity scan, so
 * the frontend can warn admins when scheduled/manual backups are failing
 * because the live database is corrupted, rather than leaving that only in
 * server logs.
 */
export interface DBHealthResponse {
  status: 'healthy' | 'corrupted'
  integrity_ok: boolean
  integrity_result: string
  wal_mode: boolean
  journal_mode: string
  last_backup: string | null
  checked_at: string
}

/**
 * Fetches the current SQLite database integrity/health status. Public,
 * unauthenticated endpoint (plan §3.9/§8 — matches `/api/v1/health`'s
 * existing monitoring-endpoint convention).
 * @throws {AxiosError} If the request fails
 */
export const getDbHealth = async (): Promise<DBHealthResponse> => {
  const response = await client.get<DBHealthResponse>('/health/db')
  return response.data
}
