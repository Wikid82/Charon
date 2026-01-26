import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { renderHook, waitFor, act } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import React from 'react'
import { useRemoteServers } from '../useRemoteServers'
import * as api from '../../api/remoteServers'

// Mock the API
vi.mock('../../api/remoteServers', () => ({
  getRemoteServers: vi.fn(),
  createRemoteServer: vi.fn(),
  updateRemoteServer: vi.fn(),
  deleteRemoteServer: vi.fn(),
  testRemoteServerConnection: vi.fn(),
}))

const createMockServer = (overrides: Partial<api.RemoteServer> = {}): api.RemoteServer => ({
  uuid: '1',
  name: 'Server 1',
  provider: 'generic',
  host: 'localhost',
  port: 8080,
  enabled: true,
  reachable: true,
  created_at: '2025-01-01T00:00:00Z',
  updated_at: '2025-01-01T00:00:00Z',
  ...overrides,
})

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

describe('useRemoteServers', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  afterEach(() => {
    vi.clearAllMocks()
  })

  it('loads all remote servers on mount', async () => {
    const mockServers = [
      createMockServer({ uuid: '1', name: 'Server 1', host: 'localhost', port: 8080, enabled: true }),
      createMockServer({ uuid: '2', name: 'Server 2', host: '192.168.1.100', port: 3000, enabled: false }),
    ]

    vi.mocked(api.getRemoteServers).mockResolvedValue(mockServers)

    const { result } = renderHook(() => useRemoteServers(), { wrapper: createWrapper() })

    expect(result.current.loading).toBe(true)
    expect(result.current.servers).toEqual([])

    await waitFor(() => {
      expect(result.current.loading).toBe(false)
    })

    expect(result.current.servers).toEqual(mockServers)
    expect(result.current.error).toBeNull()
    expect(api.getRemoteServers).toHaveBeenCalledTimes(1)
  })

  it('handles loading errors', async () => {
    const mockError = new Error('Network error')
    vi.mocked(api.getRemoteServers).mockRejectedValue(mockError)

    const { result } = renderHook(() => useRemoteServers(), { wrapper: createWrapper() })

    await waitFor(() => {
      expect(result.current.loading).toBe(false)
    })

    expect(result.current.error).toBe('Network error')
    expect(result.current.servers).toEqual([])
  })

  it('creates a new remote server', async () => {
    vi.mocked(api.getRemoteServers).mockResolvedValue([])
    const newServer = { name: 'New Server', host: 'new.local', port: 5000, provider: 'generic' }
    const createdServer = createMockServer({ uuid: '4', ...newServer, enabled: true })

    vi.mocked(api.createRemoteServer).mockImplementation(async () => {
      vi.mocked(api.getRemoteServers).mockResolvedValue([createdServer])
      return createdServer
    })

    const { result } = renderHook(() => useRemoteServers(), { wrapper: createWrapper() })

    await waitFor(() => {
      expect(result.current.loading).toBe(false)
    })

    await act(async () => {
      await result.current.createServer(newServer)
    })

    expect(api.createRemoteServer).toHaveBeenCalledWith(newServer)
    await waitFor(() => {
      expect(result.current.servers).toContainEqual(createdServer)
    })
  })

  it('updates an existing remote server', async () => {
    const existingServer = createMockServer({ uuid: '1', name: 'Server 1', host: 'localhost', port: 8080, enabled: true })
    let servers = [existingServer]
    vi.mocked(api.getRemoteServers).mockImplementation(() => Promise.resolve(servers))

    vi.mocked(api.updateRemoteServer).mockImplementation(async (_, data) => {
      servers = [{ ...existingServer, ...data }]
      return servers[0]
    })

    const { result } = renderHook(() => useRemoteServers(), { wrapper: createWrapper() })

    await waitFor(() => {
      expect(result.current.loading).toBe(false)
    })

    await act(async () => {
      await result.current.updateServer('1', { name: 'Updated Server' })
    })

    expect(api.updateRemoteServer).toHaveBeenCalledWith('1', { name: 'Updated Server' })
    await waitFor(() => {
      expect(result.current.servers[0].name).toBe('Updated Server')
    })
  })

  it('deletes a remote server', async () => {
    const servers = [
      createMockServer({ uuid: '1', name: 'Server 1', host: 'localhost', port: 8080, enabled: true }),
      createMockServer({ uuid: '2', name: 'Server 2', host: '192.168.1.100', port: 3000, enabled: false }),
    ]
    vi.mocked(api.getRemoteServers).mockResolvedValue(servers)
    vi.mocked(api.deleteRemoteServer).mockImplementation(async (uuid) => {
      const remaining = servers.filter(s => s.uuid !== uuid)
      vi.mocked(api.getRemoteServers).mockResolvedValue(remaining)
    })

    const { result } = renderHook(() => useRemoteServers(), { wrapper: createWrapper() })

    await waitFor(() => {
      expect(result.current.loading).toBe(false)
    })

    await act(async () => {
      await result.current.deleteServer('1')
    })

    expect(api.deleteRemoteServer).toHaveBeenCalledWith('1')
    await waitFor(() => {
      expect(result.current.servers).toHaveLength(1)
      expect(result.current.servers[0].uuid).toBe('2')
    })
  })

  it('tests server connection', async () => {
    vi.mocked(api.getRemoteServers).mockResolvedValue([])
    const testResult = { reachable: true, address: 'localhost:8080' }
    vi.mocked(api.testRemoteServerConnection).mockResolvedValue(testResult)

    const { result } = renderHook(() => useRemoteServers(), { wrapper: createWrapper() })

    await waitFor(() => {
      expect(result.current.loading).toBe(false)
    })

    const response = await result.current.testConnection('1')

    expect(api.testRemoteServerConnection).toHaveBeenCalledWith('1')
    expect(response).toEqual(testResult)
  })

  it('handles create errors', async () => {
    vi.mocked(api.getRemoteServers).mockResolvedValue([])
    const mockError = new Error('Failed to create')
    vi.mocked(api.createRemoteServer).mockRejectedValue(mockError)

    const { result } = renderHook(() => useRemoteServers(), { wrapper: createWrapper() })

    await waitFor(() => {
      expect(result.current.loading).toBe(false)
    })

    await expect(result.current.createServer({ name: 'Test', host: 'localhost', port: 8080 })).rejects.toThrow('Failed to create')
  })

  it('handles update errors', async () => {
    const server = createMockServer({ uuid: '1', name: 'Server 1', host: 'localhost', port: 8080, enabled: true })
    vi.mocked(api.getRemoteServers).mockResolvedValue([server])
    const mockError = new Error('Failed to update')
    vi.mocked(api.updateRemoteServer).mockRejectedValue(mockError)

    const { result } = renderHook(() => useRemoteServers(), { wrapper: createWrapper() })

    await waitFor(() => {
      expect(result.current.loading).toBe(false)
    })

    await expect(result.current.updateServer('1', { name: 'Updated Server' })).rejects.toThrow('Failed to update')
  })

  it('handles delete errors', async () => {
    const server = createMockServer({ uuid: '1', name: 'Server 1', host: 'localhost', port: 8080, enabled: true })
    vi.mocked(api.getRemoteServers).mockResolvedValue([server])
    const mockError = new Error('Failed to delete')
    vi.mocked(api.deleteRemoteServer).mockRejectedValue(mockError)

    const { result } = renderHook(() => useRemoteServers(), { wrapper: createWrapper() })

    await waitFor(() => {
      expect(result.current.loading).toBe(false)
    })

    await expect(result.current.deleteServer('1')).rejects.toThrow('Failed to delete')
  })

  it('handles connection test errors', async () => {
    vi.mocked(api.getRemoteServers).mockResolvedValue([])
    const mockError = new Error('Connection failed')
    vi.mocked(api.testRemoteServerConnection).mockRejectedValue(mockError)

    const { result } = renderHook(() => useRemoteServers(), { wrapper: createWrapper() })

    await waitFor(() => {
      expect(result.current.loading).toBe(false)
    })

    await expect(result.current.testConnection('1')).rejects.toThrow('Connection failed')
  })
})
