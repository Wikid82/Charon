import { useQuery } from '@tanstack/react-query';
import type { UseQueryResult } from '@tanstack/react-query';

import {
  getStatsSummary,
  getTopHosts,
  getStatusDistribution,
  getTrafficVolume,
  getCertExpiry,
  getStatsHealth,
  type StatsSummary,
  type HostStat,
  type StatusStat,
  type TrafficBucket,
  type CertExpiry,
  type StatsHealth,
  type StatsPeriod,
  type StatsBucket,
} from '../api/stats';

/**
 * Fetches aggregate request counts for the last 24h, 7d, and 30d.
 * When wsConnected is true, disables REST polling to avoid redundant calls
 * alongside the WebSocket.
 */
export function useStatsSummary(wsConnected?: boolean): UseQueryResult<StatsSummary> {
  return useQuery({
    queryKey: ['stats', 'summary'],
    queryFn: getStatsSummary,
    refetchInterval: wsConnected ? false : 30_000,
  });
}

/**
 * Fetches the top N hosts by request count for the given period.
 * Polls every 60 seconds.
 */
export function useTopHosts(period: StatsPeriod, limit = 10): UseQueryResult<HostStat[]> {
  return useQuery({
    queryKey: ['stats', 'top-hosts', period, limit],
    queryFn: () => getTopHosts(period, limit),
    refetchInterval: 60_000,
  });
}

/**
 * Fetches HTTP status code distribution for the given period.
 * Polls every 60 seconds.
 */
export function useStatusDistribution(period: StatsPeriod): UseQueryResult<StatusStat[]> {
  return useQuery({
    queryKey: ['stats', 'status-distribution', period],
    queryFn: () => getStatusDistribution(period),
    refetchInterval: 60_000,
  });
}

/**
 * Fetches traffic volume bucketed by time.
 * Polls every 30 seconds.
 */
export function useTrafficVolume(bucket: StatsBucket): UseQueryResult<TrafficBucket[]> {
  return useQuery({
    queryKey: ['stats', 'traffic-volume', bucket],
    queryFn: () => getTrafficVolume(bucket),
    refetchInterval: 30_000,
  });
}

/**
 * Fetches TLS certificate expiry information for hosts expiring within N days.
 * Polls every 5 minutes.
 */
export function useCertExpiry(withinDays = 30): UseQueryResult<CertExpiry[]> {
  return useQuery({
    queryKey: ['stats', 'cert-expiry', withinDays],
    queryFn: () => getCertExpiry(withinDays),
    refetchInterval: 5 * 60_000,
  });
}

/**
 * Fetches stats ingestion health metrics (dropped_count).
 * Polls every 30 seconds.
 */
export function useStatsHealth(): UseQueryResult<StatsHealth> {
  return useQuery({
    queryKey: ['stats', 'health'],
    queryFn: getStatsHealth,
    refetchInterval: 30_000,
  });
}
