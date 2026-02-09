import { describe, it, expect, vi, beforeEach } from 'vitest'
import { renderHook, waitFor } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { ReactNode } from 'react'
import {
  useCredentials,
  useCredential,
  useCreateCredential,
  useUpdateCredential,
  useDeleteCredential,
  useTestCredential,
  useEnableMultiCredentials,
} from '../useCredentials'
import * as credentialsApi from '../../api/credentials'

vi.mock('../../api/credentials')

const createWrapper = () => {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false },
    },
  })
  return function Wrapper({ children }: { children: ReactNode }) {
    return <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  }
}

describe('useCredentials', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  describe('useCredentials', () => {
    it('fetches credentials for a provider', async () => {
      const mockCredentials = [
        { id: 1, label: 'Test', zone_filter: 'example.com' },
        { id: 2, label: 'Test2', zone_filter: '*.test.com' },
      ] as Awaited<ReturnType<typeof credentialsApi.getCredentials>>
      vi.mocked(credentialsApi.getCredentials).mockResolvedValue(mockCredentials)

      const { result } = renderHook(() => useCredentials(1), { wrapper: createWrapper() })

      await waitFor(() => expect(result.current.isSuccess).toBe(true))
      expect(result.current.data).toEqual(mockCredentials)
      expect(credentialsApi.getCredentials).toHaveBeenCalledWith(1)
    })

    it('does not fetch when provider ID is 0', () => {
      renderHook(() => useCredentials(0), { wrapper: createWrapper() })
      expect(credentialsApi.getCredentials).not.toHaveBeenCalled()
    })

    it('handles fetch errors', async () => {
      vi.mocked(credentialsApi.getCredentials).mockRejectedValue(new Error('Network error'))

      const { result } = renderHook(() => useCredentials(1), { wrapper: createWrapper() })

      await waitFor(() => expect(result.current.isError).toBe(true))
      expect(result.current.error).toBeTruthy()
    })
  })

  describe('useCredential', () => {
    it('fetches a single credential', async () => {
      const mockCredential = { id: 1, label: 'Test', zone_filter: 'example.com' } as Awaited<ReturnType<typeof credentialsApi.getCredential>>
      vi.mocked(credentialsApi.getCredential).mockResolvedValue(mockCredential)

      const { result } = renderHook(() => useCredential(1, 1), { wrapper: createWrapper() })

      await waitFor(() => expect(result.current.isSuccess).toBe(true))
      expect(result.current.data).toEqual(mockCredential)
      expect(credentialsApi.getCredential).toHaveBeenCalledWith(1, 1)
    })

    it('does not fetch when provider or credential ID is 0', () => {
      renderHook(() => useCredential(0, 1), { wrapper: createWrapper() })
      expect(credentialsApi.getCredential).not.toHaveBeenCalled()

      renderHook(() => useCredential(1, 0), { wrapper: createWrapper() })
      expect(credentialsApi.getCredential).not.toHaveBeenCalled()
    })
  })

  describe('useCreateCredential', () => {
    it('creates a credential and invalidates queries', async () => {
      const mockCredential = { id: 3, label: 'New', zone_filter: 'new.com' } as Awaited<ReturnType<typeof credentialsApi.createCredential>>
      vi.mocked(credentialsApi.createCredential).mockResolvedValue(mockCredential)

      const { result } = renderHook(() => useCreateCredential(), { wrapper: createWrapper() })

      const data = {
        label: 'New',
        zone_filter: 'new.com',
        credentials: { api_token: 'test' },
      }

      await result.current.mutateAsync({ providerId: 1, data })

      expect(credentialsApi.createCredential).toHaveBeenCalledWith(1, data)
    })

    it('handles creation errors', async () => {
      vi.mocked(credentialsApi.createCredential).mockRejectedValue(
        new Error('Validation failed')
      )

      const { result } = renderHook(() => useCreateCredential(), { wrapper: createWrapper() })

      const data = {
        label: '',
        zone_filter: '',
        credentials: {},
      }

      await expect(result.current.mutateAsync({ providerId: 1, data })).rejects.toThrow(
        'Validation failed'
      )
    })
  })

  describe('useUpdateCredential', () => {
    it('updates a credential and invalidates queries', async () => {
      const mockCredential = { id: 1, label: 'Updated', zone_filter: 'updated.com' } as Awaited<ReturnType<typeof credentialsApi.updateCredential>>
      vi.mocked(credentialsApi.updateCredential).mockResolvedValue(mockCredential)

      const { result } = renderHook(() => useUpdateCredential(), { wrapper: createWrapper() })

      const data = {
        label: 'Updated',
        zone_filter: 'updated.com',
        credentials: { api_token: 'new_token' },
      }

      await result.current.mutateAsync({ providerId: 1, credentialId: 1, data })

      expect(credentialsApi.updateCredential).toHaveBeenCalledWith(1, 1, data)
    })

    it('handles update errors', async () => {
      vi.mocked(credentialsApi.updateCredential).mockRejectedValue(new Error('Not found'))

      const { result } = renderHook(() => useUpdateCredential(), { wrapper: createWrapper() })

      const data = {
        label: 'Updated',
        zone_filter: 'updated.com',
        credentials: {},
      }

      await expect(
        result.current.mutateAsync({ providerId: 1, credentialId: 999, data })
      ).rejects.toThrow('Not found')
    })
  })

  describe('useDeleteCredential', () => {
    it('deletes a credential and invalidates queries', async () => {
      vi.mocked(credentialsApi.deleteCredential).mockResolvedValue()

      const { result } = renderHook(() => useDeleteCredential(), { wrapper: createWrapper() })

      await result.current.mutateAsync({ providerId: 1, credentialId: 1 })

      expect(credentialsApi.deleteCredential).toHaveBeenCalledWith(1, 1)
    })

    it('handles delete errors', async () => {
      vi.mocked(credentialsApi.deleteCredential).mockRejectedValue(
        new Error('Credential in use')
      )

      const { result } = renderHook(() => useDeleteCredential(), { wrapper: createWrapper() })

      await expect(
        result.current.mutateAsync({ providerId: 1, credentialId: 1 })
      ).rejects.toThrow('Credential in use')
    })
  })

  describe('useTestCredential', () => {
    it('tests a credential successfully', async () => {
      const mockResult = { success: true, message: 'Test passed', propagation_time_ms: 1500 }
      vi.mocked(credentialsApi.testCredential).mockResolvedValue(mockResult)

      const { result } = renderHook(() => useTestCredential(), { wrapper: createWrapper() })

      const testResult = await result.current.mutateAsync({ providerId: 1, credentialId: 1 })

      expect(credentialsApi.testCredential).toHaveBeenCalledWith(1, 1)
      expect(testResult).toEqual(mockResult)
    })

    it('handles test failures', async () => {
      const mockResult = { success: false, error: 'Invalid credentials' }
      vi.mocked(credentialsApi.testCredential).mockResolvedValue(mockResult)

      const { result } = renderHook(() => useTestCredential(), { wrapper: createWrapper() })

      const testResult = await result.current.mutateAsync({ providerId: 1, credentialId: 1 })

      expect(testResult.success).toBe(false)
      expect(testResult.error).toBe('Invalid credentials')
    })

    it('handles network errors during test', async () => {
      vi.mocked(credentialsApi.testCredential).mockRejectedValue(new Error('Network timeout'))

      const { result } = renderHook(() => useTestCredential(), { wrapper: createWrapper() })

      await expect(
        result.current.mutateAsync({ providerId: 1, credentialId: 1 })
      ).rejects.toThrow('Network timeout')
    })
  })

  describe('useEnableMultiCredentials', () => {
    it('enables multi-credentials and invalidates queries', async () => {
      vi.mocked(credentialsApi.enableMultiCredentials).mockResolvedValue()

      const { result } = renderHook(() => useEnableMultiCredentials(), {
        wrapper: createWrapper(),
      })

      await result.current.mutateAsync(1)

      expect(credentialsApi.enableMultiCredentials).toHaveBeenCalledWith(1)
    })

    it('handles enable errors', async () => {
      vi.mocked(credentialsApi.enableMultiCredentials).mockRejectedValue(
        new Error('Already enabled')
      )

      const { result } = renderHook(() => useEnableMultiCredentials(), {
        wrapper: createWrapper(),
      })

      await expect(result.current.mutateAsync(1)).rejects.toThrow('Already enabled')
    })
  })
})
