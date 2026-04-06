import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { renderHook, waitFor } from '@testing-library/react'
import { describe, it, expect, vi, beforeEach } from 'vitest'

import * as api from '../../api/crowdsecDashboard'
import {
  useDashboardSummary,
  useDashboardTimeline,
  useDashboardTopIPs,
  useDashboardScenarios,
  useAlerts,
} from '../useCrowdsecDashboard'

vi.mock('../../api/crowdsecDashboard')

describe('useCrowdsecDashboard hooks', () => {
  let queryClient: QueryClient

  beforeEach(() => {
    queryClient = new QueryClient({
      defaultOptions: {
        queries: { retry: false },
      },
    })
    vi.clearAllMocks()
  })

  const wrapper = ({ children }: { children: React.ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  )

  describe('useDashboardSummary', () => {
    it('should fetch summary data', async () => {
      const mockData: api.DashboardSummary = {
        total_decisions: 100,
        active_decisions: 10,
        unique_ips: 50,
        top_scenario: 'crowdsecurity/http-bad-user-agent',
        decisions_trend: 5.0,
        range: '24h',
        cached: false,
        generated_at: '2025-01-01T00:00:00Z',
      }
      vi.mocked(api.getDashboardSummary).mockResolvedValue(mockData)

      const { result } = renderHook(() => useDashboardSummary('24h'), { wrapper })

      await waitFor(() => expect(result.current.isSuccess).toBe(true))
      expect(result.current.data).toEqual(mockData)
      expect(api.getDashboardSummary).toHaveBeenCalledWith('24h')
    })
  })

  describe('useDashboardTimeline', () => {
    it('should fetch timeline data', async () => {
      const mockData: api.TimelineData = {
        buckets: [{ timestamp: '2025-01-01T00:00:00Z', bans: 5, captchas: 2 }],
        range: '24h',
        interval: '1h',
        cached: false,
      }
      vi.mocked(api.getDashboardTimeline).mockResolvedValue(mockData)

      const { result } = renderHook(() => useDashboardTimeline('24h'), { wrapper })

      await waitFor(() => expect(result.current.isSuccess).toBe(true))
      expect(result.current.data).toEqual(mockData)
    })
  })

  describe('useDashboardTopIPs', () => {
    it('should fetch top IPs with default limit', async () => {
      const mockData: api.TopIPsData = {
        ips: [{ ip: '1.2.3.4', count: 10, last_seen: '2025-01-01T00:00:00Z', country: 'US' }],
        range: '24h',
        cached: false,
      }
      vi.mocked(api.getDashboardTopIPs).mockResolvedValue(mockData)

      const { result } = renderHook(() => useDashboardTopIPs('24h'), { wrapper })

      await waitFor(() => expect(result.current.isSuccess).toBe(true))
      expect(api.getDashboardTopIPs).toHaveBeenCalledWith('24h', 10)
    })
  })

  describe('useDashboardScenarios', () => {
    it('should fetch scenario data', async () => {
      const mockData: api.ScenariosData = {
        scenarios: [{ name: 'crowdsecurity/ssh-bf', count: 20, percentage: 40 }],
        total: 50,
        range: '24h',
        cached: false,
      }
      vi.mocked(api.getDashboardScenarios).mockResolvedValue(mockData)

      const { result } = renderHook(() => useDashboardScenarios('24h'), { wrapper })

      await waitFor(() => expect(result.current.isSuccess).toBe(true))
      expect(result.current.data).toEqual(mockData)
    })
  })

  describe('useAlerts', () => {
    it('should fetch alerts with params object', async () => {
      const mockData: api.AlertsData = {
        alerts: [{
          id: 1, scenario: 'test', ip: '1.2.3.4', message: '', events_count: 1,
          start_at: '', stop_at: '', created_at: '', duration: '24h', type: 'ban', origin: 'cscli',
        }],
        total: 1,
        source: 'cscli',
        cached: false,
      }
      vi.mocked(api.getAlerts).mockResolvedValue(mockData)

      const { result } = renderHook(() => useAlerts({ range: '24h', limit: 20 }), { wrapper })

      await waitFor(() => expect(result.current.isSuccess).toBe(true))
      expect(api.getAlerts).toHaveBeenCalledWith({ range: '24h', limit: 20 })
    })
  })
})
