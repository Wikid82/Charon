import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import * as api from '../api/domains'

export function useDomains() {
  const queryClient = useQueryClient()

  const { data: domains = [], isLoading, error } = useQuery({
    queryKey: ['domains'],
    queryFn: api.getDomains,
  })

  const createMutation = useMutation({
    mutationFn: api.createDomain,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['domains'] })
    },
  })

  const deleteMutation = useMutation({
    mutationFn: api.deleteDomain,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['domains'] })
    },
  })

  return {
    domains,
    isLoading,
    error,
    createDomain: createMutation.mutateAsync,
    deleteDomain: deleteMutation.mutateAsync,
  }
}
