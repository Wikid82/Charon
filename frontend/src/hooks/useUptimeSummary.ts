import { useQuery, type UseQueryResult } from '@tanstack/react-query';

import { getMonitorsSummary, getUptimeHealth, type MonitorSummary, type UptimeHealth } from '../api/uptime';

/**
 * Fetches the batch monitor summary in a single request and refreshes it every
 * 30s (the server-side cache TTL). This is the sole data source for the Uptime
 * page list view — it replaced the per-card `['uptimeHistory', id]` N+1. (§3.5.7)
 */
export const useUptimeSummary = (): UseQueryResult<MonitorSummary[]> =>
  useQuery({
    queryKey: ['uptimeSummary'],
    queryFn: () => getMonitorsSummary(30),
    refetchInterval: 30000,
  });

/**
 * Fetches the uptime ingester / worker-pool health signal, polled every 30s.
 * Surfaced on the admin "Uptime Monitoring" settings card. (§3.5.5)
 */
export const useUptimeHealth = (): UseQueryResult<UptimeHealth> =>
  useQuery({
    queryKey: ['uptimeHealth'],
    queryFn: getUptimeHealth,
    refetchInterval: 30000,
  });
