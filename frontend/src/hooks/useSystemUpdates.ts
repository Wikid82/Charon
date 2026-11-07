import { useQuery } from '@tanstack/react-query'

import { checkUpdates, type UpdateInfo } from '../api/system'

/** Shared cache key so every consumer reads the same update-check result. */
export const SYSTEM_UPDATES_QUERY_KEY = ['system-updates'] as const

export interface UseSystemUpdatesOptions {
  /** Automatic fetching is on by default; pass `false` for manual-only checks. */
  enabled?: boolean
}

/**
 * Reads the cached application update status (`GET /system/updates`).
 * Consumers that need manual control pass `{ enabled: false }` and call the returned `refetch`.
 */
export function useSystemUpdates({ enabled = true }: UseSystemUpdatesOptions = {}) {
  return useQuery<UpdateInfo>({
    queryKey: SYSTEM_UPDATES_QUERY_KEY,
    queryFn: checkUpdates,
    staleTime: 1000 * 60 * 60, // 1 hour
    enabled,
  })
}
