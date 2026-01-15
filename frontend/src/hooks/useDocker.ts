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
    queryFn: async () => {
      try {
        return await dockerApi.listContainers(host || undefined, serverId || undefined)
      } catch (err: any) {
        // Extract helpful error message from response
        if (err.response?.status === 503) {
          const details = err.response?.data?.details
          const message = details || 'Docker service unavailable. Check that Docker is running.'
          throw new Error(message)
        }
        throw err
      }
    },
    enabled: Boolean(host) || Boolean(serverId),
    retry: 1, // Don't retry too much if docker is not available
  })

  return {
    containers,
    isLoading,
    error,
    refetch,
  }
}
