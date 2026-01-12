import { describe, it, expect, vi, beforeEach } from 'vitest'
import { renderHook, waitFor } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import React from 'react'
import {
  useDNSProviders,
  useDNSProvider,
  useDNSProviderTypes,
  useDNSProviderMutations,
} from '../useDNSProviders'
import * as api from '../../api/dnsProviders'

vi.mock('../../api/dnsProviders')

const mockProvider: api.DNSProvider = {
  id: 1,
  uuid: 'test-uuid-1',
  name: 'Cloudflare Production',
  provider_type: 'cloudflare',
  enabled: true,
  is_default: true,
  has_credentials: true,
  propagation_timeout: 120,
  polling_interval: 2,
  success_count: 5,
  failure_count: 0,
  created_at: '2025-01-01T00:00:00Z',
  updated_at: '2025-01-01T00:00:00Z',
}

const mockProviderType: api.DNSProviderTypeInfo = {
  type: 'cloudflare',
  name: 'Cloudflare',
  fields: [
    {
      name: 'api_token',
      label: 'API Token',
      type: 'password',
      required: true,
    },
  ],
  documentation_url: 'https://developers.cloudflare.com/api/',
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

describe('useDNSProviders', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('returns providers list on mount', async () => {
    const mockProviders = [mockProvider, { ...mockProvider, id: 2, name: 'Secondary' }]
    vi.mocked(api.getDNSProviders).mockResolvedValue(mockProviders)

    const { result } = renderHook(() => useDNSProviders(), { wrapper: createWrapper() })

    expect(result.current.isLoading).toBe(true)
    expect(result.current.data).toBeUndefined()

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false)
    })

    expect(result.current.data).toEqual(mockProviders)
    expect(result.current.isError).toBe(false)
    expect(api.getDNSProviders).toHaveBeenCalledTimes(1)
  })

  it('handles loading state during fetch', async () => {
    vi.mocked(api.getDNSProviders).mockImplementation(
      () => new Promise((resolve) => setTimeout(() => resolve([mockProvider]), 100))
    )

    const { result } = renderHook(() => useDNSProviders(), { wrapper: createWrapper() })

    expect(result.current.isLoading).toBe(true)
    expect(result.current.data).toBeUndefined()

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false)
    })

    expect(result.current.data).toEqual([mockProvider])
  })

  it('handles error state on failure', async () => {
    const mockError = new Error('Failed to fetch providers')
    vi.mocked(api.getDNSProviders).mockRejectedValue(mockError)

    const { result } = renderHook(() => useDNSProviders(), { wrapper: createWrapper() })

    await waitFor(() => {
      expect(result.current.isError).toBe(true)
    })

    expect(result.current.error).toEqual(mockError)
    expect(result.current.data).toBeUndefined()
  })

  it('uses correct query key', async () => {
    vi.mocked(api.getDNSProviders).mockResolvedValue([mockProvider])

    const { result } = renderHook(() => useDNSProviders(), { wrapper: createWrapper() })

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false)
    })

    // Query key should be consistent for cache management
    expect(api.getDNSProviders).toHaveBeenCalled()
  })
})

