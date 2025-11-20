import { useQuery } from '@tanstack/react-query'
import { getCertificates } from '../api/certificates'

export function useCertificates() {
  const { data: certificates = [], isLoading, error } = useQuery({
    queryKey: ['certificates'],
    queryFn: getCertificates,
  })

  return {
    certificates,
    isLoading,
    error,
  }
}
