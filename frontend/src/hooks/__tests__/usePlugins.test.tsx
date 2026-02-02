import { describe, it, expect, vi, beforeEach } from 'vitest'
import { renderHook, waitFor } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import React from 'react'
import {
  usePlugins,
  usePlugin,
  useEnablePlugin,
  useDisablePlugin,
  useReloadPlugins,
} from '../usePlugins'
import * as api from '../../api/plugins'

vi.mock('../../api/plugins')

const mockBuiltInPlugin: api.PluginInfo = {
  id: 1,
  uuid: 'builtin-cloudflare',
  name: 'Cloudflare',
  type: 'cloudflare',
  enabled: true,
  status: 'loaded',
  is_built_in: true,
  version: '1.0.0',
  description: 'Cloudflare DNS provider',
  documentation_url: 'https://developers.cloudflare.com',
  created_at: '2025-01-01T00:00:00Z',
  updated_at: '2025-01-01T00:00:00Z',
}

const mockExternalPlugin: api.PluginInfo = {
  id: 2,
  uuid: 'external-powerdns',
  name: 'PowerDNS',
  type: 'powerdns',
  enabled: true,
  status: 'loaded',
  is_built_in: false,
  version: '1.0.0',
  author: 'Community',
  description: 'PowerDNS provider plugin',
  documentation_url: 'https://doc.powerdns.com',
  loaded_at: '2025-01-06T00:00:00Z',
  created_at: '2025-01-01T00:00:00Z',
  updated_at: '2025-01-06T00:00:00Z',
}

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

describe('usePlugins', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('returns plugins list on mount', async () => {
    const mockPlugins = [mockBuiltInPlugin, mockExternalPlugin]
    vi.mocked(api.getPlugins).mockResolvedValue(mockPlugins)

    const { result } = renderHook(() => usePlugins(), { wrapper: createWrapper() })

    expect(result.current.isLoading).toBe(true)
    expect(result.current.data).toBeUndefined()

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false)
    })

    expect(result.current.data).toEqual(mockPlugins)
    expect(result.current.isError).toBe(false)
    expect(api.getPlugins).toHaveBeenCalledTimes(1)
  })

  it('handles empty plugins list', async () => {
    vi.mocked(api.getPlugins).mockResolvedValue([])

    const { result } = renderHook(() => usePlugins(), { wrapper: createWrapper() })

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false)
    })

    expect(result.current.data).toEqual([])
    expect(result.current.isError).toBe(false)
  })

  it('handles error state on failure', async () => {
    const mockError = new Error('Failed to fetch plugins')
    vi.mocked(api.getPlugins).mockRejectedValue(mockError)

    const { result } = renderHook(() => usePlugins(), { wrapper: createWrapper() })

    await waitFor(() => {
      expect(result.current.isError).toBe(true)
    })

    expect(result.current.error).toEqual(mockError)
    expect(result.current.data).toBeUndefined()
  })
})

describe('usePlugin', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('fetches single plugin when id > 0', async () => {
    vi.mocked(api.getPlugin).mockResolvedValue(mockExternalPlugin)

    const { result } = renderHook(() => usePlugin(2), { wrapper: createWrapper() })

    expect(result.current.isLoading).toBe(true)

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false)
    })

    expect(result.current.data).toEqual(mockExternalPlugin)
    expect(api.getPlugin).toHaveBeenCalledWith(2)
  })

  it('is disabled when id = 0', async () => {
    vi.mocked(api.getPlugin).mockResolvedValue(mockExternalPlugin)

    const { result } = renderHook(() => usePlugin(0), { wrapper: createWrapper() })

    expect(result.current.isLoading).toBe(false)
    expect(result.current.data).toBeUndefined()
    expect(api.getPlugin).not.toHaveBeenCalled()
  })

  it('handles error state', async () => {
    const mockError = new Error('Plugin not found')
    vi.mocked(api.getPlugin).mockRejectedValue(mockError)

    const { result } = renderHook(() => usePlugin(999), { wrapper: createWrapper() })

    await waitFor(() => {
      expect(result.current.isError).toBe(true)
    })

    expect(result.current.error).toEqual(mockError)
  })
})

