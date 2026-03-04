import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  getDNSProviders,
  getDNSProvider,
  getDNSProviderTypes,
  createDNSProvider,
  updateDNSProvider,
  deleteDNSProvider,
  testDNSProvider,
  testDNSProviderCredentials,
  type DNSProvider,
  type DNSProviderRequest,
  type DNSProviderTypeInfo,
  type DNSProviderField,
  type DNSTestResult,
} from '../api/dnsProviders'

/** Query key factory for DNS providers */
const queryKeys = {
  all: ['dns-providers'] as const,
  lists: () => [...queryKeys.all, 'list'] as const,
  list: () => [...queryKeys.lists()] as const,
  details: () => [...queryKeys.all, 'detail'] as const,
  detail: (id: number) => [...queryKeys.details(), id] as const,
  types: () => [...queryKeys.all, 'types'] as const,
}

/**
 * Hook for fetching all DNS providers.
 * @returns Query result with providers array
 */
export function useDNSProviders() {
  return useQuery({
    queryKey: queryKeys.list(),
    queryFn: getDNSProviders,
  })
}

/**
 * Hook for fetching a single DNS provider.
 * @param id - DNS provider ID
 * @returns Query result with provider data
 */
export function useDNSProvider(id: number) {
  return useQuery({
    queryKey: queryKeys.detail(id),
    queryFn: () => getDNSProvider(id),
    enabled: id > 0,
  })
}

/**
 * Hook for fetching supported DNS provider types.
 * @returns Query result with provider types array
 */
export function useDNSProviderTypes() {
  return useQuery({
    queryKey: queryKeys.types(),
    queryFn: getDNSProviderTypes,
    staleTime: 1000 * 60 * 60, // 1 hour - types rarely change
  })
}

/**
 * Hook providing DNS provider mutation operations.
 * @returns Object with mutation functions for create, update, delete, and test
 */
export function useDNSProviderMutations() {
  const queryClient = useQueryClient()

  const createMutation = useMutation({
    mutationFn: (data: DNSProviderRequest) => createDNSProvider(data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.list() })
    },
  })

  const updateMutation = useMutation({
    mutationFn: ({ id, data }: { id: number; data: DNSProviderRequest }) =>
      updateDNSProvider(id, data),
    onSuccess: (_, variables) => {
      queryClient.invalidateQueries({ queryKey: queryKeys.list() })
      queryClient.invalidateQueries({ queryKey: queryKeys.detail(variables.id) })
    },
  })

  const deleteMutation = useMutation({
    mutationFn: (id: number) => deleteDNSProvider(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.list() })
    },
  })

  const testMutation = useMutation({
    mutationFn: (id: number) => testDNSProvider(id),
  })

  const testCredentialsMutation = useMutation({
    mutationFn: (data: DNSProviderRequest) => testDNSProviderCredentials(data),
  })

  return {
    createMutation,
    updateMutation,
    deleteMutation,
    testMutation,
    testCredentialsMutation,
  }
}

export type {
  DNSProvider,
  DNSProviderRequest,
  DNSProviderTypeInfo,
  DNSProviderField,
  DNSTestResult,
}
