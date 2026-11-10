import { useMutation, useQuery, useQueryClient, type QueryClient } from '@tanstack/react-query'

import {
  getGeoIPStatus,
  lookupGeoIP,
  reloadGeoIP,
  removeGeoIPDatabase,
  replaceGeoIPDatabase,
} from '../api/security'
import { getMyIP } from '../api/system'
import { ACTIVE_ALERTS_QUERY_KEY } from './useActiveAlerts'
import { SECURITY_STATUS_QUERY_KEY } from './useSecurity'


/** Cache key of the GeoIP database status read by the Security page card. */
export const GEOIP_STATUS_QUERY_KEY = ['geoipStatus'] as const

/** The status changes only through the card's own actions, so it is not polled. */
const GEOIP_STATUS_STALE_MS = 60_000

/** Reads the GeoIP database status (admin only: the caller decides whether to mount the reader). */
export function useGeoIPStatus() {
  return useQuery({
    queryKey: GEOIP_STATUS_QUERY_KEY,
    queryFn: getGeoIPStatus,
    staleTime: GEOIP_STATUS_STALE_MS,
  })
}

/**
 * Refreshes everything a database change can alter: the card status, the security status (country lists and modules)
 * and the active alerts (the missing-database alert). Runs after success and failure, because a failed reload still
 * applies the proxy configuration.
 */
function refreshAfterGeoIPChange(queryClient: QueryClient) {
  return Promise.all([
    queryClient.invalidateQueries({ queryKey: GEOIP_STATUS_QUERY_KEY }),
    queryClient.invalidateQueries({ queryKey: SECURITY_STATUS_QUERY_KEY }),
    queryClient.invalidateQueries({ queryKey: ACTIVE_ALERTS_QUERY_KEY }),
  ])
}

/** Reloads the database from disk; the proxy configuration is applied by the server. */
export function useReloadGeoIP() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: reloadGeoIP,
    onSettled: () => refreshAfterGeoIPChange(queryClient),
  })
}

/** Uploads a database into the data folder, replacing the copy there. */
export function useReplaceGeoIPDatabase() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (file: File) => replaceGeoIPDatabase(file),
    onSettled: () => refreshAfterGeoIPChange(queryClient),
  })
}

/** Removes the data-folder copy so the included database is used again. */
export function useRemoveGeoIPDatabase() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: removeGeoIPDatabase,
    onSettled: () => refreshAfterGeoIPChange(queryClient),
  })
}

/** Looks up the country of one address. Read-only, so it does not refresh any cache. */
export function useGeoIPLookup() {
  return useMutation({
    mutationFn: (ipAddress: string) => lookupGeoIP(ipAddress),
  })
}

/** Reads the address Charon sees for this browser, to fill the lookup field. */
export function useMyIPAddress() {
  return useMutation({
    mutationFn: () => getMyIP(),
  })
}
