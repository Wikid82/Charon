import { useQuery } from '@tanstack/react-query'
import { getCertificates } from '../api/certificates'

interface UseCertificatesOptions {
  refetchInterval?: number | false
}

export function useCertificates(options?: UseCertificatesOptions) {
  const { data, isLoading, error, refetch } = useQuery({
    queryKey: ['certificates'],
    queryFn: getCertificates,
    refetchInterval: options?.refetchInterval,
  })

  return {
    certificates: data || [],
    isLoading,
    error,
    refetch,
  }
}