describe('useEnablePlugin', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('enables plugin successfully', async () => {
    const mockResponse = { message: 'Plugin enabled successfully' }
    vi.mocked(api.enablePlugin).mockResolvedValue(mockResponse)

    const { result } = renderHook(() => useEnablePlugin(), { wrapper: createWrapper() })

    result.current.mutate(2)

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true)
    })

    expect(api.enablePlugin).toHaveBeenCalledWith(2)
    expect(result.current.data).toEqual(mockResponse)
  })

  it('invalidates plugins list on success', async () => {
    vi.mocked(api.enablePlugin).mockResolvedValue({ message: 'Enabled' })
    vi.mocked(api.getPlugins).mockResolvedValue([mockExternalPlugin])

    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    })
    const invalidateSpy = vi.spyOn(queryClient, 'invalidateQueries')

    const wrapper = ({ children }: { children: React.ReactNode }) => (
      <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
    )

    const { result } = renderHook(() => useEnablePlugin(), { wrapper })

    result.current.mutate(2)

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true)
    })

    expect(invalidateSpy).toHaveBeenCalled()
  })

  it('handles enable errors', async () => {
    const mockError = new Error('Failed to enable plugin')
    vi.mocked(api.enablePlugin).mockRejectedValue(mockError)

    const { result } = renderHook(() => useEnablePlugin(), { wrapper: createWrapper() })

    result.current.mutate(2)

    await waitFor(() => {
      expect(result.current.isError).toBe(true)
    })

    expect(result.current.error).toEqual(mockError)
  })
})

describe('useDisablePlugin', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('disables plugin successfully', async () => {
    const mockResponse = { message: 'Plugin disabled successfully' }
    vi.mocked(api.disablePlugin).mockResolvedValue(mockResponse)

    const { result } = renderHook(() => useDisablePlugin(), { wrapper: createWrapper() })

    result.current.mutate(2)

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true)
    })

    expect(api.disablePlugin).toHaveBeenCalledWith(2)
    expect(result.current.data).toEqual(mockResponse)
  })

  it('invalidates plugins list on success', async () => {
    vi.mocked(api.disablePlugin).mockResolvedValue({ message: 'Disabled' })
    vi.mocked(api.getPlugins).mockResolvedValue([mockExternalPlugin])

    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    })
    const invalidateSpy = vi.spyOn(queryClient, 'invalidateQueries')

    const wrapper = ({ children }: { children: React.ReactNode }) => (
      <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
    )

    const { result } = renderHook(() => useDisablePlugin(), { wrapper })

    result.current.mutate(2)

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true)
    })

    expect(invalidateSpy).toHaveBeenCalled()
  })

  it('handles disable errors', async () => {
    const mockError = new Error('Cannot disable: plugin in use')
    vi.mocked(api.disablePlugin).mockRejectedValue(mockError)

    const { result } = renderHook(() => useDisablePlugin(), { wrapper: createWrapper() })

    result.current.mutate(2)

    await waitFor(() => {
      expect(result.current.isError).toBe(true)
    })

    expect(result.current.error).toEqual(mockError)
  })
})

describe('useReloadPlugins', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('reloads plugins successfully', async () => {
    const mockResponse = { message: 'Plugins reloaded', count: 3 }
    vi.mocked(api.reloadPlugins).mockResolvedValue(mockResponse)

    const { result } = renderHook(() => useReloadPlugins(), { wrapper: createWrapper() })

    result.current.mutate()

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true)
    })

    expect(api.reloadPlugins).toHaveBeenCalledTimes(1)
    expect(result.current.data).toEqual(mockResponse)
  })

  it('invalidates plugins list on success', async () => {
    vi.mocked(api.reloadPlugins).mockResolvedValue({ message: 'Reloaded', count: 2 })
    vi.mocked(api.getPlugins).mockResolvedValue([mockBuiltInPlugin, mockExternalPlugin])

    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    })
    const invalidateSpy = vi.spyOn(queryClient, 'invalidateQueries')

    const wrapper = ({ children }: { children: React.ReactNode }) => (
      <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
    )

    const { result } = renderHook(() => useReloadPlugins(), { wrapper })

    result.current.mutate()

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true)
    })

    expect(invalidateSpy).toHaveBeenCalled()
  })

  it('handles reload errors', async () => {
    const mockError = new Error('Failed to reload plugins')
    vi.mocked(api.reloadPlugins).mockRejectedValue(mockError)

    const { result } = renderHook(() => useReloadPlugins(), { wrapper: createWrapper() })

    result.current.mutate()

    await waitFor(() => {
      expect(result.current.isError).toBe(true)
    })

    expect(result.current.error).toEqual(mockError)
  })
})
