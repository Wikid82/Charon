import { useQuery } from '@tanstack/react-query'

import { statusCrowdsec } from '../api/crowdsec'

export const CROWDSEC_STATUS_QUERY_KEY = ['crowdsec-status'] as const

export interface CrowdsecProcessStatus {
  running: boolean
  pid?: number
}

/**
 * CrowdSec process status for the Security dashboard.
 * Errors map to `null` (never surfaced), so callers fall back to the configured state.
 * Single attempt and no focus refetch, matching the previous mount-time fetch.
 */
export function useCrowdsecStatus() {
  return useQuery<CrowdsecProcessStatus | null>({
    queryKey: CROWDSEC_STATUS_QUERY_KEY,
    queryFn: async () => {
      try {
        return await statusCrowdsec()
      } catch {
        return null
      }
    },
    retry: false,
    refetchOnWindowFocus: false,
  })
}
