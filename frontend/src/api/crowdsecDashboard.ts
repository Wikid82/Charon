import client from './client'

export type TimeRange = '1h' | '6h' | '24h' | '7d' | '30d'

export interface DashboardSummary {
  total_decisions: number
  active_decisions: number
  unique_ips: number
  top_scenario: string
  decisions_trend: number
  range: string
  cached: boolean
  generated_at: string
}

export interface TimelineBucket {
  timestamp: string
  bans: number
  captchas: number
}

export interface TimelineData {
  buckets: TimelineBucket[]
  range: string
  interval: string
  cached: boolean
}

export interface TopIP {
  ip: string
  count: number
  last_seen: string
  country: string
}

export interface TopIPsData {
  ips: TopIP[]
  range: string
  cached: boolean
}

export interface ScenarioEntry {
  name: string
  count: number
  percentage: number
}

export interface ScenariosData {
  scenarios: ScenarioEntry[]
  total: number
  range: string
  cached: boolean
}

export interface CrowdSecAlert {
  id: number
  scenario: string
  ip: string
  message: string
  events_count: number
  start_at: string
  stop_at: string
  created_at: string
  duration: string
  type: string
  origin: string
}

export interface AlertsData {
  alerts: CrowdSecAlert[]
  total: number
  source: string
  cached: boolean
}

export async function getDashboardSummary(range: TimeRange): Promise<DashboardSummary> {
  const resp = await client.get<DashboardSummary>('/admin/crowdsec/dashboard/summary', { params: { range } })
  return resp.data
}

export async function getDashboardTimeline(range: TimeRange): Promise<TimelineData> {
  const resp = await client.get<TimelineData>('/admin/crowdsec/dashboard/timeline', { params: { range } })
  return resp.data
}

export async function getDashboardTopIPs(range: TimeRange, limit = 10): Promise<TopIPsData> {
  const resp = await client.get<TopIPsData>('/admin/crowdsec/dashboard/top-ips', { params: { range, limit } })
  return resp.data
}

export async function getDashboardScenarios(range: TimeRange): Promise<ScenariosData> {
  const resp = await client.get<ScenariosData>('/admin/crowdsec/dashboard/scenarios', { params: { range } })
  return resp.data
}

export async function getAlerts(params: {
  range?: TimeRange
  scenario?: string
  limit?: number
  offset?: number
}): Promise<AlertsData> {
  const resp = await client.get<AlertsData>('/admin/crowdsec/alerts', { params })
  return resp.data
}

export async function exportDecisions(
  format: 'csv' | 'json',
  range: TimeRange,
  source = 'all',
): Promise<Blob> {
  const resp = await client.get<Blob>('/admin/crowdsec/decisions/export', {
    params: { format, range, source },
    responseType: 'blob',
  })
  return resp.data
}
