import { useQuery } from '@tanstack/react-query'
import { dockerApi } from '../api/docker'

export function useDocker(host?: string | null, serverId?: string | null) {
  const {
    data: containers = [],
    isLoading,
    error,
    refetch,
  } = useQuery({
    queryKey: ['docker-containers', host, serverId],
    queryFn: () => dockerApi.listContainers(host || undefined, serverId || undefined),
    enabled: host !== null || serverId !== null, // Disable if both are explicitly null/undefined
    retry: 1, // Don't retry too much if docker is not available
  })

  return {
    containers,
    isLoading,
    error,
    refetch,
  }
}
