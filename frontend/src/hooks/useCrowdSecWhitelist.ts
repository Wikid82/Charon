import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { isAxiosError } from 'axios'
import type { TFunction } from 'i18next'
import { useTranslation } from 'react-i18next'

import { listWhitelists, addWhitelist, deleteWhitelist, type AddWhitelistPayload } from '../api/crowdsec'
import { crowdsecErrorMessage } from '../api/errorBody'
import { toast } from '../utils/toast'

/**
 * Text for a failed whitelist add. A validation response (400) keeps its message for the inline IP field;
 * every other Axios failure gets the translated fallback so raw transport text is never shown.
 * Other errors keep their message, and anything else gets the fallback.
 */
export const whitelistAddErrorMessage = (err: unknown, t: TFunction): string => {
  const fallback = t('crowdsecConfig.whitelist.addFailed')
  if (isAxiosError(err)) return err.response?.status === 400 ? err.message : fallback
  return err instanceof Error ? err.message : fallback
}

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
