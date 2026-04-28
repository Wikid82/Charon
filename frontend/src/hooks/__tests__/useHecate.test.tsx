import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { renderHook, waitFor, act } from '@testing-library/react'
import React from 'react'
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'

import * as api from '../../api/hecate'
import { useHecate, TUNNELS_QUERY_KEY, STATUS_QUERY_KEY } from '../useHecate'

vi.mock('../../api/hecate', () => ({
  listTunnels: vi.fn(),
  getTunnelStatus: vi.fn(),
  createTunnel: vi.fn(),
  updateTunnel: vi.fn(),
  deleteTunnel: vi.fn(),
  startTunnel: vi.fn(),
  stopTunnel: vi.fn(),
  rotateCredentials: vi.fn(),
}))

const createWrapper = () => {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  return ({ children }: { children: React.ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  )
}

const mockTunnel: api.TunnelConfig = {
  uuid: 'tunnel-1',
  name: 'CF Tunnel',
  provider: 'cloudflare',
  configuration: '{}',
  is_active: true,
  created_at: '2025-01-01T00:00:00Z',
  updated_at: '2025-01-01T00:00:00Z',
}

const mockStatus: api.TunnelStatus = {
  uuid: 'tunnel-1',
  name: 'CF Tunnel',
  provider: 'cloudflare',
  state: 'connected',
  uptime_seconds: 100,
  last_error: '',
}

describe('useHecate', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.mocked(api.listTunnels).mockResolvedValue([mockTunnel])
    vi.mocked(api.getTunnelStatus).mockResolvedValue([mockStatus])
  })

  afterEach(() => {
    vi.clearAllMocks()
  })

  it('loads tunnels and statuses on mount', async () => {
    const { result } = renderHook(() => useHecate(), { wrapper: createWrapper() })

    expect(result.current.loadingTunnels).toBe(true)

    await waitFor(() => {
      expect(result.current.loadingTunnels).toBe(false)
    })

    expect(result.current.tunnels).toEqual([mockTunnel])
    expect(result.current.statuses).toEqual([mockStatus])
    expect(result.current.error).toBeNull()
    expect(api.listTunnels).toHaveBeenCalledTimes(1)
    expect(api.getTunnelStatus).toHaveBeenCalledTimes(1)
  })

  it('handles tunnel loading error', async () => {
    vi.mocked(api.listTunnels).mockRejectedValue(new Error('fetch failed'))
    vi.mocked(api.getTunnelStatus).mockResolvedValue([])

    const { result } = renderHook(() => useHecate(), { wrapper: createWrapper() })

    await waitFor(() => {
      expect(result.current.loadingTunnels).toBe(false)
    })

    expect(result.current.error).toBe('fetch failed')
    expect(result.current.tunnels).toEqual([])
  })

  it('handles status loading error', async () => {
    vi.mocked(api.listTunnels).mockResolvedValue([])
    vi.mocked(api.getTunnelStatus).mockRejectedValue(new Error('status failed'))

    const { result } = renderHook(() => useHecate(), { wrapper: createWrapper() })

    await waitFor(() => {
      expect(result.current.loadingStatus).toBe(false)
    })

    expect(result.current.error).toBe('status failed')
  })

  it('getStatus returns the matching TunnelStatus', async () => {
    const { result } = renderHook(() => useHecate(), { wrapper: createWrapper() })

    await waitFor(() => expect(result.current.loadingTunnels).toBe(false))

    const status = result.current.getStatus('tunnel-1')
    expect(status).toEqual(mockStatus)
  })

  it('getStatus returns undefined for unknown uuid', async () => {
    const { result } = renderHook(() => useHecate(), { wrapper: createWrapper() })

    await waitFor(() => expect(result.current.loadingTunnels).toBe(false))

    expect(result.current.getStatus('nonexistent')).toBeUndefined()
  })

  it('createTunnel calls API and invalidates queries', async () => {
    const newTunnel = { ...mockTunnel, uuid: 'tunnel-2', name: 'New' }
    vi.mocked(api.createTunnel).mockImplementation(async () => {
      vi.mocked(api.listTunnels).mockResolvedValue([mockTunnel, newTunnel])
      return newTunnel
    })

    const { result } = renderHook(() => useHecate(), { wrapper: createWrapper() })
    await waitFor(() => expect(result.current.loadingTunnels).toBe(false))

    await act(async () => {
      await result.current.createTunnel({ name: 'New', provider: 'cloudflare', credentials: 'tok' })
    })

    expect(api.createTunnel).toHaveBeenCalledWith({ name: 'New', provider: 'cloudflare', credentials: 'tok' })
    await waitFor(() => expect(result.current.tunnels).toHaveLength(2))
  })

  it('updateTunnel calls API and invalidates tunnels query', async () => {
    const updated = { ...mockTunnel, name: 'Updated' }
    vi.mocked(api.updateTunnel).mockImplementation(async () => {
      vi.mocked(api.listTunnels).mockResolvedValue([updated])
      return { message: 'updated' }
    })

    const { result } = renderHook(() => useHecate(), { wrapper: createWrapper() })
    await waitFor(() => expect(result.current.loadingTunnels).toBe(false))

    await act(async () => {
      await result.current.updateTunnel({ uuid: 'tunnel-1', req: { name: 'Updated', provider: 'cloudflare' } })
    })

    expect(api.updateTunnel).toHaveBeenCalledWith('tunnel-1', { name: 'Updated', provider: 'cloudflare' })
  })

  it('deleteTunnel calls API and invalidates queries', async () => {
    vi.mocked(api.deleteTunnel).mockImplementation(async () => {
      vi.mocked(api.listTunnels).mockResolvedValue([])
    })

    const { result } = renderHook(() => useHecate(), { wrapper: createWrapper() })
    await waitFor(() => expect(result.current.loadingTunnels).toBe(false))

    await act(async () => {
      await result.current.deleteTunnel('tunnel-1')
    })

    expect(api.deleteTunnel).toHaveBeenCalledWith('tunnel-1')
  })

  it('startTunnel calls API and invalidates status query', async () => {
    vi.mocked(api.startTunnel).mockResolvedValue({ message: 'started' })

    const { result } = renderHook(() => useHecate(), { wrapper: createWrapper() })
    await waitFor(() => expect(result.current.loadingTunnels).toBe(false))

    await act(async () => {
      await result.current.startTunnel('tunnel-1')
    })

    expect(api.startTunnel).toHaveBeenCalledWith('tunnel-1')
  })

  it('stopTunnel calls API and invalidates status query', async () => {
    vi.mocked(api.stopTunnel).mockResolvedValue({ message: 'stopped' })

    const { result } = renderHook(() => useHecate(), { wrapper: createWrapper() })
    await waitFor(() => expect(result.current.loadingTunnels).toBe(false))

    await act(async () => {
      await result.current.stopTunnel('tunnel-1')
    })

    expect(api.stopTunnel).toHaveBeenCalledWith('tunnel-1')
  })

  it('rotateCredentials calls API', async () => {
    vi.mocked(api.rotateCredentials).mockResolvedValue({ message: 'rotated' })

    const { result } = renderHook(() => useHecate(), { wrapper: createWrapper() })
    await waitFor(() => expect(result.current.loadingTunnels).toBe(false))

    await act(async () => {
      await result.current.rotateCredentials({ uuid: 'tunnel-1', credentials: 'new-creds' })
    })

    expect(api.rotateCredentials).toHaveBeenCalledWith('tunnel-1', 'new-creds')
  })

  it('exposes loading and pending state flags', async () => {
    const { result } = renderHook(() => useHecate(), { wrapper: createWrapper() })

    expect(result.current.isCreating).toBe(false)
    expect(result.current.isUpdating).toBe(false)
    expect(result.current.isDeleting).toBe(false)
    expect(result.current.isStarting).toBe(false)
    expect(result.current.isStopping).toBe(false)
    expect(result.current.isRotating).toBe(false)
  })

  it('exports query key constants', () => {
    expect(TUNNELS_QUERY_KEY).toEqual(['hecate', 'tunnels'])
    expect(STATUS_QUERY_KEY).toEqual(['hecate', 'status'])
  })
})
