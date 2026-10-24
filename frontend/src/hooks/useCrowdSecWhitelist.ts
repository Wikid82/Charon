import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { isAxiosError } from 'axios'
import type { TFunction } from 'i18next'
import { useTranslation } from 'react-i18next'

import { listWhitelists, addWhitelist, deleteWhitelist, type AddWhitelistPayload } from '../api/crowdsec'
import { crowdsecErrorMessage, isCrowdsecBusyError } from '../api/errorBody'
import { toast } from '../utils/toast'

/**
 * True when a failed whitelist add is a problem with the submitted entry, which belongs on the IP field:
 * a 400 validation error or a 409 duplicate. A busy refusal (409 crowdsec_busy) is not a field problem,
 * and transport or server failures are never field problems.
 */
export const isWhitelistFieldError = (err: unknown): boolean => {
  if (!isAxiosError(err) || isCrowdsecBusyError(err)) return false
  const status = err.response?.status
  return status === 400 || status === 409
}

/**
 * Text for a failed whitelist add. Field errors (see isWhitelistFieldError) keep the server message;
 * every other Axios failure gets the translated fallback so raw transport text is never shown.
 * Non-Axios errors keep their message, and anything else gets the fallback.
 */
export const whitelistAddErrorMessage = (err: unknown, t: TFunction): string => {
  const fallback = t('crowdsecConfig.whitelist.addFailed')
  if (isAxiosError(err)) return isWhitelistFieldError(err) ? err.message : fallback
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
