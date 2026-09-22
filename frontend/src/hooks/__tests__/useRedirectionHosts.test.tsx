import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { act, renderHook, waitFor } from '@testing-library/react'
import React from 'react'
import toast from 'react-hot-toast'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { redirectionHostsApi, type RedirectionHost } from '../../api/redirectionHosts'
import {
  useCreateRedirectionHost,
  useDeleteRedirectionHost,
  useRedirectionHosts,
  useUpdateRedirectionHost,
} from '../useRedirectionHosts'

vi.mock('../../api/redirectionHosts', async () => {
  const actual = await vi.importActual<typeof import('../../api/redirectionHosts')>('../../api/redirectionHosts')
  return {
    ...actual,
    redirectionHostsApi: {
      list: vi.fn(),
      get: vi.fn(),
      create: vi.fn(),
      update: vi.fn(),
      delete: vi.fn(),
    },
  }
})

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

const sampleHost = (overrides: Partial<RedirectionHost> = {}): RedirectionHost => ({
  uuid: 'rh-1',
  name: 'Old blog redirect',
  domain_names: 'old-blog.example.com',
  target_url: 'https://newblog.example.com',
  status_code: 301,
  preserve_path: true,
  ssl_forced: true,
  http2_support: true,
  hsts_enabled: false,
  hsts_subdomains: false,
  enabled: true,
  certificate_id: null,
  certificate: null,
  dns_provider_id: null,
  dns_provider: null,
  use_dns_challenge: false,
  created_at: '2026-01-01T00:00:00Z',
  updated_at: '2026-01-01T00:00:00Z',
  ...overrides,
})

describe('useRedirectionHosts', () => {
  beforeEach(() => vi.clearAllMocks())

  it('returns data when fetch succeeds', async () => {
    const hosts = [sampleHost()]
    vi.mocked(redirectionHostsApi.list).mockResolvedValue(hosts)

    const { result } = renderHook(() => useRedirectionHosts(), { wrapper: createWrapper() })
    await waitFor(() => expect(result.current.isSuccess).toBe(true))
    expect(result.current.data).toEqual(hosts)
  })

  it('returns error state when fetch fails', async () => {
    vi.mocked(redirectionHostsApi.list).mockRejectedValue(new Error('Network error'))

    const { result } = renderHook(() => useRedirectionHosts(), { wrapper: createWrapper() })
    await waitFor(() => expect(result.current.isError).toBe(true))
    expect(result.current.error).toBeInstanceOf(Error)
  })
})

describe('useCreateRedirectionHost', () => {
  beforeEach(() => vi.clearAllMocks())

  it('calls create API and shows success toast', async () => {
    const host = sampleHost()
    vi.mocked(redirectionHostsApi.create).mockResolvedValue(host)
    vi.mocked(redirectionHostsApi.list).mockResolvedValue([host])

    const { result } = renderHook(() => useCreateRedirectionHost(), { wrapper: createWrapper() })
    await act(async () => {
      await result.current.mutateAsync({ name: 'Old blog redirect', domain_names: 'old-blog.example.com', target_url: 'https://newblog.example.com', status_code: 301 })
    })

    expect(redirectionHostsApi.create).toHaveBeenCalledWith({
      name: 'Old blog redirect',
      domain_names: 'old-blog.example.com',
      target_url: 'https://newblog.example.com',
      status_code: 301,
    })
    expect(toast.success).toHaveBeenCalledWith('Redirection host created')
  })

  it('shows error toast when create fails', async () => {
    vi.mocked(redirectionHostsApi.create).mockRejectedValue(new Error('Server error'))

    const { result } = renderHook(() => useCreateRedirectionHost(), { wrapper: createWrapper() })
    await act(async () => {
      try {
        await result.current.mutateAsync({ domain_names: 'x.com' })
      } catch {
        // expected
      }
    })

    expect(toast.error).toHaveBeenCalledWith('Failed to create redirection host: Server error')
  })
})

describe('useUpdateRedirectionHost', () => {
  beforeEach(() => vi.clearAllMocks())

  it('calls update API with uuid and data, shows success toast', async () => {
    const updated = sampleHost({ target_url: 'https://updated.example.com' })
    vi.mocked(redirectionHostsApi.update).mockResolvedValue(updated)
    vi.mocked(redirectionHostsApi.list).mockResolvedValue([updated])

    const { result } = renderHook(() => useUpdateRedirectionHost(), { wrapper: createWrapper() })
    await act(async () => {
      await result.current.mutateAsync({ uuid: 'rh-1', data: { target_url: 'https://updated.example.com' } })
    })

    expect(redirectionHostsApi.update).toHaveBeenCalledWith('rh-1', { target_url: 'https://updated.example.com' })
    expect(toast.success).toHaveBeenCalledWith('Redirection host updated')
  })

  it('shows error toast when update fails', async () => {
    vi.mocked(redirectionHostsApi.update).mockRejectedValue(new Error('Update failed'))

    const { result } = renderHook(() => useUpdateRedirectionHost(), { wrapper: createWrapper() })
    await act(async () => {
      try {
        await result.current.mutateAsync({ uuid: 'rh-1', data: { target_url: 'https://x.com' } })
      } catch {
        // expected
      }
    })

    expect(toast.error).toHaveBeenCalledWith('Failed to update redirection host: Update failed')
  })
})

describe('useDeleteRedirectionHost', () => {
  beforeEach(() => vi.clearAllMocks())

  it('calls delete API and shows success toast', async () => {
    vi.mocked(redirectionHostsApi.delete).mockResolvedValue(undefined)
    vi.mocked(redirectionHostsApi.list).mockResolvedValue([])

    const { result } = renderHook(() => useDeleteRedirectionHost(), { wrapper: createWrapper() })
    await act(async () => {
      await result.current.mutateAsync('rh-1')
    })

    expect(redirectionHostsApi.delete).toHaveBeenCalledWith('rh-1')
    expect(toast.success).toHaveBeenCalledWith('Redirection host deleted')
  })

  it('shows error toast when delete fails', async () => {
    vi.mocked(redirectionHostsApi.delete).mockRejectedValue(new Error('Delete failed'))

    const { result } = renderHook(() => useDeleteRedirectionHost(), { wrapper: createWrapper() })
    await act(async () => {
      try {
        await result.current.mutateAsync('rh-1')
      } catch {
        // expected
      }
    })

    expect(toast.error).toHaveBeenCalledWith('Failed to delete redirection host: Delete failed')
  })
})
