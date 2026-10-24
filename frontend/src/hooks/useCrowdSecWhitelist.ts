import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import type { TFunction } from 'i18next'
import { useTranslation } from 'react-i18next'

import { listWhitelists, addWhitelist, deleteWhitelist, type AddWhitelistPayload } from '../api/crowdsec'
import { crowdsecErrorMessage } from '../api/errorBody'
import { toast } from '../utils/toast'

/** Text for a failed whitelist add: the server's message when it is an Error, otherwise the generic fallback. */
export const whitelistAddErrorMessage = (err: unknown, t: TFunction): string =>
  err instanceof Error ? err.message : t('crowdsecConfig.whitelist.addFailed')

export const useWhitelistEntries = () =>
  useQuery({
    queryKey: ['crowdsec-whitelist'],
    queryFn: listWhitelists,
  })

export const useAddWhitelist = () => {
  const queryClient = useQueryClient()
  const { t } = useTranslation()
  return useMutation({
    mutationFn: (data: AddWhitelistPayload) => addWhitelist(data),
    onSuccess: () => {
      toast.success(t('crowdsecConfig.whitelist.added'))
      queryClient.invalidateQueries({ queryKey: ['crowdsec-whitelist'] })
    },
    onError: (err: unknown) => {
      toast.error(crowdsecErrorMessage(err, t, whitelistAddErrorMessage(err, t)))
    },
  })
}

export const useDeleteWhitelist = () => {
  const queryClient = useQueryClient()
  const { t } = useTranslation()
  return useMutation({
    mutationFn: (uuid: string) => deleteWhitelist(uuid),
    onSuccess: () => {
      toast.success(t('crowdsecConfig.whitelist.removed'))
      queryClient.invalidateQueries({ queryKey: ['crowdsec-whitelist'] })
    },
    onError: (err: unknown) => {
      toast.error(crowdsecErrorMessage(err, t, t('crowdsecConfig.whitelist.removeFailed')))
    },
  })
}
