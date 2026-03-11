import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'

import { enrollConsole, getConsoleStatus, clearConsoleEnrollment, type ConsoleEnrollPayload, type ConsoleEnrollmentStatus } from '../api/consoleEnrollment'

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

  return useMutation({
    mutationFn: clearConsoleEnrollment,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['crowdsec-console-status'] })
    },
  })
}
