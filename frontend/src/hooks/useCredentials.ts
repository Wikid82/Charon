import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'

import {
  getCredentials,
  getCredential,
  createCredential,
  updateCredential,
  deleteCredential,
  testCredential,
  enableMultiCredentials,
  type DNSProviderCredential,
  type CredentialRequest,
  type CredentialTestResult,
} from '../api/credentials'

/** Query key factory for credentials */
export const credentialQueryKeys = {
  all: ['credentials'] as const,
  byProvider: (providerId: number) => [...credentialQueryKeys.all, 'provider', providerId] as const,
  detail: (providerId: number, credentialId: number) =>
    [...credentialQueryKeys.all, 'provider', providerId, 'detail', credentialId] as const,
}

/**
 * Hook for fetching all credentials for a DNS provider.
 * @param providerId - DNS provider ID
 * @returns Query result with credentials array
 */
export function useCredentials(providerId: number) {
  return useQuery({
    queryKey: credentialQueryKeys.byProvider(providerId),
    queryFn: () => getCredentials(providerId),
    enabled: providerId > 0,
  })
}

/**
 * Hook for fetching a single credential.
 * @param providerId - DNS provider ID
 * @param credentialId - Credential ID
 * @returns Query result with credential data
 */
export function useCredential(providerId: number, credentialId: number) {
  return useQuery({
    queryKey: credentialQueryKeys.detail(providerId, credentialId),
    queryFn: () => getCredential(providerId, credentialId),
    enabled: providerId > 0 && credentialId > 0,
  })
}

/**
 * Hook for creating a new credential.
 * @returns Mutation function for creating credentials
 */
export function useCreateCredential() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: ({ providerId, data }: { providerId: number; data: CredentialRequest }) =>
      createCredential(providerId, data),
    onSuccess: (_, variables) => {
      queryClient.invalidateQueries({
        queryKey: credentialQueryKeys.byProvider(variables.providerId),
      })
    },
  })
}

/**
 * Hook for updating an existing credential.
 * @returns Mutation function for updating credentials
 */
export function useUpdateCredential() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: ({
      providerId,
      credentialId,
      data,
    }: {
      providerId: number
      credentialId: number
      data: CredentialRequest
    }) => updateCredential(providerId, credentialId, data),
    onSuccess: (_, variables) => {
      queryClient.invalidateQueries({
        queryKey: credentialQueryKeys.byProvider(variables.providerId),
      })
      queryClient.invalidateQueries({
        queryKey: credentialQueryKeys.detail(variables.providerId, variables.credentialId),
      })
    },
  })
}

/**
 * Hook for deleting a credential.
 * @returns Mutation function for deleting credentials
 */
export function useDeleteCredential() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: ({ providerId, credentialId }: { providerId: number; credentialId: number }) =>
      deleteCredential(providerId, credentialId),
    onSuccess: (_, variables) => {
      queryClient.invalidateQueries({
        queryKey: credentialQueryKeys.byProvider(variables.providerId),
      })
    },
  })
}

/**
 * Hook for testing a credential.
 * @returns Mutation function for testing credentials
 */
export function useTestCredential() {
  return useMutation({
    mutationFn: ({ providerId, credentialId }: { providerId: number; credentialId: number }) =>
      testCredential(providerId, credentialId),
  })
}

/**
 * Hook for enabling multi-credential mode.
 * @returns Mutation function for enabling multi-credential mode
 */
export function useEnableMultiCredentials() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (providerId: number) => enableMultiCredentials(providerId),
    onSuccess: (_, providerId) => {
      // Invalidate DNS provider queries to refresh use_multi_credentials flag
      queryClient.invalidateQueries({ queryKey: ['dns-providers'] })
      queryClient.invalidateQueries({
        queryKey: credentialQueryKeys.byProvider(providerId),
      })
    },
  })
}

export type {
  DNSProviderCredential,
  CredentialRequest,
  CredentialTestResult,
}
