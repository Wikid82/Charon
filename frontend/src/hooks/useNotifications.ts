import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';

import {
  getSecurityNotificationSettings,
  updateSecurityNotificationSettings,
  type SecurityNotificationSettings,
} from '../api/notifications';
import { toast } from '../utils/toast';

export function useSecurityNotificationSettings() {
  return useQuery({
    queryKey: ['security-notification-settings'],
    queryFn: getSecurityNotificationSettings,
  });
}

export function useUpdateSecurityNotificationSettings() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (settings: Partial<SecurityNotificationSettings>) =>
      updateSecurityNotificationSettings(settings),
    onMutate: async (newSettings) => {
      // Cancel any outgoing refetches
      await queryClient.cancelQueries({ queryKey: ['security-notification-settings'] });

      // Snapshot the previous value
      const previousSettings = queryClient.getQueryData(['security-notification-settings']);

      // Optimistically update to the new value
      queryClient.setQueryData(['security-notification-settings'], (old: unknown) => {
        if (old && typeof old === 'object') {
          return { ...old, ...newSettings };
        }
        return old;
      });

      return { previousSettings };
    },
    onError: (err, _newSettings, context) => {
      // Rollback on error
      if (context?.previousSettings) {
        queryClient.setQueryData(['security-notification-settings'], context.previousSettings);
      }
      const message = err instanceof Error ? err.message : 'Failed to update notification settings';
      toast.error(message);
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['security-notification-settings'] });
      toast.success('Notification settings updated');
    },
  });
}
