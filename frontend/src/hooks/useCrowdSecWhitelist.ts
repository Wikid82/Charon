import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'

import { listWhitelists, addWhitelist, deleteWhitelist, type AddWhitelistPayload } from '../api/crowdsec'
import { toast } from '../utils/toast'

export const useWhitelistEntries = () =>
  useQuery({
    queryKey: ['crowdsec-whitelist'],
    queryFn: listWhitelists,
  })

export const useAddWhitelist = () => {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (data: AddWhitelistPayload) => addWhitelist(data),
    onSuccess: () => {
      toast.success('Whitelist entry added')
      queryClient.invalidateQueries({ queryKey: ['crowdsec-whitelist'] })
    },
    onError: (err: unknown) => {
      toast.error(err instanceof Error ? err.message : 'Failed to add whitelist entry')
    },
  })
}

export const useDeleteWhitelist = () => {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (uuid: string) => deleteWhitelist(uuid),
    onSuccess: () => {
      toast.success('Whitelist entry removed')
      queryClient.invalidateQueries({ queryKey: ['crowdsec-whitelist'] })
    },
    onError: (err: unknown) => {
      toast.error(err instanceof Error ? err.message : 'Failed to remove whitelist entry')
    },
  })
}
