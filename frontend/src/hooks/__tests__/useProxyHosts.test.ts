import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { renderHook, waitFor } from '@testing-library/react'
import { useProxyHosts } from '../useProxyHosts'
import * as api from '../../services/api'

// Mock the API
vi.mock('../../services/api', () => ({
  proxyHostsAPI: {
    list: vi.fn(),
    get: vi.fn(),
    create: vi.fn(),
    update: vi.fn(),
    delete: vi.fn(),
  },
}))

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

    vi.mocked(api.proxyHostsAPI.list).mockResolvedValue(mockHosts)

    const { result } = renderHook(() => useProxyHosts())

    expect(result.current.loading).toBe(true)
    expect(result.current.hosts).toEqual([])

    await waitFor(() => {
      expect(result.current.loading).toBe(false)
    })

    expect(result.current.hosts).toEqual(mockHosts)
    expect(result.current.error).toBeNull()
    expect(api.proxyHostsAPI.list).toHaveBeenCalledOnce()
  })

  it('handles loading errors', async () => {
    const mockError = new Error('Failed to fetch')
    vi.mocked(api.proxyHostsAPI.list).mockRejectedValue(mockError)

    const { result } = renderHook(() => useProxyHosts())

    await waitFor(() => {
      expect(result.current.loading).toBe(false)
    })

    expect(result.current.error).toBe('Failed to fetch')
    expect(result.current.hosts).toEqual([])
  })

  it('creates a new proxy host', async () => {
    vi.mocked(api.proxyHostsAPI.list).mockResolvedValue([])
    const newHost = { domain_names: 'new.com', forward_host: 'localhost', forward_port: 9000 }
    const createdHost = { uuid: '3', ...newHost, enabled: true }
    
    vi.mocked(api.proxyHostsAPI.create).mockResolvedValue(createdHost)

    const { result } = renderHook(() => useProxyHosts())

    await waitFor(() => {
      expect(result.current.loading).toBe(false)
    })

    await result.current.createHost(newHost)

    expect(api.proxyHostsAPI.create).toHaveBeenCalledWith(newHost)
    expect(api.proxyHostsAPI.list).toHaveBeenCalledTimes(2) // Initial load + reload after create
  })

  it('updates an existing proxy host', async () => {
    const existingHost = { uuid: '1', domain_names: 'test.com', enabled: true, forward_host: 'localhost', forward_port: 8080 }
    vi.mocked(api.proxyHostsAPI.list).mockResolvedValue([existingHost])
    
    const updatedHost = { ...existingHost, domain_names: 'updated.com' }
    vi.mocked(api.proxyHostsAPI.update).mockResolvedValue(updatedHost)

    const { result } = renderHook(() => useProxyHosts())

    await waitFor(() => {
      expect(result.current.loading).toBe(false)
    })

    await result.current.updateHost('1', { domain_names: 'updated.com' })

    expect(api.proxyHostsAPI.update).toHaveBeenCalledWith('1', { domain_names: 'updated.com' })
    expect(api.proxyHostsAPI.list).toHaveBeenCalledTimes(2)
  })

  it('deletes a proxy host', async () => {
    const hosts = [
      { uuid: '1', domain_names: 'test.com', enabled: true, forward_host: 'localhost', forward_port: 8080 },
      { uuid: '2', domain_names: 'app.com', enabled: true, forward_host: 'localhost', forward_port: 3000 },
    ]
    vi.mocked(api.proxyHostsAPI.list).mockResolvedValue(hosts)
    vi.mocked(api.proxyHostsAPI.delete).mockResolvedValue(undefined)

    const { result } = renderHook(() => useProxyHosts())

    await waitFor(() => {
      expect(result.current.loading).toBe(false)
    })

    await result.current.deleteHost('1')

    expect(api.proxyHostsAPI.delete).toHaveBeenCalledWith('1')
    expect(api.proxyHostsAPI.list).toHaveBeenCalledTimes(2)
  })

  it('handles create errors', async () => {
    vi.mocked(api.proxyHostsAPI.list).mockResolvedValue([])
    const mockError = new Error('Failed to create')
    vi.mocked(api.proxyHostsAPI.create).mockRejectedValue(mockError)

    const { result } = renderHook(() => useProxyHosts())

    await waitFor(() => {
      expect(result.current.loading).toBe(false)
    })

    await expect(result.current.createHost({ domain_names: 'test.com', forward_host: 'localhost', forward_port: 8080 })).rejects.toThrow('Failed to create')
  })

  it('handles update errors', async () => {
    const host = { uuid: '1', domain_names: 'test.com', enabled: true, forward_host: 'localhost', forward_port: 8080 }
    vi.mocked(api.proxyHostsAPI.list).mockResolvedValue([host])
    const mockError = new Error('Failed to update')
    vi.mocked(api.proxyHostsAPI.update).mockRejectedValue(mockError)

    const { result } = renderHook(() => useProxyHosts())

    await waitFor(() => {
      expect(result.current.loading).toBe(false)
    })

    await expect(result.current.updateHost('1', { domain_names: 'updated.com' })).rejects.toThrow('Failed to update')
  })

  it('handles delete errors', async () => {
    const host = { uuid: '1', domain_names: 'test.com', enabled: true, forward_host: 'localhost', forward_port: 8080 }
    vi.mocked(api.proxyHostsAPI.list).mockResolvedValue([host])
    const mockError = new Error('Failed to delete')
    vi.mocked(api.proxyHostsAPI.delete).mockRejectedValue(mockError)

    const { result } = renderHook(() => useProxyHosts())

    await waitFor(() => {
      expect(result.current.loading).toBe(false)
    })

    await expect(result.current.deleteHost('1')).rejects.toThrow('Failed to delete')
  })
})
