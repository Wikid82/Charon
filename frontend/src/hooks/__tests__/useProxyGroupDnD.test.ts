import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { act, renderHook } from '@testing-library/react'
import React from 'react'
import { toast } from 'react-hot-toast'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import type { ProxyGroup } from '../../api/proxyGroups'
import type { ProxyHost } from '../../api/proxyHosts'
import { useProxyGroupDnD } from '../useProxyGroupDnD'
import { QUERY_KEY } from '../useProxyHosts'

vi.mock('react-hot-toast', () => ({
  toast: { success: vi.fn(), error: vi.fn() },
}))

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string, opts?: Record<string, unknown>) => {
      if (opts && typeof opts.count === 'number') return `${key}:${opts.count}`
      return key
    },
  }),
}))

const makeGroup = (uuid: string, name = 'Group'): ProxyGroup => ({
  uuid,
  name,
  color: '#6366f1',
  description: '',
  created_at: '2024-01-01T00:00:00Z',
  updated_at: '2024-01-01T00:00:00Z',
})

const makeHost = (uuid: string, groupUuid: string | null): ProxyHost => ({
  uuid,
  name: `host-${uuid}`,
  domain_names: [],
  forward_host: 'localhost',
  forward_port: 80,
  forward_scheme: 'http',
  enabled: true,
  proxy_group_id: groupUuid,
  proxy_group: groupUuid ? { uuid: groupUuid, name: 'Group', color: '#6366f1' } : null,
} as unknown as ProxyHost)

const createWrapper = (queryClient: QueryClient) =>
  function Wrapper({ children }: { children: React.ReactNode }) {
    return React.createElement(QueryClientProvider, { client: queryClient }, children)
  }

