import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { act, renderHook, waitFor } from '@testing-library/react'
import React from 'react'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import * as api from '../../api/databaseMaintenance'
import {
  DATABASE_STATUS_QUERY_KEY,
  useCancelOptimize,
  useDatabaseStatus,
  useMaintenanceStatus,
  useRequestOptimize,
} from '../useDatabaseMaintenance'

vi.mock('../../api/databaseMaintenance')

const setup = () => {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  const wrapper = ({ children }: { children: React.ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  )
  return { queryClient, wrapper }
}

describe('useDatabaseMaintenance hooks', () => {
  beforeEach(() => vi.clearAllMocks())

  it('useDatabaseStatus returns the card data', async () => {
    vi.mocked(api.getDatabaseStatus).mockResolvedValue({ size_bytes: 5 } as api.DatabaseStatus)
    const { wrapper } = setup()
    const { result } = renderHook(() => useDatabaseStatus(), { wrapper })
    await waitFor(() => expect(result.current.data?.size_bytes).toBe(5))
  })

  it('useDatabaseStatus does not retry a failure (e.g. 403 for non-admins)', async () => {
    vi.mocked(api.getDatabaseStatus).mockRejectedValue(new Error('forbidden'))
    const queryClient = new QueryClient() // library default retry would be 3
    const wrapper = ({ children }: { children: React.ReactNode }) => (
      <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
    )
    const { result } = renderHook(() => useDatabaseStatus(), { wrapper })
    await waitFor(() => expect(result.current.isError).toBe(true))
    expect(api.getDatabaseStatus).toHaveBeenCalledTimes(1)
  })

  it('useRequestOptimize POSTs then invalidates the card query', async () => {
    vi.mocked(api.requestOptimizeOnRestart).mockResolvedValue({ requested: true })
    const { queryClient, wrapper } = setup()
    const spy = vi.spyOn(queryClient, 'invalidateQueries')
    const { result } = renderHook(() => useRequestOptimize(), { wrapper })
    await act(async () => {
      await result.current.mutateAsync()
    })
    expect(api.requestOptimizeOnRestart).toHaveBeenCalledTimes(1)
    expect(spy).toHaveBeenCalledWith({ queryKey: DATABASE_STATUS_QUERY_KEY })
  })

  it('useRequestOptimize still refreshes after a 409', async () => {
    vi.mocked(api.requestOptimizeOnRestart).mockRejectedValue(new Error('nothing_to_optimize'))
    const { queryClient, wrapper } = setup()
    const spy = vi.spyOn(queryClient, 'invalidateQueries')
    const { result } = renderHook(() => useRequestOptimize(), { wrapper })
    await act(async () => {
      await result.current.mutateAsync().catch(() => undefined)
    })
    expect(spy).toHaveBeenCalledWith({ queryKey: DATABASE_STATUS_QUERY_KEY })
  })

  it('useCancelOptimize DELETEs then invalidates the card query', async () => {
    vi.mocked(api.cancelOptimizeOnRestart).mockResolvedValue({ requested: false })
    const { queryClient, wrapper } = setup()
    const spy = vi.spyOn(queryClient, 'invalidateQueries')
    const { result } = renderHook(() => useCancelOptimize(), { wrapper })
    await act(async () => {
      await result.current.mutateAsync()
    })
    expect(api.cancelOptimizeOnRestart).toHaveBeenCalledTimes(1)
    expect(spy).toHaveBeenCalledWith({ queryKey: DATABASE_STATUS_QUERY_KEY })
  })

  it('useMaintenanceStatus only runs when enabled', async () => {
    vi.mocked(api.getMaintenanceStatus).mockResolvedValue({ active: true, phase: 'converting', elapsed_seconds: 1 })
    const { wrapper } = setup()
    const off = renderHook(() => useMaintenanceStatus(false), { wrapper })
    expect(off.result.current.fetchStatus).toBe('idle')
    expect(api.getMaintenanceStatus).not.toHaveBeenCalled()

    const on = renderHook(() => useMaintenanceStatus(true), { wrapper })
    await waitFor(() => expect(on.result.current.data?.active).toBe(true))
  })
})
