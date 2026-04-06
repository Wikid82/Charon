import { useQuery } from '@tanstack/react-query'

import {
  getDashboardSummary,
  getDashboardTimeline,
  getDashboardTopIPs,
  getDashboardScenarios,
  getAlerts,
  type TimeRange,
} from '../api/crowdsecDashboard'

const STALE_TIME = 30_000

export function useDashboardSummary(range: TimeRange) {
  return useQuery({
    queryKey: ['crowdsec-dashboard', 'summary', range],
    queryFn: () => getDashboardSummary(range),
    staleTime: STALE_TIME,
  })
}

export function useDashboardTimeline(range: TimeRange) {
  return useQuery({
    queryKey: ['crowdsec-dashboard', 'timeline', range],
    queryFn: () => getDashboardTimeline(range),
    staleTime: STALE_TIME,
  })
}

export function useDashboardTopIPs(range: TimeRange, limit = 10) {
  return useQuery({
    queryKey: ['crowdsec-dashboard', 'top-ips', range, limit],
    queryFn: () => getDashboardTopIPs(range, limit),
    staleTime: STALE_TIME,
  })
}

export function useDashboardScenarios(range: TimeRange) {
  return useQuery({
    queryKey: ['crowdsec-dashboard', 'scenarios', range],
    queryFn: () => getDashboardScenarios(range),
    staleTime: STALE_TIME,
  })
}

export function useAlerts(params: {
  range?: TimeRange
  scenario?: string
  limit?: number
  offset?: number
}) {
  return useQuery({
    queryKey: ['crowdsec-dashboard', 'alerts', params],
    queryFn: () => getAlerts(params),
    staleTime: STALE_TIME,
  })
}
