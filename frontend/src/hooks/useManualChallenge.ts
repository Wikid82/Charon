import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  getChallenge,
  createChallenge,
  verifyChallenge,
  pollChallenge,
  deleteChallenge,
  type ManualChallenge,
  type CreateChallengeRequest,
  type ChallengePollResponse,
  type ChallengeVerifyResponse,
} from '../api/manualChallenge'

/** Query key factory for manual challenges */
const queryKeys = {
  all: ['manual-challenges'] as const,
  detail: (providerId: number, challengeId: string) =>
    [...queryKeys.all, 'detail', providerId, challengeId] as const,
  poll: (providerId: number, challengeId: string) =>
    [...queryKeys.all, 'poll', providerId, challengeId] as const,
}

/**
 * Hook for fetching a manual challenge by ID.
 * @param providerId - DNS provider ID
 * @param challengeId - Challenge UUID
 * @returns Query result with challenge data
 */
export function useManualChallenge(providerId: number, challengeId: string) {
  return useQuery({
    queryKey: queryKeys.detail(providerId, challengeId),
    queryFn: () => getChallenge(providerId, challengeId),
    enabled: providerId > 0 && !!challengeId,
    staleTime: 1000 * 5, // 5 seconds
  })
}

/**
 * Hook for polling challenge status with automatic refresh.
 * @param providerId - DNS provider ID
 * @param challengeId - Challenge UUID
 * @param enabled - Whether polling is active
 * @param refetchInterval - Polling interval in ms (default 10s)
 * @returns Query result with poll data
 */
export function useChallengePoll(
  providerId: number,
  challengeId: string,
  enabled: boolean = true,
  refetchInterval: number = 10000
) {
  return useQuery({
    queryKey: queryKeys.poll(providerId, challengeId),
    queryFn: () => pollChallenge(providerId, challengeId),
    enabled: enabled && providerId > 0 && !!challengeId,
    refetchInterval: enabled ? refetchInterval : false,
    refetchIntervalInBackground: false,
  })
}

/**
 * Hook providing manual challenge mutation operations.
 * @returns Object with mutation functions for create, verify, and delete
 */
export function useManualChallengeMutations() {
  const queryClient = useQueryClient()

  const createMutation = useMutation({
    mutationFn: ({ providerId, data }: { providerId: number; data: CreateChallengeRequest }) =>
      createChallenge(providerId, data),
  })

  const verifyMutation = useMutation({
    mutationFn: ({ providerId, challengeId }: { providerId: number; challengeId: string }) =>
      verifyChallenge(providerId, challengeId),
    onSuccess: (_, variables) => {
      queryClient.invalidateQueries({
        queryKey: queryKeys.poll(variables.providerId, variables.challengeId),
      })
      queryClient.invalidateQueries({
        queryKey: queryKeys.detail(variables.providerId, variables.challengeId),
      })
    },
  })

  const deleteMutation = useMutation({
    mutationFn: ({ providerId, challengeId }: { providerId: number; challengeId: string }) =>
      deleteChallenge(providerId, challengeId),
    onSuccess: (_, variables) => {
      queryClient.removeQueries({
        queryKey: queryKeys.poll(variables.providerId, variables.challengeId),
      })
      queryClient.removeQueries({
        queryKey: queryKeys.detail(variables.providerId, variables.challengeId),
      })
    },
  })

  return {
    createMutation,
    verifyMutation,
    deleteMutation,
  }
}

export type {
  ManualChallenge,
  CreateChallengeRequest,
  ChallengePollResponse,
  ChallengeVerifyResponse,
}
