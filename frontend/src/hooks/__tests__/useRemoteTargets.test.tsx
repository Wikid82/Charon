import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { renderHook, waitFor } from '@testing-library/react'
import React from 'react'
import { describe, it, expect, vi, beforeEach } from 'vitest'

import * as api from '../../api/backups'
import {
  useRemoteTargets,
  useCreateRemoteTarget,
  useUpdateRemoteTarget,
  useDeleteRemoteTarget,
  useTestRemoteTarget,
  REMOTE_TARGETS_QUERY_KEY,
} from '../useRemoteTargets'

vi.mock('../../api/backups')

const mockTarget: api.RemoteTarget = {
  uuid: 'r1',
  name: 'Home NAS',
  type: 'sftp',
  enabled: true,
  config: { host: 'nas.lan', port: 22, path: '/backups/charon', username: 'charon' },
  secrets_set: true,
  last_test_at: null,
  last_test_status: 'never',
  last_error: '',
  created_at: '2026-07-01T00:00:00Z',
  updated_at: '2026-07-01T00:00:00Z',
}

const createWrapper = () => {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return ({ children }: { children: React.ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  )
}

describe('useRemoteTargets', () => {
  beforeEach(() => vi.clearAllMocks())

  it('fetches the remote target list', async () => {
    vi.mocked(api.getRemoteTargets).mockResolvedValue([mockTarget])
    const { result } = renderHook(() => useRemoteTargets(), { wrapper: createWrapper() })

    await waitFor(() => expect(result.current.isLoading).toBe(false))
    expect(result.current.data).toEqual([mockTarget])
  })

  it('handles error state', async () => {
    vi.mocked(api.getRemoteTargets).mockRejectedValue(new Error('failed to list'))
    const { result } = renderHook(() => useRemoteTargets(), { wrapper: createWrapper() })

    await waitFor(() => expect(result.current.isError).toBe(true))
  })
})

describe('useCreateRemoteTarget', () => {
  beforeEach(() => vi.clearAllMocks())

  it('creates a target and invalidates the list', async () => {
    vi.mocked(api.createRemoteTarget).mockResolvedValue(mockTarget)
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    const invalidateSpy = vi.spyOn(queryClient, 'invalidateQueries')
    const wrapper = ({ children }: { children: React.ReactNode }) => (
      <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
    )

    const { result } = renderHook(() => useCreateRemoteTarget(), { wrapper })
    result.current.mutate({ name: 'Home NAS', type: 'sftp', config: {} })

    await waitFor(() => expect(result.current.isSuccess).toBe(true))
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: REMOTE_TARGETS_QUERY_KEY })
  })
})

describe('useUpdateRemoteTarget', () => {
  it('updates a target by uuid', async () => {
    vi.mocked(api.updateRemoteTarget).mockResolvedValue(mockTarget)
    const { result } = renderHook(() => useUpdateRemoteTarget(), { wrapper: createWrapper() })

    result.current.mutate({ uuid: 'r1', payload: { name: 'Renamed' } })

    await waitFor(() => expect(result.current.isSuccess).toBe(true))
    expect(api.updateRemoteTarget).toHaveBeenCalledWith('r1', { name: 'Renamed' })
  })
})

describe('useDeleteRemoteTarget', () => {
  it('removes the deleted target from the cached list without a refetch', async () => {
    vi.mocked(api.deleteRemoteTarget).mockResolvedValue(undefined)
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    queryClient.setQueryData(REMOTE_TARGETS_QUERY_KEY, [mockTarget, { ...mockTarget, uuid: 'r2', name: 'Other' }])
    const wrapper = ({ children }: { children: React.ReactNode }) => (
      <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
    )

    const { result } = renderHook(() => useDeleteRemoteTarget(), { wrapper })
    result.current.mutate('r1')

    await waitFor(() => expect(result.current.isSuccess).toBe(true))
    const cached = queryClient.getQueryData<api.RemoteTarget[]>(REMOTE_TARGETS_QUERY_KEY)
    expect(cached).toEqual([{ ...mockTarget, uuid: 'r2', name: 'Other' }])
  })
})

describe('useTestRemoteTarget', () => {
  it('tests a target connection and returns the result', async () => {
    vi.mocked(api.testRemoteTarget).mockResolvedValue({ success: true, message: 'Connected', latency_ms: 10 })
    const { result } = renderHook(() => useTestRemoteTarget(), { wrapper: createWrapper() })

    result.current.mutate('r1')

    await waitFor(() => expect(result.current.isSuccess).toBe(true))
    expect(result.current.data?.success).toBe(true)
  })

  it('surfaces a discovered_fingerprint for the SFTP host-key discovery flow', async () => {
    vi.mocked(api.testRemoteTarget).mockResolvedValue({
      success: false,
      message: 'host key not yet trusted',
      discovered_fingerprint: 'SHA256:abcdef',
    })
    const { result } = renderHook(() => useTestRemoteTarget(), { wrapper: createWrapper() })

    result.current.mutate('draft')

    await waitFor(() => expect(result.current.isSuccess).toBe(true))
    expect(result.current.data?.discovered_fingerprint).toBe('SHA256:abcdef')
  })
})
