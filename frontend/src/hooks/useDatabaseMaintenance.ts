import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'

import {
  cancelOptimizeOnRestart,
  getDatabaseStatus,
  getMaintenanceStatus,
  requestOptimizeOnRestart,
} from '../api/databaseMaintenance'

export const DATABASE_STATUS_QUERY_KEY = ['system-database']
export const MAINTENANCE_STATUS_QUERY_KEY = ['maintenance-status']

/** Maintenance view poll interval; matches the server-rendered page (plan 3.6). */
export const MAINTENANCE_POLL_MS = 5000

/**
 * Database card data. The notice is computed fresh server-side on every call,
 * so a 30 s staleTime is enough. No retry: a 403 for non-admin users is final.
 */
export function useDatabaseStatus() {
  return useQuery({
    queryKey: DATABASE_STATUS_QUERY_KEY,
    queryFn: getDatabaseStatus,
    staleTime: 30_000,
    retry: false,
  })
}

/** Sets the "reclaim space on next restart" request, then refreshes the card. */
export function useRequestOptimize() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: requestOptimizeOnRestart,
    // A 409 means the server's view changed; refresh either way.
    onSettled: () => queryClient.invalidateQueries({ queryKey: DATABASE_STATUS_QUERY_KEY }),
  })
}

/** Withdraws the request (undo), then refreshes the card. */
export function useCancelOptimize() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: cancelOptimizeOnRestart,
    onSettled: () => queryClient.invalidateQueries({ queryKey: DATABASE_STATUS_QUERY_KEY }),
  })
}

/** Polls the maintenance gate while the maintenance view is shown. Never retries on its own. */
export function useMaintenanceStatus(enabled: boolean) {
  return useQuery({
    queryKey: MAINTENANCE_STATUS_QUERY_KEY,
    queryFn: getMaintenanceStatus,
    enabled,
    refetchInterval: MAINTENANCE_POLL_MS,
    staleTime: 0,
    gcTime: 0,
    retry: false,
  })
}