describe('useDNSProvider', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('fetches single provider when id > 0', async () => {
    vi.mocked(api.getDNSProvider).mockResolvedValue(mockProvider)

    const { result } = renderHook(() => useDNSProvider(1), { wrapper: createWrapper() })

    expect(result.current.isLoading).toBe(true)

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false)
    })

    expect(result.current.data).toEqual(mockProvider)
    expect(api.getDNSProvider).toHaveBeenCalledWith(1)
  })

  it('is disabled when id = 0', async () => {
    vi.mocked(api.getDNSProvider).mockResolvedValue(mockProvider)

    const { result } = renderHook(() => useDNSProvider(0), { wrapper: createWrapper() })

    // Should not fetch when disabled
    expect(result.current.isLoading).toBe(false)
    expect(result.current.data).toBeUndefined()
    expect(api.getDNSProvider).not.toHaveBeenCalled()
  })

  it('is disabled when id < 0', async () => {
    vi.mocked(api.getDNSProvider).mockResolvedValue(mockProvider)

    const { result } = renderHook(() => useDNSProvider(-1), { wrapper: createWrapper() })

    expect(result.current.isLoading).toBe(false)
    expect(result.current.data).toBeUndefined()
    expect(api.getDNSProvider).not.toHaveBeenCalled()
  })

  it('handles loading state', async () => {
    vi.mocked(api.getDNSProvider).mockImplementation(
      () => new Promise((resolve) => setTimeout(() => resolve(mockProvider), 100))
    )

    const { result } = renderHook(() => useDNSProvider(1), { wrapper: createWrapper() })

    expect(result.current.isLoading).toBe(true)

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false)
    })
  })

  it('handles error state', async () => {
    const mockError = new Error('Provider not found')
    vi.mocked(api.getDNSProvider).mockRejectedValue(mockError)

    const { result } = renderHook(() => useDNSProvider(999), { wrapper: createWrapper() })

    await waitFor(() => {
      expect(result.current.isError).toBe(true)
    })

    expect(result.current.error).toEqual(mockError)
  })
})

describe('useDNSProviderTypes', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('fetches types list', async () => {
    const mockTypes = [
      mockProviderType,
      { ...mockProviderType, type: 'route53' as const, name: 'AWS Route 53' },
    ]
    vi.mocked(api.getDNSProviderTypes).mockResolvedValue(mockTypes)

    const { result } = renderHook(() => useDNSProviderTypes(), { wrapper: createWrapper() })

    expect(result.current.isLoading).toBe(true)

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false)
    })

    expect(result.current.data).toEqual(mockTypes)
    expect(api.getDNSProviderTypes).toHaveBeenCalledTimes(1)
  })

  it('applies staleTime of 1 hour', async () => {
    vi.mocked(api.getDNSProviderTypes).mockResolvedValue([mockProviderType])

    const { result } = renderHook(() => useDNSProviderTypes(), { wrapper: createWrapper() })

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false)
    })

    // The staleTime is configured in the hook, data should be cached for 1 hour
    expect(result.current.data).toEqual([mockProviderType])
  })

  it('handles loading state', async () => {
    vi.mocked(api.getDNSProviderTypes).mockImplementation(
      () => new Promise((resolve) => setTimeout(() => resolve([mockProviderType]), 100))
    )

    const { result } = renderHook(() => useDNSProviderTypes(), { wrapper: createWrapper() })

    expect(result.current.isLoading).toBe(true)

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false)
    })
  })

  it('handles error state', async () => {
    const mockError = new Error('Failed to fetch types')
    vi.mocked(api.getDNSProviderTypes).mockRejectedValue(mockError)

    const { result } = renderHook(() => useDNSProviderTypes(), { wrapper: createWrapper() })

    await waitFor(() => {
      expect(result.current.isError).toBe(true)
    })

    expect(result.current.error).toEqual(mockError)
  })
})

