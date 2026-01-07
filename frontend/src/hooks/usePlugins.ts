import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  getPlugins,
  getPlugin,
  enablePlugin,
  disablePlugin,
  reloadPlugins,
  getProviderFields,
  type PluginInfo,
  type ProviderFieldsResponse,
} from '../api/plugins'

/** Query key factory for plugins */
const queryKeys = {
  all: ['plugins'] as const,
  lists: () => [...queryKeys.all, 'list'] as const,
  list: () => [...queryKeys.lists()] as const,
  details: () => [...queryKeys.all, 'detail'] as const,
  detail: (id: number) => [...queryKeys.details(), id] as const,
  providerFields: (type: string) => ['dns-providers', 'fields', type] as const,
}

/**
 * Hook for fetching all plugins.
 * @returns Query result with plugins array
 */
export function usePlugins() {
  return useQuery({
    queryKey: queryKeys.list(),
    queryFn: getPlugins,
  })
}

/**
 * Hook for fetching a single plugin.
 * @param id - Plugin ID
 * @returns Query result with plugin data
 */
export function usePlugin(id: number) {
  return useQuery({
    queryKey: queryKeys.detail(id),
    queryFn: () => getPlugin(id),
    enabled: id > 0,
  })
}

/**
 * Hook for fetching provider credential field definitions.
 * @param providerType - Provider type identifier
 * @returns Query result with field specifications
 */
export function useProviderFields(providerType: string) {
  return useQuery({
    queryKey: queryKeys.providerFields(providerType),
    queryFn: () => getProviderFields(providerType),
    enabled: !!providerType,
    staleTime: 1000 * 60 * 60, // 1 hour - field definitions rarely change
  })
}

/**
 * Hook for enabling a plugin.
 * @returns Mutation function for enabling plugins
 */
export function useEnablePlugin() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (id: number) => enablePlugin(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.list() })
    },
  })
}

/**
 * Hook for disabling a plugin.
 * @returns Mutation function for disabling plugins
 */
export function useDisablePlugin() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (id: number) => disablePlugin(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.list() })
    },
  })
}

/**
 * Hook for reloading all plugins.
 * @returns Mutation function for reloading plugins
 */
export function useReloadPlugins() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: reloadPlugins,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.list() })
    },
  })
}

export type { PluginInfo, ProviderFieldsResponse }
