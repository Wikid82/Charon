import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'

import {
  getEncryptionStatus,
  rotateEncryptionKey,
  getRotationHistory,
  validateKeyConfiguration,
  type RotationStatus,
  type RotationResult,
  type RotationHistoryEntry,
  type KeyValidationResult,
} from '../api/encryption'

/** Query key factory for encryption management */
const queryKeys = {
  all: ['encryption'] as const,
  status: () => [...queryKeys.all, 'status'] as const,
  history: () => [...queryKeys.all, 'history'] as const,
}

/**
 * Hook for fetching encryption status with auto-refresh.
 * @param refetchInterval - Milliseconds between refetches (default: 5000ms during rotation)
 * @returns Query result with status data
 */
export function useEncryptionStatus(refetchInterval?: number) {
  return useQuery({
    queryKey: queryKeys.status(),
    queryFn: getEncryptionStatus,
    refetchInterval: refetchInterval || false,
    staleTime: 30000, // 30 seconds
  })
}

/**
 * Hook for fetching rotation audit history.
 * @returns Query result with history array
 */
export function useRotationHistory() {
  return useQuery({
    queryKey: queryKeys.history(),
    queryFn: getRotationHistory,
    staleTime: 60000, // 1 minute
  })
}

/**
 * Hook providing key rotation mutation.
 * @returns Mutation object for triggering key rotation
 */
export function useRotateKey() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: rotateEncryptionKey,
    onSuccess: () => {
      // Invalidate status and history to refresh UI
      queryClient.invalidateQueries({ queryKey: queryKeys.status() })
      queryClient.invalidateQueries({ queryKey: queryKeys.history() })
    },
  })
}

/**
 * Hook providing key validation mutation.
 * @returns Mutation object for validating key configuration
 */
export function useValidateKeys() {
  return useMutation({
    mutationFn: validateKeyConfiguration,
  })
}

export type {
  RotationStatus,
  RotationResult,
  RotationHistoryEntry,
  KeyValidationResult,
}
