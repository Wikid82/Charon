import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  detectDNSProvider,
  getDetectionPatterns,
  type DetectionResult,
  type NameserverPattern,
} from '../api/dnsDetection'

/** Query key factory for DNS detection */
const queryKeys = {
  all: ['dns-detection'] as const,
  results: () => [...queryKeys.all, 'results'] as const,
  result: (domain: string) => [...queryKeys.results(), domain] as const,
  patterns: () => [...queryKeys.all, 'patterns'] as const,
}

/**
 * Hook for detecting DNS provider for a domain.
 * Results are cached for 1 hour per domain.
 * @returns Mutation function with detection result
 */
export function useDetectDNSProvider() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (domain: string) => detectDNSProvider(domain),
    onSuccess: (result, domain) => {
      // Cache result for 1 hour
      queryClient.setQueryData(queryKeys.result(domain), result, {
        updatedAt: Date.now(),
      })
    },
  })
}

/**
 * Hook for fetching cached detection result for a domain.
 * @param domain - Domain to get cached result for
 * @returns Query result with detection data
 */
export function useCachedDetectionResult(domain: string, enabled = false) {
  return useQuery({
    queryKey: queryKeys.result(domain),
    queryFn: () => detectDNSProvider(domain),
    enabled: enabled && !!domain,
    staleTime: 60 * 60 * 1000, // 1 hour
    gcTime: 60 * 60 * 1000, // Keep cached for 1 hour
  })
}

/**
 * Hook for fetching nameserver detection patterns.
 * Patterns are cached for 24 hours as they rarely change.
 * @returns Query result with patterns array
 */
export function useDetectionPatterns() {
  return useQuery({
    queryKey: queryKeys.patterns(),
    queryFn: getDetectionPatterns,
    staleTime: 24 * 60 * 60 * 1000, // 24 hours
    gcTime: 24 * 60 * 60 * 1000,
  })
}

export type { DetectionResult, NameserverPattern }
