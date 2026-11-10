import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { renderHook, waitFor } from '@testing-library/react'
import React from 'react'
import { vi, describe, it, expect, beforeEach } from 'vitest'

import { getActiveAlerts, type ActiveAlert } from '../../api/system'
import { ACTIVE_ALERTS_POLL_INTERVAL_MS, ACTIVE_ALERTS_QUERY_KEY, useActiveAlerts } from '../useActiveAlerts'

vi.mock('../../api/system', () => ({
  getActiveAlerts: vi.fn(),
}))

const alerts: ActiveAlert[] = [
  { key: 'geoip_unavailable', severity: 'error', affected_lists: ['Office'], affected_lists_total: 1, database_available: false },
]

const createClient = () => new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: 0 } } })

const createWrapper = (queryClient: QueryClient) => {
  return ({ children }: { children: React.ReactNode }) =>
    React.createElement(QueryClientProvider, { client: queryClient }, children)
}

describe('useActiveAlerts', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.mocked(getActiveAlerts).mockResolvedValue(alerts)
  })

  it('uses the shared active-alerts query key', () => {
    expect(ACTIVE_ALERTS_QUERY_KEY).toEqual(['active-alerts'])
  })

  it('returns the alerts from getActiveAlerts', async () => {
    const queryClient = createClient()
    const { result } = renderHook(() => useActiveAlerts(), { wrapper: createWrapper(queryClient) })

    await waitFor(() => expect(result.current.data).toEqual(alerts))
    expect(getActiveAlerts).toHaveBeenCalledTimes(1)
  })

  it('polls every 30 seconds and refetches on window focus', async () => {
    const queryClient = createClient()
    const { result } = renderHook(() => useActiveAlerts(), { wrapper: createWrapper(queryClient) })

    await waitFor(() => expect(result.current.isSuccess).toBe(true))
    const query = queryClient.getQueryCache().find({ queryKey: ACTIVE_ALERTS_QUERY_KEY })
    expect(ACTIVE_ALERTS_POLL_INTERVAL_MS).toBe(30_000)
    expect(query?.options).toMatchObject({ refetchInterval: 30_000, refetchOnWindowFocus: true })
  })
})
