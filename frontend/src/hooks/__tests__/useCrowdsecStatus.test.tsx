import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { renderHook, waitFor } from '@testing-library/react'
import { describe, it, expect, vi, beforeEach } from 'vitest'

import * as crowdsecApi from '../../api/crowdsec'
import { useCrowdsecStatus } from '../useCrowdsecStatus'

import type { ReactNode } from 'react'

vi.mock('../../api/crowdsec')

const wrapper = ({ children }: { children: ReactNode }) => {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return <QueryClientProvider client={qc}>{children}</QueryClientProvider>
}

describe('useCrowdsecStatus', () => {
  beforeEach(() => vi.clearAllMocks())

  it('returns the status on success', async () => {
    vi.mocked(crowdsecApi.statusCrowdsec).mockResolvedValue({ running: true, pid: 42, lapi_ready: true })
    const { result } = renderHook(() => useCrowdsecStatus(), { wrapper })
    await waitFor(() => expect(result.current.isSuccess).toBe(true))
    expect(result.current.data).toMatchObject({ running: true, pid: 42 })
  })

  it('maps errors to null without failing the query', async () => {
    vi.mocked(crowdsecApi.statusCrowdsec).mockRejectedValue(new Error('boom'))
    const { result } = renderHook(() => useCrowdsecStatus(), { wrapper })
    await waitFor(() => expect(result.current.isSuccess).toBe(true))
    expect(result.current.data).toBeNull()
    expect(crowdsecApi.statusCrowdsec).toHaveBeenCalledTimes(1)
  })
})
