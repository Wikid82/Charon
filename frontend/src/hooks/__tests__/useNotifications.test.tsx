import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { renderHook, waitFor } from '@testing-library/react';
import { type ReactNode } from 'react';
import { describe, it, expect, vi, beforeEach } from 'vitest';

import * as notificationsApi from '../../api/notifications';
import {
  useSecurityNotificationSettings,
  useUpdateSecurityNotificationSettings,
} from '../useNotifications';

// Mock the API
vi.mock('../../api/notifications', async () => {
  const actual = await vi.importActual('../../api/notifications');
  return {
    ...actual,
    getSecurityNotificationSettings: vi.fn(),
    updateSecurityNotificationSettings: vi.fn(),
  };
});

// Mock toast
vi.mock('../../utils/toast', () => ({
  toast: {
    success: vi.fn(),
    error: vi.fn(),
  },
}));

describe('useNotifications hooks', () => {
  let queryClient: QueryClient;

  const createWrapper = () => {
    return ({ children }: { children: ReactNode }) => (
      <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
    );
  };

  beforeEach(() => {
    queryClient = new QueryClient({
      defaultOptions: {
        queries: { retry: false },
        mutations: { retry: false },
      },
    });
    vi.clearAllMocks();
  });

  describe('useSecurityNotificationSettings', () => {
    it('fetches security notification settings', async () => {
      const mockSettings: notificationsApi.SecurityNotificationSettings = {
        enabled: true,
        min_log_level: 'warn',
        security_waf_enabled: true,
        security_acl_enabled: true,
        security_rate_limit_enabled: false,
      };

      vi.mocked(notificationsApi.getSecurityNotificationSettings).mockResolvedValue(mockSettings);

      const { result } = renderHook(() => useSecurityNotificationSettings(), {
        wrapper: createWrapper(),
      });

      await waitFor(() => expect(result.current.isSuccess).toBe(true));

      expect(result.current.data).toEqual(mockSettings);
      expect(notificationsApi.getSecurityNotificationSettings).toHaveBeenCalledTimes(1);
    });

    it('handles fetch errors', async () => {
      vi.mocked(notificationsApi.getSecurityNotificationSettings).mockRejectedValue(
        new Error('Network error')
      );

      const { result } = renderHook(() => useSecurityNotificationSettings(), {
        wrapper: createWrapper(),
      });

      await waitFor(() => expect(result.current.isError).toBe(true));

      expect(result.current.error).toBeTruthy();
    });
  });

  describe('useUpdateSecurityNotificationSettings', () => {
    const mockSettings: notificationsApi.SecurityNotificationSettings = {
      enabled: true,
      min_log_level: 'warn',
      security_waf_enabled: true,
      security_acl_enabled: true,
      security_rate_limit_enabled: false,
    };

    beforeEach(() => {
      vi.mocked(notificationsApi.getSecurityNotificationSettings).mockResolvedValue(mockSettings);
    });

    it('updates security notification settings', async () => {
      const updatedSettings = { ...mockSettings, min_log_level: 'error' };
      vi.mocked(notificationsApi.updateSecurityNotificationSettings).mockResolvedValue(
        updatedSettings
      );

      const { result } = renderHook(() => useUpdateSecurityNotificationSettings(), {
        wrapper: createWrapper(),
      });

      result.current.mutate({ min_log_level: 'error' });

      await waitFor(() => expect(result.current.isSuccess).toBe(true));

      expect(notificationsApi.updateSecurityNotificationSettings).toHaveBeenCalledWith({
        min_log_level: 'error',
      });
    });

    it('performs optimistic update', async () => {
      const updatedSettings = { ...mockSettings, enabled: false };
      vi.mocked(notificationsApi.updateSecurityNotificationSettings).mockResolvedValue(
        updatedSettings
      );

      // Pre-populate cache
      queryClient.setQueryData(['security-notification-settings'], mockSettings);

      const { result } = renderHook(() => useUpdateSecurityNotificationSettings(), {
        wrapper: createWrapper(),
      });

      result.current.mutate({ enabled: false });

      // Wait a bit for the optimistic update to take effect
      await waitFor(() => {
        const cachedData = queryClient.getQueryData(['security-notification-settings']);
        expect(cachedData).toMatchObject({ enabled: false });
      });

      await waitFor(() => expect(result.current.isSuccess).toBe(true));
    });

    it('rolls back on error', async () => {
      vi.mocked(notificationsApi.updateSecurityNotificationSettings).mockRejectedValue(
        new Error('Update failed')
      );

      // Pre-populate cache
      queryClient.setQueryData(['security-notification-settings'], mockSettings);

      const { result } = renderHook(() => useUpdateSecurityNotificationSettings(), {
        wrapper: createWrapper(),
      });

      result.current.mutate({ enabled: false });

      await waitFor(() => expect(result.current.isError).toBe(true));

      // Check that original data is restored
      const cachedData = queryClient.getQueryData(['security-notification-settings']);
      expect(cachedData).toEqual(mockSettings);
    });

    it('shows success toast on successful update', async () => {
      const toast = await import('../../utils/toast');
      const updatedSettings = { ...mockSettings, min_log_level: 'error' };
      vi.mocked(notificationsApi.updateSecurityNotificationSettings).mockResolvedValue(
        updatedSettings
      );

      const { result } = renderHook(() => useUpdateSecurityNotificationSettings(), {
        wrapper: createWrapper(),
      });

      result.current.mutate({ min_log_level: 'error' });

      await waitFor(() => expect(result.current.isSuccess).toBe(true));

      expect(toast.toast.success).toHaveBeenCalledWith('Notification settings updated');
    });

    it('shows error toast on failed update', async () => {
      const toast = await import('../../utils/toast');
      vi.mocked(notificationsApi.updateSecurityNotificationSettings).mockRejectedValue(
        new Error('Update failed')
      );

      const { result } = renderHook(() => useUpdateSecurityNotificationSettings(), {
        wrapper: createWrapper(),
      });

      result.current.mutate({ enabled: false });

      await waitFor(() => expect(result.current.isError).toBe(true));

      expect(toast.toast.error).toHaveBeenCalledWith('Update failed');
    });

    it('invalidates queries on success', async () => {
      const updatedSettings = { ...mockSettings, min_log_level: 'error' };
      vi.mocked(notificationsApi.updateSecurityNotificationSettings).mockResolvedValue(
        updatedSettings
      );

      const invalidateSpy = vi.spyOn(queryClient, 'invalidateQueries');

      const { result } = renderHook(() => useUpdateSecurityNotificationSettings(), {
        wrapper: createWrapper(),
      });

      result.current.mutate({ min_log_level: 'error' });

      await waitFor(() => expect(result.current.isSuccess).toBe(true));

      expect(invalidateSpy).toHaveBeenCalledWith({
        queryKey: ['security-notification-settings'],
      });
    });

    it('handles updates with multiple fields', async () => {
      const updatedSettings = {
        ...mockSettings,
        enabled: false,
        min_log_level: 'error',
        security_waf_enabled: false,
      };

      vi.mocked(notificationsApi.updateSecurityNotificationSettings).mockResolvedValue(
        updatedSettings
      );

      const { result } = renderHook(() => useUpdateSecurityNotificationSettings(), {
        wrapper: createWrapper(),
      });

      result.current.mutate({
        enabled: false,
        min_log_level: 'error',
        security_waf_enabled: false,
      });

      await waitFor(() => expect(result.current.isSuccess).toBe(true));

      expect(notificationsApi.updateSecurityNotificationSettings).toHaveBeenCalledWith({
        enabled: false,
        min_log_level: 'error',
        security_waf_enabled: false,
      });
    });
  });
});
