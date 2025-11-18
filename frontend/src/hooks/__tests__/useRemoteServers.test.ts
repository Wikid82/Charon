import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { renderHook, waitFor, act } from '@testing-library/react'
import { useRemoteServers } from '../useRemoteServers'
import * as api from '../../services/api'

// Mock the API
vi.mock('../../services/api', () => ({
  remoteServersAPI: {
    list: vi.fn(),
    get: vi.fn(),
    create: vi.fn(),
    update: vi.fn(),
    delete: vi.fn(),
    test: vi.fn(),
  },
}))

describe('useRemoteServers', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  afterEach(() => {
    vi.clearAllMocks()
  })

  it('loads all remote servers on mount', async () => {
    const mockServers = [
      { uuid: '1', name: 'Server 1', host: 'localhost', port: 8080, enabled: true },
      { uuid: '2', name: 'Server 2', host: '192.168.1.100', port: 3000, enabled: false },
    ]

    vi.mocked(api.remoteServersAPI.list).mockResolvedValue(mockServers)

    const { result } = renderHook(() => useRemoteServers())

    expect(result.current.loading).toBe(true)
    expect(result.current.servers).toEqual([])

    await waitFor(() => {
      expect(result.current.loading).toBe(false)
    })

    expect(result.current.servers).toEqual(mockServers)
    expect(result.current.error).toBeNull()
    expect(api.remoteServersAPI.list).toHaveBeenCalledOnce()
  })

  it('filters enabled servers', async () => {
    const mockServers = [
      { uuid: '1', name: 'Server 1', host: 'localhost', port: 8080, enabled: true },
      { uuid: '2', name: 'Server 2', host: '192.168.1.100', port: 3000, enabled: false },
      { uuid: '3', name: 'Server 3', host: '10.0.0.1', port: 9000, enabled: true },
    ]

    vi.mocked(api.remoteServersAPI.list).mockResolvedValue(mockServers)

    const { result } = renderHook(() => useRemoteServers())

    await waitFor(() => {
      expect(result.current.loading).toBe(false)
    })

    expect(result.current.enabledServers).toHaveLength(2)
    expect(result.current.enabledServers).toEqual([
      mockServers[0],
      mockServers[2],
    ])
  })

  it('handles loading errors', async () => {
    const mockError = new Error('Network error')
    vi.mocked(api.remoteServersAPI.list).mockRejectedValue(mockError)

    const { result } = renderHook(() => useRemoteServers())

    await waitFor(() => {
      expect(result.current.loading).toBe(false)
    })

    expect(result.current.error).toBe('Network error')
    expect(result.current.servers).toEqual([])
    expect(result.current.enabledServers).toEqual([])
  })

  it('creates a new remote server', async () => {
    vi.mocked(api.remoteServersAPI.list).mockResolvedValue([])
    const newServer = { name: 'New Server', host: 'new.local', port: 5000, enabled: true, provider: 'generic' }
    const createdServer = { uuid: '4', ...newServer }

    vi.mocked(api.remoteServersAPI.create).mockResolvedValue(createdServer)

    const { result } = renderHook(() => useRemoteServers())

    await waitFor(() => {
      expect(result.current.loading).toBe(false)
    })

    await act(async () => {
      await result.current.createServer(newServer)
    })

    expect(api.remoteServersAPI.create).toHaveBeenCalledWith(newServer)
    await waitFor(() => {
      expect(result.current.servers).toContainEqual(createdServer)
    })
  })

  it('updates an existing remote server', async () => {
    const existingServer = { uuid: '1', name: 'Server 1', host: 'localhost', port: 8080, enabled: true }
    vi.mocked(api.remoteServersAPI.list).mockResolvedValue([existingServer])

    const updatedServer = { ...existingServer, name: 'Updated Server' }
    vi.mocked(api.remoteServersAPI.update).mockResolvedValue(updatedServer)

    const { result } = renderHook(() => useRemoteServers())

    await waitFor(() => {
      expect(result.current.loading).toBe(false)
    })

    await act(async () => {
      await result.current.updateServer('1', { name: 'Updated Server' })
    })

    expect(api.remoteServersAPI.update).toHaveBeenCalledWith('1', { name: 'Updated Server' })
    await waitFor(() => {
      expect(result.current.servers[0].name).toBe('Updated Server')
    })
  })

  it('deletes a remote server', async () => {
    const servers = [
      { uuid: '1', name: 'Server 1', host: 'localhost', port: 8080, enabled: true },
      { uuid: '2', name: 'Server 2', host: '192.168.1.100', port: 3000, enabled: false },
    ]
    vi.mocked(api.remoteServersAPI.list).mockResolvedValue(servers)
    vi.mocked(api.remoteServersAPI.delete).mockResolvedValue(undefined)

    const { result } = renderHook(() => useRemoteServers())

    await waitFor(() => {
      expect(result.current.loading).toBe(false)
    })

    await act(async () => {
      await result.current.deleteServer('1')
    })

    expect(api.remoteServersAPI.delete).toHaveBeenCalledWith('1')
    await waitFor(() => {
      expect(result.current.servers).toHaveLength(1)
      expect(result.current.servers[0].uuid).toBe('2')
    })
  })

  it('tests server connection', async () => {
    vi.mocked(api.remoteServersAPI.list).mockResolvedValue([])
    const testResult = { reachable: true, address: 'localhost:8080' }
    vi.mocked(api.remoteServersAPI.test).mockResolvedValue(testResult)

    const { result } = renderHook(() => useRemoteServers())

    await waitFor(() => {
      expect(result.current.loading).toBe(false)
    })

    const response = await result.current.testConnection('1')

    expect(api.remoteServersAPI.test).toHaveBeenCalledWith('1')
    expect(response).toEqual(testResult)
  })

  it('handles create errors', async () => {
    vi.mocked(api.remoteServersAPI.list).mockResolvedValue([])
    const mockError = new Error('Failed to create')
    vi.mocked(api.remoteServersAPI.create).mockRejectedValue(mockError)

    const { result } = renderHook(() => useRemoteServers())

    await waitFor(() => {
      expect(result.current.loading).toBe(false)
    })

    await expect(result.current.createServer({ name: 'Test', host: 'localhost', port: 8080 })).rejects.toThrow('Failed to create')
  })

  it('handles update errors', async () => {
    const server = { uuid: '1', name: 'Server 1', host: 'localhost', port: 8080, enabled: true }
    vi.mocked(api.remoteServersAPI.list).mockResolvedValue([server])
    const mockError = new Error('Failed to update')
    vi.mocked(api.remoteServersAPI.update).mockRejectedValue(mockError)

    const { result } = renderHook(() => useRemoteServers())

    await waitFor(() => {
      expect(result.current.loading).toBe(false)
    })

    await expect(result.current.updateServer('1', { name: 'Updated' })).rejects.toThrow('Failed to update')
  })

  it('handles delete errors', async () => {
    const server = { uuid: '1', name: 'Server 1', host: 'localhost', port: 8080, enabled: true }
    vi.mocked(api.remoteServersAPI.list).mockResolvedValue([server])
    const mockError = new Error('Failed to delete')
    vi.mocked(api.remoteServersAPI.delete).mockRejectedValue(mockError)

    const { result } = renderHook(() => useRemoteServers())

    await waitFor(() => {
      expect(result.current.loading).toBe(false)
    })

    await expect(result.current.deleteServer('1')).rejects.toThrow('Failed to delete')
  })

  it('handles connection test errors', async () => {
    vi.mocked(api.remoteServersAPI.list).mockResolvedValue([])
    const mockError = new Error('Connection failed')
    vi.mocked(api.remoteServersAPI.test).mockRejectedValue(mockError)

    const { result } = renderHook(() => useRemoteServers())

    await waitFor(() => {
      expect(result.current.loading).toBe(false)
    })

    await expect(result.current.testConnection('1')).rejects.toThrow('Connection failed')
  })
})
