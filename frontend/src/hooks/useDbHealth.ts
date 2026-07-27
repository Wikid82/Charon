import { useQuery } from '@tanstack/react-query'

import { getDbHealth, type DBHealthResponse } from '../api/dbHealth'

/** Query key for the DB integrity/health status (plan §3.10). */
export const DB_HEALTH_QUERY_KEY = ['db-health']

/**
 * Fetches the current SQLite database integrity status, so the Backups page
 * can warn admins when backups are failing due to a corrupted database
 * (plan §2.5/§3.10) rather than leaving that only in server logs. A 60s
 * `staleTime` avoids re-checking on every render — corruption status
 * changes rarely and isn't time-critical to reflect instantly.
 */
export function useDbHealth() {
  return useQuery({
    queryKey: DB_HEALTH_QUERY_KEY,
    queryFn: getDbHealth,
    staleTime: 60_000,
  })
}

export type { DBHealthResponse }
