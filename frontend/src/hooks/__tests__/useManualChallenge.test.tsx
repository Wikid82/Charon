import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { renderHook, waitFor, act } from '@testing-library/react'
import { describe, it, expect, vi, beforeEach } from 'vitest'

import * as api from '../../api/manualChallenge'
import { useManualChallenge, useChallengePoll, useManualChallengeMutations } from '../useManualChallenge'

vi.mock('../../api/manualChallenge')

const createWrapper = () => {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false },
    },
  })
  return ({ children }: { children: React.ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  )
}

describe('useManualChallenge hooks', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  describe('useManualChallenge', () => {
    it('fetches challenge by ID when enabled', async () => {
      const mockChallenge = {
        id: 'test-uuid',
        status: 'pending' as const,
        fqdn: '_acme-challenge.example.com',
        value: 'test-value',
        ttl: 300,
        created_at: '2026-01-11T00:00:00Z',
        expires_at: '2026-01-11T00:10:00Z',
        dns_propagated: false,
      }

      vi.mocked(api.getChallenge).mockResolvedValueOnce(mockChallenge)

      const { result } = renderHook(
        () => useManualChallenge(1, 'test-uuid'),
        { wrapper: createWrapper() }
      )

      await waitFor(() => expect(result.current.isSuccess).toBe(true))

      expect(api.getChallenge).toHaveBeenCalledWith(1, 'test-uuid')
      expect(result.current.data).toEqual(mockChallenge)
    })

    it('does not fetch when providerId is 0', async () => {
      const { result } = renderHook(
        () => useManualChallenge(0, 'test-uuid'),
        { wrapper: createWrapper() }
      )

      await waitFor(() => expect(result.current.isLoading).toBe(false))

      expect(api.getChallenge).not.toHaveBeenCalled()
    })

    it('does not fetch when challengeId is empty', async () => {
      const { result } = renderHook(
        () => useManualChallenge(1, ''),
        { wrapper: createWrapper() }
      )

      await waitFor(() => expect(result.current.isLoading).toBe(false))

      expect(api.getChallenge).not.toHaveBeenCalled()
    })
  })

  describe('useChallengePoll', () => {
    it('fetches poll data when enabled', async () => {
      const mockPoll = {
        status: 'pending' as const,
        dns_propagated: false,
        time_remaining_seconds: 480,
        last_check_at: '2026-01-11T00:02:00Z',
      }

      vi.mocked(api.pollChallenge).mockResolvedValue(mockPoll)

      const { result } = renderHook(
        () => useChallengePoll(1, 'test-uuid', true),
        { wrapper: createWrapper() }
      )

      await waitFor(() => expect(result.current.isSuccess).toBe(true))

      expect(api.pollChallenge).toHaveBeenCalledWith(1, 'test-uuid')
      expect(result.current.data).toEqual(mockPoll)
    })

    it('does not fetch when disabled', async () => {
      const { result } = renderHook(
        () => useChallengePoll(1, 'test-uuid', false),
        { wrapper: createWrapper() }
      )

      await waitFor(() => expect(result.current.isLoading).toBe(false))

      expect(api.pollChallenge).not.toHaveBeenCalled()
    })

    it('uses custom refetch interval', async () => {
      const mockPoll = {
        status: 'pending' as const,
        dns_propagated: false,
        time_remaining_seconds: 480,
        last_check_at: '2026-01-11T00:02:00Z',
      }

      vi.mocked(api.pollChallenge).mockResolvedValue(mockPoll)

      const { result } = renderHook(
        () => useChallengePoll(1, 'test-uuid', true, 5000),
        { wrapper: createWrapper() }
      )

      await waitFor(() => expect(result.current.isSuccess).toBe(true))

      // Hook configured with 5 second interval
      expect(result.current.data).toEqual(mockPoll)
    })
  })

  describe('useManualChallengeMutations', () => {
    describe('createMutation', () => {
      it('creates a new challenge', async () => {
        const mockChallenge = {
          id: 'new-uuid',
          status: 'created' as const,
          fqdn: '_acme-challenge.example.com',
          value: 'generated-value',
          ttl: 300,
          created_at: '2026-01-11T00:00:00Z',
          expires_at: '2026-01-11T00:10:00Z',
          dns_propagated: false,
        }

        vi.mocked(api.createChallenge).mockResolvedValueOnce(mockChallenge)

        const { result } = renderHook(
          () => useManualChallengeMutations(),
          { wrapper: createWrapper() }
        )

        await act(async () => {
          const response = await result.current.createMutation.mutateAsync({
            providerId: 1,
            data: { domain: 'example.com' },
          })
          expect(response).toEqual(mockChallenge)
        })

        expect(api.createChallenge).toHaveBeenCalledWith(1, { domain: 'example.com' })
      })
    })

    describe('verifyMutation', () => {
      it('verifies a challenge', async () => {
        const mockResult = {
          success: true,
          dns_found: true,
          message: 'TXT record verified',
        }

        vi.mocked(api.verifyChallenge).mockResolvedValueOnce(mockResult)

        const { result } = renderHook(
          () => useManualChallengeMutations(),
          { wrapper: createWrapper() }
        )

        await act(async () => {
          const response = await result.current.verifyMutation.mutateAsync({
            providerId: 1,
            challengeId: 'test-uuid',
          })
          expect(response).toEqual(mockResult)
        })

        expect(api.verifyChallenge).toHaveBeenCalledWith(1, 'test-uuid')
      })

      it('handles verification failure', async () => {
        vi.mocked(api.verifyChallenge).mockRejectedValueOnce(new Error('Network error'))

        const { result } = renderHook(
          () => useManualChallengeMutations(),
          { wrapper: createWrapper() }
        )

        await expect(
          act(() =>
            result.current.verifyMutation.mutateAsync({
              providerId: 1,
              challengeId: 'test-uuid',
            })
          )
        ).rejects.toThrow('Network error')
      })
    })

    describe('deleteMutation', () => {
      it('deletes a challenge', async () => {
        vi.mocked(api.deleteChallenge).mockResolvedValueOnce()

        const { result } = renderHook(
          () => useManualChallengeMutations(),
          { wrapper: createWrapper() }
        )

        await act(async () => {
          await result.current.deleteMutation.mutateAsync({
            providerId: 1,
            challengeId: 'test-uuid',
          })
        })

        expect(api.deleteChallenge).toHaveBeenCalledWith(1, 'test-uuid')
      })

      it('handles deletion failure', async () => {
        vi.mocked(api.deleteChallenge).mockRejectedValueOnce(
          new Error('Challenge not found')
        )

        const { result } = renderHook(
          () => useManualChallengeMutations(),
          { wrapper: createWrapper() }
        )

        await expect(
          act(() =>
            result.current.deleteMutation.mutateAsync({
              providerId: 1,
              challengeId: 'invalid-uuid',
            })
          )
        ).rejects.toThrow('Challenge not found')
      })
    })
  })
})
