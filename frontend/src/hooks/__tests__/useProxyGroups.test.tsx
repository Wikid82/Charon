import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { act, renderHook, waitFor } from '@testing-library/react'
import React from 'react'
import toast from 'react-hot-toast'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { proxyGroupsApi } from '../../api/proxyGroups'
import {
  useCreateProxyGroup,
  useDeleteProxyGroup,
  useProxyGroups,
  useUpdateProxyGroup,
} from '../useProxyGroups'

vi.mock('../../api/proxyGroups', () => ({
  proxyGroupsApi: {
    list: vi.fn(),
    create: vi.fn(),
    update: vi.fn(),
    delete: vi.fn(),
  },
}))

vi.mock('react-hot-toast', () => ({
  default: { success: vi.fn(), error: vi.fn() },
}))

const createWrapper = () => {
  const qc = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false },
    },
  })
  return function Wrapper({ children }: { children: React.ReactNode }) {
    return <QueryClientProvider client={qc}>{children}</QueryClientProvider>
  }
}

const sampleGroup = (overrides = {}) => ({
  uuid: 'g1',
  name: 'Test Group',
  description: 'A test group',
  color: '#6366f1',
  created_at: '2024-01-01T00:00:00Z',
  updated_at: '2024-01-01T00:00:00Z',
  ...overrides,
})

describe('useProxyGroups', () => {
  beforeEach(() => vi.clearAllMocks())

  it('returns data when fetch succeeds', async () => {
    const groups = [sampleGroup()]
    vi.mocked(proxyGroupsApi.list).mockResolvedValue(groups)

    const { result } = renderHook(() => useProxyGroups(), { wrapper: createWrapper() })
    await waitFor(() => expect(result.current.isSuccess).toBe(true))
    expect(result.current.data).toEqual(groups)
  })

  it('returns error state when fetch fails', async () => {
    vi.mocked(proxyGroupsApi.list).mockRejectedValue(new Error('Network error'))

    const { result } = renderHook(() => useProxyGroups(), { wrapper: createWrapper() })
    await waitFor(() => expect(result.current.isError).toBe(true))
    expect(result.current.error).toBeInstanceOf(Error)
  })
})

describe('useCreateProxyGroup', () => {
  beforeEach(() => vi.clearAllMocks())

  it('calls create API and shows success toast', async () => {
    const group = sampleGroup()
    vi.mocked(proxyGroupsApi.create).mockResolvedValue(group)
    vi.mocked(proxyGroupsApi.list).mockResolvedValue([group])

    const { result } = renderHook(() => useCreateProxyGroup(), { wrapper: createWrapper() })
    await act(async () => {
      await result.current.mutateAsync({ name: 'Test Group', description: 'desc', color: '#6366f1' })
    })

    expect(proxyGroupsApi.create).toHaveBeenCalledWith({
      name: 'Test Group',
      description: 'desc',
      color: '#6366f1',
    })
    expect(toast.success).toHaveBeenCalledWith('Group created')
  })

  it('shows error toast when create fails', async () => {
    vi.mocked(proxyGroupsApi.create).mockRejectedValue(new Error('Server error'))

    const { result } = renderHook(() => useCreateProxyGroup(), { wrapper: createWrapper() })
    await act(async () => {
      try {
        await result.current.mutateAsync({ name: 'New Group' })
      } catch {
        // expected
      }
    })

    expect(toast.error).toHaveBeenCalledWith('Failed to create group: Server error')
  })
})

describe('useUpdateProxyGroup', () => {
  beforeEach(() => vi.clearAllMocks())

  it('calls update API with uuid and data, shows success toast', async () => {
    const updated = sampleGroup({ name: 'Updated' })
    vi.mocked(proxyGroupsApi.update).mockResolvedValue(updated)
    vi.mocked(proxyGroupsApi.list).mockResolvedValue([updated])

    const { result } = renderHook(() => useUpdateProxyGroup(), { wrapper: createWrapper() })
    await act(async () => {
      await result.current.mutateAsync({ uuid: 'g1', data: { name: 'Updated' } })
    })

    expect(proxyGroupsApi.update).toHaveBeenCalledWith('g1', { name: 'Updated' })
    expect(toast.success).toHaveBeenCalledWith('Group updated')
  })

  it('shows error toast when update fails', async () => {
    vi.mocked(proxyGroupsApi.update).mockRejectedValue(new Error('Update failed'))

    const { result } = renderHook(() => useUpdateProxyGroup(), { wrapper: createWrapper() })
    await act(async () => {
      try {
        await result.current.mutateAsync({ uuid: 'g1', data: { name: 'X' } })
      } catch {
        // expected
      }
    })

    expect(toast.error).toHaveBeenCalledWith('Failed to update group: Update failed')
  })
})

describe('useDeleteProxyGroup', () => {
  beforeEach(() => vi.clearAllMocks())

  it('calls delete API and shows success toast', async () => {
    vi.mocked(proxyGroupsApi.delete).mockResolvedValue(undefined)
    vi.mocked(proxyGroupsApi.list).mockResolvedValue([])

    const { result } = renderHook(() => useDeleteProxyGroup(), { wrapper: createWrapper() })
    await act(async () => {
      await result.current.mutateAsync('g1')
    })

    expect(proxyGroupsApi.delete).toHaveBeenCalledWith('g1')
    expect(toast.success).toHaveBeenCalledWith('Group deleted — hosts moved to Ungrouped')
  })

  it('shows error toast when delete fails', async () => {
    vi.mocked(proxyGroupsApi.delete).mockRejectedValue(new Error('Delete failed'))

    const { result } = renderHook(() => useDeleteProxyGroup(), { wrapper: createWrapper() })
    await act(async () => {
      try {
        await result.current.mutateAsync('g1')
      } catch {
        // expected
      }
    })

    expect(toast.error).toHaveBeenCalledWith('Failed to delete group: Delete failed')
  })
})
