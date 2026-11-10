import { useQuery } from '@tanstack/react-query';

import { getActiveAlerts, type ActiveAlert } from '../api/system';

/** Shared cache key so the bell and any future consumer read the same alert list. */
export const ACTIVE_ALERTS_QUERY_KEY = ['active-alerts'] as const;

/** Alerts are recomputed on the server, so the bell re-reads them every 30 seconds. */
export const ACTIVE_ALERTS_POLL_INTERVAL_MS = 30_000;

/** Reads the alerts that currently need attention (`GET /notifications/alerts`). */
export function useActiveAlerts() {
  return useQuery<ActiveAlert[]>({
    queryKey: ACTIVE_ALERTS_QUERY_KEY,
    queryFn: getActiveAlerts,
    refetchInterval: ACTIVE_ALERTS_POLL_INTERVAL_MS,
    refetchOnWindowFocus: true,
  });
}
