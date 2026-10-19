import { useQuery } from '@tanstack/react-query'

import { dockerApi } from '../api/docker'

/** Reports local Docker socket availability; callers must treat errors as "unknown" (fail open). */
export function useDockerStatus() {
  return useQuery({
    queryKey: ['docker-status'],
    queryFn: dockerApi.getStatus,
    staleTime: 30_000,
    retry: false,
    refetchOnWindowFocus: false,
  })
}
