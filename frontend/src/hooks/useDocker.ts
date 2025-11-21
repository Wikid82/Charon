import { useQuery } from '@tanstack/react-query'
import { dockerApi } from '../api/docker'

export function useDocker(host?: string) {
  const {
    data: containers = [],
    isLoading,
    error,
    refetch,
  } = useQuery({
    queryKey: ['docker-containers', host],
    queryFn: () => dockerApi.listContainers(host),
    retry: 1, // Don't retry too much if docker is not available
  })

  return {
    containers,
    isLoading,
    error,
    refetch,
  }
}
