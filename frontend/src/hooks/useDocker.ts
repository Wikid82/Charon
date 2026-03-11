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
      } catch (err: unknown) {
        // Extract helpful error message from response
        const error = err as { response?: { status?: number; data?: { details?: string } } }
        if (error.response?.status === 503) {
          const details = error.response?.data?.details
          const message = details || 'Docker service unavailable. Check that Docker is running.'
          throw new Error(message, {
            cause: err,
          })
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
