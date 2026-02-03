import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  getPlugins,
  getPlugin,
  enablePlugin,
  disablePlugin,
  reloadPlugins,
  type PluginInfo,
} from '../api/plugins'

/** Query key factory for plugins */
const queryKeys = {
  all: ['plugins'] as const,
  lists: () => [...queryKeys.all, 'list'] as const,
  list: () => [...queryKeys.lists()] as const,
  details: () => [...queryKeys.all, 'detail'] as const,
  detail: (id: number) => [...queryKeys.details(), id] as const,
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

export type { PluginInfo }
