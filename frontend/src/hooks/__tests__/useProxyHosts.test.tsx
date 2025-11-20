import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { renderHook, waitFor, act } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import React from 'react'
import { useProxyHosts } from '../useProxyHosts'
import * as api from '../../api/proxyHosts'

// Mock the API
vi.mock('../../api/proxyHosts', () => ({
  getProxyHosts: vi.fn(),
  createProxyHost: vi.fn(),
  updateProxyHost: vi.fn(),
  deleteProxyHost: vi.fn(),
}))

const createWrapper = () => {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: {
        retry: false,
      },
    },
  })
  return ({ children }: { children: React.ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  )
}

describe('useProxyHosts', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  afterEach(() => {
    vi.clearAllMocks()
  })

  it('loads proxy hosts on mount', async () => {
    const mockHosts = [
      { uuid: '1', domain_names: 'test.com', enabled: true, forward_host: 'localhost', forward_port: 8080 },
      { uuid: '2', domain_names: 'app.com', enabled: true, forward_host: 'localhost', forward_port: 3000 },
    ]

    vi.mocked(api.getProxyHosts).mockResolvedValue(mockHosts)

    const { result } = renderHook(() => useProxyHosts(), { wrapper: createWrapper() })

    expect(result.current.loading).toBe(true)
    expect(result.current.hosts).toEqual([])

    await waitFor(() => {
      expect(result.current.loading).toBe(false)
    })

    expect(result.current.hosts).toEqual(mockHosts)
    expect(result.current.error).toBeNull()
    expect(api.getProxyHosts).toHaveBeenCalledOnce()
  })

  it('handles loading errors', async () => {
    const mockError = new Error('Failed to fetch')
    vi.mocked(api.getProxyHosts).mockRejectedValue(mockError)

    const { result } = renderHook(() => useProxyHosts(), { wrapper: createWrapper() })

    await waitFor(() => {
      expect(result.current.loading).toBe(false)
    })

    expect(result.current.error).toBe('Failed to fetch')
    expect(result.current.hosts).toEqual([])
  })

  it('creates a new proxy host', async () => {
    vi.mocked(api.getProxyHosts).mockResolvedValue([])
    const newHost = { domain_names: 'new.com', forward_host: 'localhost', forward_port: 9000 }
    const createdHost = { uuid: '3', ...newHost, enabled: true }

    vi.mocked(api.createProxyHost).mockImplementation(async () => {
      vi.mocked(api.getProxyHosts).mockResolvedValue([createdHost])
      return createdHost
    })

    const { result } = renderHook(() => useProxyHosts(), { wrapper: createWrapper() })

    await waitFor(() => {
      expect(result.current.loading).toBe(false)
    })

    await act(async () => {
      await result.current.createHost(newHost)
    })

    expect(api.createProxyHost).toHaveBeenCalledWith(newHost)
    await waitFor(() => {
      expect(result.current.hosts).toContainEqual(createdHost)
    })
  })

  it('updates an existing proxy host', async () => {
    const existingHost = { uuid: '1', domain_names: 'test.com', enabled: true, forward_host: 'localhost', forward_port: 8080 }
    let hosts = [existingHost]
    vi.mocked(api.getProxyHosts).mockImplementation(() => Promise.resolve(hosts))

    const updatedHost = { ...existingHost, domain_names: 'updated.com' }
    vi.mocked(api.updateProxyHost).mockImplementation(async (uuid, data) => {
      hosts = [{ ...existingHost, ...data }]
      return hosts[0]
    })

    const { result } = renderHook(() => useProxyHosts(), { wrapper: createWrapper() })

    await waitFor(() => {
      expect(result.current.loading).toBe(false)
    })

    await act(async () => {
      await result.current.updateHost('1', { domain_names: 'updated.com' })
    })

    expect(api.updateProxyHost).toHaveBeenCalledWith('1', { domain_names: 'updated.com' })
    await waitFor(() => {
      expect(result.current.hosts[0].domain_names).toBe('updated.com')
    })
  })

  it('deletes a proxy host', async () => {
    const hosts = [
      { uuid: '1', domain_names: 'test.com', enabled: true, forward_host: 'localhost', forward_port: 8080 },
      { uuid: '2', domain_names: 'app.com', enabled: true, forward_host: 'localhost', forward_port: 3000 },
    ]
    vi.mocked(api.getProxyHosts).mockResolvedValue(hosts)
    vi.mocked(api.deleteProxyHost).mockImplementation(async (uuid) => {
      const remaining = hosts.filter(h => h.uuid !== uuid)
      vi.mocked(api.getProxyHosts).mockResolvedValue(remaining)
    })

    const { result } = renderHook(() => useProxyHosts(), { wrapper: createWrapper() })

    await waitFor(() => {
      expect(result.current.loading).toBe(false)
    })

    await act(async () => {
      await result.current.deleteHost('1')
    })

    expect(api.deleteProxyHost).toHaveBeenCalledWith('1')
    await waitFor(() => {
      expect(result.current.hosts).toHaveLength(1)
      expect(result.current.hosts[0].uuid).toBe('2')
    })
  })

  it('handles create errors', async () => {
    vi.mocked(api.getProxyHosts).mockResolvedValue([])
    const mockError = new Error('Failed to create')
    vi.mocked(api.createProxyHost).mockRejectedValue(mockError)

    const { result } = renderHook(() => useProxyHosts(), { wrapper: createWrapper() })

    await waitFor(() => {
      expect(result.current.loading).toBe(false)
    })

    await expect(result.current.createHost({ domain_names: 'test.com', forward_host: 'localhost', forward_port: 8080 })).rejects.toThrow('Failed to create')
  })

  it('handles update errors', async () => {
    const host = { uuid: '1', domain_names: 'test.com', enabled: true, forward_host: 'localhost', forward_port: 8080 }
    vi.mocked(api.getProxyHosts).mockResolvedValue([host])
    const mockError = new Error('Failed to update')
    vi.mocked(api.updateProxyHost).mockRejectedValue(mockError)

    const { result } = renderHook(() => useProxyHosts(), { wrapper: createWrapper() })

    await waitFor(() => {
      expect(result.current.loading).toBe(false)
    })

    await expect(result.current.updateHost('1', { domain_names: 'updated.com' })).rejects.toThrow('Failed to update')
  })

  it('handles delete errors', async () => {
    const host = { uuid: '1', domain_names: 'test.com', enabled: true, forward_host: 'localhost', forward_port: 8080 }
    vi.mocked(api.getProxyHosts).mockResolvedValue([host])
    const mockError = new Error('Failed to delete')
    vi.mocked(api.deleteProxyHost).mockRejectedValue(mockError)

    const { result } = renderHook(() => useProxyHosts(), { wrapper: createWrapper() })

    await waitFor(() => {
      expect(result.current.loading).toBe(false)
    })

    await expect(result.current.deleteHost('1')).rejects.toThrow('Failed to delete')
  })
})
