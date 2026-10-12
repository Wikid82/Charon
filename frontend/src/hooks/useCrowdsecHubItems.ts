import { useMutation, useQueryClient } from '@tanstack/react-query'

import { restoreHubItems, type HubItem } from '../api/crowdsec'

/**
 * Installs missing hub items. Errors are surfaced inline by the caller
 * (no toast: the restore is an optional follow-up to an import).
 */
export const useRestoreHubItems = () => {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (items: HubItem[]) => restoreHubItems(items),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['crowdsec-files'] })
    },
  })
}
