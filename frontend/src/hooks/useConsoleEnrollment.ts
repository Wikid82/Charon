import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { isAxiosError } from 'axios'
import { useTranslation } from 'react-i18next'

import { enrollConsole, getConsoleStatus, clearConsoleEnrollment, type ConsoleEnrollPayload, type ConsoleEnrollmentStatus } from '../api/consoleEnrollment'
import { sanitizeSecret } from '../utils/sanitizeSecret'
import { toast } from '../utils/toast'

export function useConsoleStatus(enabled = true) {
  return useQuery<ConsoleEnrollmentStatus>({ queryKey: ['crowdsec-console-status'], queryFn: getConsoleStatus, enabled })
}

export function useEnrollConsole() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (payload: ConsoleEnrollPayload) => enrollConsole(payload),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['crowdsec-console-status'] })
    },
  })
}

export function useClearConsoleEnrollment() {
  const queryClient = useQueryClient()
  const { t } = useTranslation()

  return useMutation({
    mutationFn: clearConsoleEnrollment,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['crowdsec-console-status'] })
    },
    onError: (err: unknown) => {
      const detail = isAxiosError(err) ? err.response?.data?.error || err.message : err instanceof Error ? err.message : ''
      toast.error(t('crowdsecConfig.reenroll.clearFailed', { message: sanitizeSecret(String(detail)) }))
    },
  })
}
