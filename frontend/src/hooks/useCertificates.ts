import { useQuery } from '@tanstack/react-query'
import { getCertificates } from '../api/certificates'

export function useCertificates() {
  const { data, isLoading, error } = useQuery({
    queryKey: ['certificates'],
    queryFn: getCertificates,
  })

  return {
    certificates: data || [],
    isLoading,
    error,
  }
}