describe('useProxyGroupDnD', () => {
  let qc: QueryClient

  beforeEach(() => {
    qc = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } })
    vi.clearAllMocks()
  })

  it('happy path: single host drag to new group calls bulkUpdateGroup and updates cache', async () => {
    const group = makeGroup('g1', 'Group A')
    const host = makeHost('h1', null)
    qc.setQueryData(QUERY_KEY, [host])

    const bulkUpdateGroup = vi.fn().mockResolvedValue({ updated: 1, errors: [] })
    const setSelectedHosts = vi.fn()

    const { result } = renderHook(
      () =>
        useProxyGroupDnD({
          hosts: [host],
          groups: [group],
          selectedHosts: new Set(),
          setSelectedHosts,
          bulkUpdateGroup,
        }),
      { wrapper: createWrapper(qc) },
    )

    act(() => {
      result.current.handleDragStart({ active: { id: 'h1' } } as Parameters<typeof result.current.handleDragStart>[0])
    })

    await act(async () => {
      await result.current.handleDragEnd({
        active: { id: 'h1' },
        over: { id: 'g1' },
      } as Parameters<typeof result.current.handleDragEnd>[0])
    })

    expect(bulkUpdateGroup).toHaveBeenCalledExactlyOnceWith(['h1'], 'g1')

    const cached = qc.getQueryData<ProxyHost[]>(QUERY_KEY)
    expect(cached?.[0].proxy_group?.uuid).toBe('g1')
  })

  it('no-op: host dragged to the same group does not call bulkUpdateGroup', async () => {
    const group = makeGroup('g1')
    const host = makeHost('h1', 'g1')
    qc.setQueryData(QUERY_KEY, [host])

    const bulkUpdateGroup = vi.fn()
    const setSelectedHosts = vi.fn()

    const { result } = renderHook(
      () =>
        useProxyGroupDnD({
          hosts: [host],
          groups: [group],
          selectedHosts: new Set(),
          setSelectedHosts,
          bulkUpdateGroup,
        }),
      { wrapper: createWrapper(qc) },
    )

    act(() => {
      result.current.handleDragStart({ active: { id: 'h1' } } as Parameters<typeof result.current.handleDragStart>[0])
    })

    await act(async () => {
      await result.current.handleDragEnd({
        active: { id: 'h1' },
        over: { id: 'g1' },
      } as Parameters<typeof result.current.handleDragEnd>[0])
    })

    expect(bulkUpdateGroup).not.toHaveBeenCalled()
  })

  it('API error: rolls back optimistic update and shows error toast', async () => {
    const group = makeGroup('g1')
    const host = makeHost('h1', null)
    const snapshot = [host]
    qc.setQueryData(QUERY_KEY, snapshot)

    const bulkUpdateGroup = vi.fn().mockRejectedValue(new Error('Network error'))
    const setSelectedHosts = vi.fn()

    const { result } = renderHook(
      () =>
        useProxyGroupDnD({
          hosts: [host],
          groups: [group],
          selectedHosts: new Set(),
          setSelectedHosts,
          bulkUpdateGroup,
        }),
      { wrapper: createWrapper(qc) },
    )

    act(() => {
      result.current.handleDragStart({ active: { id: 'h1' } } as Parameters<typeof result.current.handleDragStart>[0])
    })

    await act(async () => {
      await result.current.handleDragEnd({
        active: { id: 'h1' },
        over: { id: 'g1' },
      } as Parameters<typeof result.current.handleDragEnd>[0])
    })

    const cached = qc.getQueryData<ProxyHost[]>(QUERY_KEY)
    expect(cached?.[0].proxy_group?.uuid).toBeUndefined()
    expect(toast.error).toHaveBeenCalled()
  })

  it('partial success: shows error toast but does not roll back', async () => {
    const group = makeGroup('g1')
    const hosts = [makeHost('h1', null), makeHost('h2', null)]
    qc.setQueryData(QUERY_KEY, hosts)

    const bulkUpdateGroup = vi.fn().mockResolvedValue({
      updated: 1,
      errors: [{ uuid: 'h2', error: 'not found' }],
    })
    const setSelectedHosts = vi.fn()

    const { result } = renderHook(
      () =>
        useProxyGroupDnD({
          hosts,
          groups: [group],
          selectedHosts: new Set(['h1', 'h2']),
          setSelectedHosts,
          bulkUpdateGroup,
        }),
      { wrapper: createWrapper(qc) },
    )

    act(() => {
      result.current.handleDragStart({ active: { id: 'h1' } } as Parameters<typeof result.current.handleDragStart>[0])
    })

    await act(async () => {
      await result.current.handleDragEnd({
        active: { id: 'h1' },
        over: { id: 'g1' },
      } as Parameters<typeof result.current.handleDragEnd>[0])
    })

    expect(toast.error).toHaveBeenCalled()
    // No rollback — cache should still reflect optimistic state
    const cached = qc.getQueryData<ProxyHost[]>(QUERY_KEY)
    expect(cached?.every((h) => h.proxy_group?.uuid === 'g1')).toBe(true)
  })

  it('handleDragOver: sets overGroupId from event', () => {
    const group = makeGroup('g1')
    const host = makeHost('h1', null)
    const bulkUpdateGroup = vi.fn()
    const setSelectedHosts = vi.fn()

    const { result } = renderHook(
      () =>
        useProxyGroupDnD({
          hosts: [host],
          groups: [group],
          selectedHosts: new Set(),
          setSelectedHosts,
          bulkUpdateGroup,
        }),
      { wrapper: createWrapper(qc) },
    )

    act(() => {
      result.current.handleDragOver({ over: { id: 'g1' } } as Parameters<typeof result.current.handleDragOver>[0])
    })
    expect(result.current.overGroupId).toBe('g1')

    act(() => {
      result.current.handleDragOver({ over: null } as unknown as Parameters<typeof result.current.handleDragOver>[0])
    })
    expect(result.current.overGroupId).toBeNull()
  })

  it('handleDragEnd with no over: calls handleDragCancel', async () => {
    const group = makeGroup('g1')
    const host = makeHost('h1', null)
    const bulkUpdateGroup = vi.fn()
    const setSelectedHosts = vi.fn()

    const { result } = renderHook(
      () =>
        useProxyGroupDnD({
          hosts: [host],
          groups: [group],
          selectedHosts: new Set(),
          setSelectedHosts,
          bulkUpdateGroup,
        }),
      { wrapper: createWrapper(qc) },
    )

    act(() => {
      result.current.handleDragStart({ active: { id: 'h1' } } as Parameters<typeof result.current.handleDragStart>[0])
    })
    expect(result.current.activeDragId).toBe('h1')

    await act(async () => {
      await result.current.handleDragEnd({
        active: { id: 'h1' },
        over: null,
      } as unknown as Parameters<typeof result.current.handleDragEnd>[0])
    })

    expect(result.current.activeDragId).toBeNull()
    expect(result.current.hostsBeingDragged).toHaveLength(0)
    expect(bulkUpdateGroup).not.toHaveBeenCalled()
  })

  it('multi-select drag: all selected UUIDs passed to bulkUpdateGroup', async () => {
    const group = makeGroup('g1')
    const hosts = [makeHost('h1', null), makeHost('h2', null), makeHost('h3', null)]
    qc.setQueryData(QUERY_KEY, hosts)

    const bulkUpdateGroup = vi.fn().mockResolvedValue({ updated: 3, errors: [] })
    const setSelectedHosts = vi.fn()

    const { result } = renderHook(
      () =>
        useProxyGroupDnD({
          hosts,
          groups: [group],
          selectedHosts: new Set(['h1', 'h2', 'h3']),
          setSelectedHosts,
          bulkUpdateGroup,
        }),
      { wrapper: createWrapper(qc) },
    )

    act(() => {
      result.current.handleDragStart({ active: { id: 'h1' } } as Parameters<typeof result.current.handleDragStart>[0])
    })

    await act(async () => {
      await result.current.handleDragEnd({
        active: { id: 'h1' },
        over: { id: 'g1' },
      } as Parameters<typeof result.current.handleDragEnd>[0])
    })

    expect(bulkUpdateGroup).toHaveBeenCalledOnce()
    const [uuids] = bulkUpdateGroup.mock.calls[0]
    expect(uuids).toHaveLength(3)
    expect(uuids).toContain('h1')
    expect(uuids).toContain('h2')
    expect(uuids).toContain('h3')
  })
})
