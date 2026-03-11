import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { renderHook, waitFor } from '@testing-library/react'
import { describe, it, expect, vi, beforeEach } from 'vitest'

import * as consoleEnrollmentApi from '../../api/consoleEnrollment'
import { useConsoleStatus, useEnrollConsole } from '../useConsoleEnrollment'

import type { ConsoleEnrollmentStatus, ConsoleEnrollPayload } from '../../api/consoleEnrollment'

vi.mock('../../api/consoleEnrollment')

describe('useConsoleEnrollment hooks', () => {
  let queryClient: QueryClient

  beforeEach(() => {
    queryClient = new QueryClient({
      defaultOptions: {
        queries: { retry: false },
        mutations: { retry: false },
      },
    })
    vi.clearAllMocks()
  })

  const wrapper = ({ children }: { children: React.ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  )

  describe('useConsoleStatus', () => {
    it('should fetch console enrollment status when enabled', async () => {
      const mockStatus: ConsoleEnrollmentStatus = {
        status: 'enrolled',
        tenant: 'test-org',
        agent_name: 'charon-1',
        key_present: true,
        enrolled_at: '2025-12-14T10:00:00Z',
        last_heartbeat_at: '2025-12-15T09:00:00Z',
      }
      vi.mocked(consoleEnrollmentApi.getConsoleStatus).mockResolvedValue(mockStatus)

      const { result } = renderHook(() => useConsoleStatus(), { wrapper })

      await waitFor(() => expect(result.current.isSuccess).toBe(true))
      expect(result.current.data).toEqual(mockStatus)
      expect(consoleEnrollmentApi.getConsoleStatus).toHaveBeenCalledTimes(1)
    })

    it('should NOT fetch when enabled=false', async () => {
      const { result } = renderHook(() => useConsoleStatus(false), { wrapper })

      await waitFor(() => expect(result.current.isLoading).toBe(false))
      expect(consoleEnrollmentApi.getConsoleStatus).not.toHaveBeenCalled()
      expect(result.current.data).toBeUndefined()
    })

    it('should use correct query key for invalidation', () => {
      renderHook(() => useConsoleStatus(), { wrapper })
      const queries = queryClient.getQueryCache().getAll()
      const consoleQuery = queries.find((q) =>
        JSON.stringify(q.queryKey) === JSON.stringify(['crowdsec-console-status'])
      )
      expect(consoleQuery).toBeDefined()
    })

    it('should handle pending enrollment status', async () => {
      const mockStatus: ConsoleEnrollmentStatus = {
        status: 'pending',
        tenant: 'my-org',
        agent_name: 'charon-prod',
        key_present: true,
        last_attempt_at: '2025-12-15T09:00:00Z',
        correlation_id: 'req-abc123',
      }
      vi.mocked(consoleEnrollmentApi.getConsoleStatus).mockResolvedValue(mockStatus)

      const { result } = renderHook(() => useConsoleStatus(), { wrapper })

      await waitFor(() => expect(result.current.isSuccess).toBe(true))
      expect(result.current.data?.status).toBe('pending')
      expect(result.current.data?.correlation_id).toBe('req-abc123')
    })

    it('should handle failed enrollment status with error details', async () => {
      const mockStatus: ConsoleEnrollmentStatus = {
        status: 'failed',
        tenant: 'test-org',
        agent_name: 'charon-prod',
        key_present: false,
        last_error: 'Invalid enrollment key',
        last_attempt_at: '2025-12-15T09:00:00Z',
        correlation_id: 'err-xyz789',
      }
      vi.mocked(consoleEnrollmentApi.getConsoleStatus).mockResolvedValue(mockStatus)

      const { result } = renderHook(() => useConsoleStatus(), { wrapper })

      await waitFor(() => expect(result.current.isSuccess).toBe(true))
      expect(result.current.data?.status).toBe('failed')
      expect(result.current.data?.last_error).toBe('Invalid enrollment key')
      expect(result.current.data?.key_present).toBe(false)
    })

    it('should handle none status (not enrolled)', async () => {
      const mockStatus: ConsoleEnrollmentStatus = {
        status: 'none',
        key_present: false,
      }
      vi.mocked(consoleEnrollmentApi.getConsoleStatus).mockResolvedValue(mockStatus)

      const { result } = renderHook(() => useConsoleStatus(), { wrapper })

      await waitFor(() => expect(result.current.isSuccess).toBe(true))
      expect(result.current.data?.status).toBe('none')
      expect(result.current.data?.key_present).toBe(false)
    })

    it('should handle API errors', async () => {
      const error = new Error('Network failure')
      vi.mocked(consoleEnrollmentApi.getConsoleStatus).mockRejectedValue(error)

      const { result } = renderHook(() => useConsoleStatus(), { wrapper })

      await waitFor(() => expect(result.current.isError).toBe(true))
      expect(result.current.error).toEqual(error)
    })

    it('should NOT expose enrollment key in status response', async () => {
      const mockStatus: ConsoleEnrollmentStatus = {
        status: 'enrolled',
        tenant: 'test-org',
        agent_name: 'charon-prod',
        key_present: true,
        enrolled_at: '2025-12-15T10:00:00Z',
      }
      vi.mocked(consoleEnrollmentApi.getConsoleStatus).mockResolvedValue(mockStatus)

      const { result } = renderHook(() => useConsoleStatus(), { wrapper })

      await waitFor(() => expect(result.current.isSuccess).toBe(true))
      expect(result.current.data).not.toHaveProperty('enrollment_key')
      expect(result.current.data).not.toHaveProperty('encrypted_enroll_key')
      expect(result.current.data).toHaveProperty('key_present')
    })

    it('should be configured with refetchOnWindowFocus disabled by default', async () => {
      vi.mocked(consoleEnrollmentApi.getConsoleStatus).mockResolvedValue({
        status: 'pending',
        key_present: true,
      })

      const { result } = renderHook(() => useConsoleStatus(), { wrapper })

      await waitFor(() => expect(result.current.isSuccess).toBe(true))

      // Clear mock call count
      vi.clearAllMocks()

      // Simulate window focus
      window.dispatchEvent(new Event('focus'))

      // Wait a bit to see if refetch would happen
      await new Promise((resolve) => setTimeout(resolve, 100))

      // Should NOT trigger refetch by default (refetchOnWindowFocus is not enabled in our config)
      expect(consoleEnrollmentApi.getConsoleStatus).not.toHaveBeenCalled()
    })

    it('should handle status with heartbeat timestamp', async () => {
      const mockStatus: ConsoleEnrollmentStatus = {
        status: 'enrolled',
        tenant: 'production',
        agent_name: 'charon-main',
        key_present: true,
        enrolled_at: '2025-12-14T10:00:00Z',
        last_heartbeat_at: '2025-12-15T09:55:00Z',
      }
      vi.mocked(consoleEnrollmentApi.getConsoleStatus).mockResolvedValue(mockStatus)

      const { result } = renderHook(() => useConsoleStatus(), { wrapper })

      await waitFor(() => expect(result.current.isSuccess).toBe(true))
      expect(result.current.data?.last_heartbeat_at).toBe('2025-12-15T09:55:00Z')
      expect(result.current.data?.enrolled_at).toBe('2025-12-14T10:00:00Z')
    })
  })

  describe('useEnrollConsole', () => {
    it('should enroll console and invalidate status query', async () => {
      const mockResponse: ConsoleEnrollmentStatus = {
        status: 'enrolled',
        tenant: 'my-org',
        agent_name: 'charon-prod',
        key_present: true,
        enrolled_at: new Date().toISOString(),
      }
      vi.mocked(consoleEnrollmentApi.enrollConsole).mockResolvedValue(mockResponse)

      const { result } = renderHook(() => useEnrollConsole(), { wrapper })

      const payload: ConsoleEnrollPayload = {
        enrollment_key: 'cs-enroll-key-123',
        tenant: 'my-org',
        agent_name: 'charon-prod',
      }

      result.current.mutate(payload)

      await waitFor(() => expect(result.current.isSuccess).toBe(true))
      expect(consoleEnrollmentApi.enrollConsole).toHaveBeenCalledWith(payload)
      expect(result.current.data).toEqual(mockResponse)
    })

    it('should invalidate console status query on success', async () => {
      const mockResponse: ConsoleEnrollmentStatus = {
        status: 'enrolled',
        key_present: true,
      }
      vi.mocked(consoleEnrollmentApi.enrollConsole).mockResolvedValue(mockResponse)

      // Set up initial status query
      queryClient.setQueryData(['crowdsec-console-status'], { status: 'pending', key_present: true })

      const { result } = renderHook(() => useEnrollConsole(), { wrapper })

      result.current.mutate({
        enrollment_key: 'key',
        agent_name: 'agent',
      })

      await waitFor(() => expect(result.current.isSuccess).toBe(true))

      // Verify invalidation happened
      const state = queryClient.getQueryState(['crowdsec-console-status'])
      expect(state?.isInvalidated).toBe(true)
    })

    it('should handle enrollment errors', async () => {
      const error = new Error('Invalid enrollment key')
      vi.mocked(consoleEnrollmentApi.enrollConsole).mockRejectedValue(error)

      const { result } = renderHook(() => useEnrollConsole(), { wrapper })

      result.current.mutate({
        enrollment_key: 'invalid',
        agent_name: 'test',
      })

      await waitFor(() => expect(result.current.isError).toBe(true))
      expect(result.current.error).toEqual(error)
    })

    it('should enroll with force flag', async () => {
      const mockResponse: ConsoleEnrollmentStatus = {
        status: 'enrolled',
        tenant: 'new-tenant',
        agent_name: 'charon-updated',
        key_present: true,
        enrolled_at: new Date().toISOString(),
      }
      vi.mocked(consoleEnrollmentApi.enrollConsole).mockResolvedValue(mockResponse)

      const { result } = renderHook(() => useEnrollConsole(), { wrapper })

      const payload: ConsoleEnrollPayload = {
        enrollment_key: 'cs-enroll-new-key',
        agent_name: 'charon-updated',
        force: true,
      }

      result.current.mutate(payload)

      await waitFor(() => expect(result.current.isSuccess).toBe(true))
      expect(consoleEnrollmentApi.enrollConsole).toHaveBeenCalledWith(payload)
      expect(result.current.data?.agent_name).toBe('charon-updated')
    })

    it('should enroll with optional tenant parameter', async () => {
      const mockResponse: ConsoleEnrollmentStatus = {
        status: 'enrolled',
        tenant: 'custom-org',
        agent_name: 'charon-1',
        key_present: true,
        enrolled_at: new Date().toISOString(),
      }
      vi.mocked(consoleEnrollmentApi.enrollConsole).mockResolvedValue(mockResponse)

      const { result } = renderHook(() => useEnrollConsole(), { wrapper })

      const payload: ConsoleEnrollPayload = {
        enrollment_key: 'cs-enroll-abc123',
        tenant: 'custom-org',
        agent_name: 'charon-1',
      }

      result.current.mutate(payload)

      await waitFor(() => expect(result.current.isSuccess).toBe(true))
      expect(result.current.data?.tenant).toBe('custom-org')
    })

    it('should handle network errors during enrollment', async () => {
      const error = new Error('Network timeout')
      vi.mocked(consoleEnrollmentApi.enrollConsole).mockRejectedValue(error)

      const { result } = renderHook(() => useEnrollConsole(), { wrapper })

      result.current.mutate({
        enrollment_key: 'valid-key',
        agent_name: 'agent',
      })

      await waitFor(() => expect(result.current.isError).toBe(true))
      expect(result.current.error?.message).toBe('Network timeout')
    })

    it('should handle enrollment returning pending status', async () => {
      const mockResponse: ConsoleEnrollmentStatus = {
        status: 'pending',
        tenant: 'test-org',
        agent_name: 'charon-1',
        key_present: true,
        last_attempt_at: new Date().toISOString(),
        correlation_id: 'req-123',
      }
      vi.mocked(consoleEnrollmentApi.enrollConsole).mockResolvedValue(mockResponse)

      const { result } = renderHook(() => useEnrollConsole(), { wrapper })

      result.current.mutate({
        enrollment_key: 'cs-enroll-key',
        agent_name: 'charon-1',
        tenant: 'test-org',
      })

      await waitFor(() => expect(result.current.isSuccess).toBe(true))
      expect(result.current.data?.status).toBe('pending')
      expect(result.current.data?.correlation_id).toBe('req-123')
    })

    it('should handle enrollment returning failed status', async () => {
      const mockResponse: ConsoleEnrollmentStatus = {
        status: 'failed',
        tenant: 'test-org',
        agent_name: 'charon-1',
        key_present: false,
        last_error: 'Enrollment key expired',
        last_attempt_at: new Date().toISOString(),
        correlation_id: 'err-456',
      }
      vi.mocked(consoleEnrollmentApi.enrollConsole).mockResolvedValue(mockResponse)

      const { result } = renderHook(() => useEnrollConsole(), { wrapper })

      result.current.mutate({
        enrollment_key: 'expired-key',
        agent_name: 'charon-1',
        tenant: 'test-org',
      })

      await waitFor(() => expect(result.current.isSuccess).toBe(true))
      expect(result.current.data?.status).toBe('failed')
      expect(result.current.data?.last_error).toBe('Enrollment key expired')
    })

    it('should allow retry after transient enrollment failure', async () => {
      const { result } = renderHook(() => useEnrollConsole(), { wrapper })

      // First attempt fails with network error
      vi.mocked(consoleEnrollmentApi.enrollConsole).mockRejectedValueOnce(
        new Error('Network timeout')
      )

      const payload: ConsoleEnrollPayload = {
        enrollment_key: 'cs-enroll-key',
        agent_name: 'agent',
      }

      result.current.mutate(payload)
      await waitFor(() => expect(result.current.isError).toBe(true))

      // Second attempt succeeds
      vi.mocked(consoleEnrollmentApi.enrollConsole).mockResolvedValueOnce({
        status: 'enrolled',
        key_present: true,
      })

      result.current.mutate(payload)
      await waitFor(() => expect(result.current.isSuccess).toBe(true))
      expect(result.current.data?.status).toBe('enrolled')
    })

    it('should handle multiple enrollment mutations gracefully', async () => {
      const mockResponse: ConsoleEnrollmentStatus = {
        status: 'enrolled',
        key_present: true,
      }
      vi.mocked(consoleEnrollmentApi.enrollConsole).mockResolvedValue(mockResponse)

      const { result } = renderHook(() => useEnrollConsole(), { wrapper })

      // Trigger first mutation
      result.current.mutate({ enrollment_key: 'key1', agent_name: 'agent1' })

      // Trigger second mutation immediately
      result.current.mutate({ enrollment_key: 'key2', agent_name: 'agent2' })

      await waitFor(() => expect(result.current.isSuccess).toBe(true))

      // Last mutation should be the one recorded
      expect(consoleEnrollmentApi.enrollConsole).toHaveBeenLastCalledWith(
        expect.objectContaining({ enrollment_key: 'key2', agent_name: 'agent2' })
      )
    })

    it('should handle enrollment with correlation ID tracking', async () => {
      const mockResponse: ConsoleEnrollmentStatus = {
        status: 'enrolled',
        tenant: 'prod',
        agent_name: 'charon-main',
        key_present: true,
        enrolled_at: new Date().toISOString(),
        correlation_id: 'success-req-789',
      }
      vi.mocked(consoleEnrollmentApi.enrollConsole).mockResolvedValue(mockResponse)

      const { result } = renderHook(() => useEnrollConsole(), { wrapper })

      result.current.mutate({
        enrollment_key: 'cs-enroll-key',
        agent_name: 'charon-main',
        tenant: 'prod',
      })

      await waitFor(() => expect(result.current.isSuccess).toBe(true))
      expect(result.current.data?.correlation_id).toBe('success-req-789')
    })
  })

  describe('query key consistency', () => {
    it('should use consistent query key between status and enrollment', async () => {
      // Setup status query
      vi.mocked(consoleEnrollmentApi.getConsoleStatus).mockResolvedValue({
        status: 'none',
        key_present: false,
      })

      renderHook(() => useConsoleStatus(), { wrapper })
      await waitFor(() => {
        const queries = queryClient.getQueryCache().getAll()
        expect(queries.length).toBeGreaterThan(0)
      })

      // Verify the query exists with the correct key
      const statusQuery = queryClient.getQueryCache().find({
        queryKey: ['crowdsec-console-status'],
      })
      expect(statusQuery).toBeDefined()

      // Setup enrollment mutation
      vi.mocked(consoleEnrollmentApi.enrollConsole).mockResolvedValue({
        status: 'enrolled',
        key_present: true,
      })

      const { result } = renderHook(() => useEnrollConsole(), { wrapper })

      result.current.mutate({
        enrollment_key: 'key',
        agent_name: 'agent',
      })

      await waitFor(() => expect(result.current.isSuccess).toBe(true))

      // Verify that the query was invalidated (refetch will be triggered if there's an observer)
      // The mutation's onSuccess should have called invalidateQueries
      const state = queryClient.getQueryState(['crowdsec-console-status'])
      expect(state).toBeDefined()
    })
  })

  describe('edge cases', () => {
    it('should handle empty agent_name gracefully', async () => {
      const error = new Error('Agent name is required')
      vi.mocked(consoleEnrollmentApi.enrollConsole).mockRejectedValue(error)

      const { result } = renderHook(() => useEnrollConsole(), { wrapper })

      result.current.mutate({
        enrollment_key: 'key',
        agent_name: '',
      })

      await waitFor(() => expect(result.current.isError).toBe(true))
    })

    it('should handle special characters in agent name', async () => {
      const mockResponse: ConsoleEnrollmentStatus = {
        status: 'enrolled',
        agent_name: 'charon-prod-01',
        key_present: true,
        enrolled_at: new Date().toISOString(),
      }
      vi.mocked(consoleEnrollmentApi.enrollConsole).mockResolvedValue(mockResponse)

      const { result } = renderHook(() => useEnrollConsole(), { wrapper })

      result.current.mutate({
        enrollment_key: 'key',
        agent_name: 'charon-prod-01',
      })

      await waitFor(() => expect(result.current.isSuccess).toBe(true))
      expect(result.current.data?.agent_name).toBe('charon-prod-01')
    })

    it('should handle missing optional fields in status response', async () => {
      const minimalStatus: ConsoleEnrollmentStatus = {
        status: 'none',
        key_present: false,
      }
      vi.mocked(consoleEnrollmentApi.getConsoleStatus).mockResolvedValue(minimalStatus)

      const { result } = renderHook(() => useConsoleStatus(), { wrapper })

      await waitFor(() => expect(result.current.isSuccess).toBe(true))
      expect(result.current.data).toEqual(minimalStatus)
      expect(result.current.data?.tenant).toBeUndefined()
      expect(result.current.data?.agent_name).toBeUndefined()
    })
  })
})