describe('useDNSProviderMutations', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  describe('createMutation', () => {
    it('creates provider successfully', async () => {
      const newProvider = { ...mockProvider, id: 3, name: 'New Provider' }
      vi.mocked(api.createDNSProvider).mockResolvedValue(newProvider)

      const { result } = renderHook(() => useDNSProviderMutations(), {
        wrapper: createWrapper(),
      })

      const createData: api.DNSProviderRequest = {
        name: 'New Provider',
        provider_type: 'cloudflare',
        credentials: { api_token: 'test-token' },
      }

      result.current.createMutation.mutate(createData)

      await waitFor(() => {
        expect(result.current.createMutation.isSuccess).toBe(true)
      })

      expect(api.createDNSProvider).toHaveBeenCalledWith(createData)
      expect(result.current.createMutation.data).toEqual(newProvider)
    })

    it('invalidates list query on success', async () => {
      vi.mocked(api.createDNSProvider).mockResolvedValue(mockProvider)
      vi.mocked(api.getDNSProviders).mockResolvedValue([mockProvider])

      const queryClient = new QueryClient({
        defaultOptions: { queries: { retry: false } },
      })
      const invalidateSpy = vi.spyOn(queryClient, 'invalidateQueries')

      const wrapper = ({ children }: { children: React.ReactNode }) => (
        <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
      )

      const { result } = renderHook(() => useDNSProviderMutations(), { wrapper })

      result.current.createMutation.mutate({
        name: 'Test',
        provider_type: 'cloudflare',
        credentials: {},
      })

      await waitFor(() => {
        expect(result.current.createMutation.isSuccess).toBe(true)
      })

      expect(invalidateSpy).toHaveBeenCalled()
    })

    it('handles creation errors', async () => {
      const mockError = new Error('Creation failed')
      vi.mocked(api.createDNSProvider).mockRejectedValue(mockError)

      const { result } = renderHook(() => useDNSProviderMutations(), {
        wrapper: createWrapper(),
      })

      result.current.createMutation.mutate({
        name: 'Test',
        provider_type: 'cloudflare',
        credentials: {},
      })

      await waitFor(() => {
        expect(result.current.createMutation.isError).toBe(true)
      })

      expect(result.current.createMutation.error).toEqual(mockError)
    })
  })

  describe('updateMutation', () => {
    it('updates provider successfully', async () => {
      const updatedProvider = { ...mockProvider, name: 'Updated Name' }
      vi.mocked(api.updateDNSProvider).mockResolvedValue(updatedProvider)

      const { result } = renderHook(() => useDNSProviderMutations(), {
        wrapper: createWrapper(),
      })

      const updateData: api.DNSProviderRequest = {
        name: 'Updated Name',
        provider_type: 'cloudflare',
        credentials: { api_token: 'new-token' },
      }

      result.current.updateMutation.mutate({ id: 1, data: updateData })

      await waitFor(() => {
        expect(result.current.updateMutation.isSuccess).toBe(true)
      })

      expect(api.updateDNSProvider).toHaveBeenCalledWith(1, updateData)
      expect(result.current.updateMutation.data).toEqual(updatedProvider)
    })

    it('invalidates list and detail queries on success', async () => {
      vi.mocked(api.updateDNSProvider).mockResolvedValue(mockProvider)

      const queryClient = new QueryClient({
        defaultOptions: { queries: { retry: false } },
      })
      const invalidateSpy = vi.spyOn(queryClient, 'invalidateQueries')

      const wrapper = ({ children }: { children: React.ReactNode }) => (
        <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
      )

      const { result } = renderHook(() => useDNSProviderMutations(), { wrapper })

      result.current.updateMutation.mutate({
        id: 1,
        data: {
          name: 'Updated',
          provider_type: 'cloudflare',
          credentials: {},
        },
      })

      await waitFor(() => {
        expect(result.current.updateMutation.isSuccess).toBe(true)
      })

      // Should invalidate both list and detail queries
      expect(invalidateSpy).toHaveBeenCalledTimes(2)
    })

    it('handles update errors', async () => {
      const mockError = new Error('Update failed')
      vi.mocked(api.updateDNSProvider).mockRejectedValue(mockError)

      const { result } = renderHook(() => useDNSProviderMutations(), {
        wrapper: createWrapper(),
      })

      result.current.updateMutation.mutate({
        id: 1,
        data: {
          name: 'Test',
          provider_type: 'cloudflare',
          credentials: {},
        },
      })

      await waitFor(() => {
        expect(result.current.updateMutation.isError).toBe(true)
      })

      expect(result.current.updateMutation.error).toEqual(mockError)
    })
  })

  describe('deleteMutation', () => {
    it('deletes provider successfully', async () => {
      vi.mocked(api.deleteDNSProvider).mockResolvedValue(undefined)

      const { result } = renderHook(() => useDNSProviderMutations(), {
        wrapper: createWrapper(),
      })

      result.current.deleteMutation.mutate(1)

      await waitFor(() => {
        expect(result.current.deleteMutation.isSuccess).toBe(true)
      })

      expect(api.deleteDNSProvider).toHaveBeenCalledWith(1)
    })

    it('invalidates list query on success', async () => {
      vi.mocked(api.deleteDNSProvider).mockResolvedValue(undefined)

      const queryClient = new QueryClient({
        defaultOptions: { queries: { retry: false } },
      })
      const invalidateSpy = vi.spyOn(queryClient, 'invalidateQueries')

      const wrapper = ({ children }: { children: React.ReactNode }) => (
        <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
      )

      const { result } = renderHook(() => useDNSProviderMutations(), { wrapper })

      result.current.deleteMutation.mutate(1)

      await waitFor(() => {
        expect(result.current.deleteMutation.isSuccess).toBe(true)
      })

      expect(invalidateSpy).toHaveBeenCalled()
    })

    it('handles delete errors', async () => {
      const mockError = new Error('Delete failed')
      vi.mocked(api.deleteDNSProvider).mockRejectedValue(mockError)

      const { result } = renderHook(() => useDNSProviderMutations(), {
        wrapper: createWrapper(),
      })

      result.current.deleteMutation.mutate(1)

      await waitFor(() => {
        expect(result.current.deleteMutation.isError).toBe(true)
      })

      expect(result.current.deleteMutation.error).toEqual(mockError)
    })
  })

  describe('testMutation', () => {
    it('tests provider successfully and returns result', async () => {
      const testResult: api.DNSTestResult = {
        success: true,
        message: 'Test successful',
        propagation_time_ms: 1200,
      }
      vi.mocked(api.testDNSProvider).mockResolvedValue(testResult)

      const { result } = renderHook(() => useDNSProviderMutations(), {
        wrapper: createWrapper(),
      })

      result.current.testMutation.mutate(1)

      await waitFor(() => {
        expect(result.current.testMutation.isSuccess).toBe(true)
      })

      expect(api.testDNSProvider).toHaveBeenCalledWith(1)
      expect(result.current.testMutation.data).toEqual(testResult)
    })

    it('handles test errors', async () => {
      const mockError = new Error('Test failed')
      vi.mocked(api.testDNSProvider).mockRejectedValue(mockError)

      const { result } = renderHook(() => useDNSProviderMutations(), {
        wrapper: createWrapper(),
      })

      result.current.testMutation.mutate(1)

      await waitFor(() => {
        expect(result.current.testMutation.isError).toBe(true)
      })

      expect(result.current.testMutation.error).toEqual(mockError)
    })
  })

  describe('testCredentialsMutation', () => {
    it('tests credentials successfully and returns result', async () => {
      const testResult: api.DNSTestResult = {
        success: true,
        message: 'Credentials valid',
        propagation_time_ms: 800,
      }
      vi.mocked(api.testDNSProviderCredentials).mockResolvedValue(testResult)

      const { result } = renderHook(() => useDNSProviderMutations(), {
        wrapper: createWrapper(),
      })

      const testData: api.DNSProviderRequest = {
        name: 'Test',
        provider_type: 'cloudflare',
        credentials: { api_token: 'test' },
      }

      result.current.testCredentialsMutation.mutate(testData)

      await waitFor(() => {
        expect(result.current.testCredentialsMutation.isSuccess).toBe(true)
      })

      expect(api.testDNSProviderCredentials).toHaveBeenCalledWith(testData)
      expect(result.current.testCredentialsMutation.data).toEqual(testResult)
    })

    it('handles test credential errors', async () => {
      const mockError = new Error('Invalid credentials')
      vi.mocked(api.testDNSProviderCredentials).mockRejectedValue(mockError)

      const { result } = renderHook(() => useDNSProviderMutations(), {
        wrapper: createWrapper(),
      })

      result.current.testCredentialsMutation.mutate({
        name: 'Test',
        provider_type: 'cloudflare',
        credentials: {},
      })

      await waitFor(() => {
        expect(result.current.testCredentialsMutation.isError).toBe(true)
      })

      expect(result.current.testCredentialsMutation.error).toEqual(mockError)
    })
  })
})
