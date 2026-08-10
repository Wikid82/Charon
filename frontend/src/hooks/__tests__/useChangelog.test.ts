import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { renderHook, waitFor } from '@testing-library/react';
import { createElement, type ReactNode } from 'react';
import { describe, it, expect, vi, beforeEach } from 'vitest';

import * as changelogApi from '../../api/changelog';
import {
  useChangelogStatus,
  useChangelogAll,
  useAckChangelog,
  useOptInChangelog,
} from '../useChangelog';

vi.mock('../../api/changelog', async () => {
  const actual = await vi.importActual('../../api/changelog');
  return {
    ...actual,
    getChangelogStatus: vi.fn(),
    getChangelogAll: vi.fn(),
    ackChangelog: vi.fn(),
    optInChangelog: vi.fn(),
  };
});

vi.mock('../../utils/toast', () => ({
  toast: {
    success: vi.fn(),
    error: vi.fn(),
  },
}));

const sampleStatus: changelogApi.ChangelogStatus = {
  show_changelog: true,
  versions: [
    {
      version: '1.2.0',
      date: '2026-07-01',
      features: ['New thing'],
      fixes: [],
      other: [],
      security: [],
    },
  ],
};

const sampleAll: changelogApi.ChangelogAll = {
  versions: [
    {
      version: '1.2.0',
      date: '2026-07-01',
      features: ['New thing'],
      fixes: [],
      other: [],
      security: [],
    },
    {
      version: '1.1.0',
      date: '2026-06-01',
      features: [],
      fixes: ['Old fix'],
      other: [],
      security: [],
    },
  ],
};

describe('useChangelog hooks', () => {
  let queryClient: QueryClient;

  const createWrapper = () => {
    return ({ children }: { children: ReactNode }) =>
      createElement(QueryClientProvider, { client: queryClient }, children);
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

  describe('useChangelogStatus', () => {
    it('fetches changelog status', async () => {
      vi.mocked(changelogApi.getChangelogStatus).mockResolvedValue(sampleStatus);

      const { result } = renderHook(() => useChangelogStatus(), {
        wrapper: createWrapper(),
      });

      await waitFor(() => expect(result.current.isSuccess).toBe(true));

      expect(result.current.data).toEqual(sampleStatus);
      expect(changelogApi.getChangelogStatus).toHaveBeenCalledTimes(1);
    });

    it('surfaces fetch errors via isError', async () => {
      vi.mocked(changelogApi.getChangelogStatus).mockRejectedValue(new Error('Network error'));

      const { result } = renderHook(() => useChangelogStatus(), {
        wrapper: createWrapper(),
      });

      await waitFor(() => expect(result.current.isError).toBe(true));
    });
  });

  describe('useChangelogAll', () => {
    it('does not fire the query when enabled=false', async () => {
      const { result } = renderHook(() => useChangelogAll(false), {
        wrapper: createWrapper(),
      });

      expect(result.current.fetchStatus).toBe('idle');
      expect(changelogApi.getChangelogAll).not.toHaveBeenCalled();
    });

    it('fires the query when enabled=true', async () => {
      vi.mocked(changelogApi.getChangelogAll).mockResolvedValue(sampleAll);

      const { result } = renderHook(() => useChangelogAll(true), {
        wrapper: createWrapper(),
      });

      await waitFor(() => expect(result.current.isSuccess).toBe(true));

      expect(result.current.data).toEqual(sampleAll);
      expect(changelogApi.getChangelogAll).toHaveBeenCalledTimes(1);
    });
  });

  describe('useAckChangelog', () => {
    it('invalidates changelog-status on success', async () => {
      vi.mocked(changelogApi.ackChangelog).mockResolvedValue(undefined);
      const invalidateSpy = vi.spyOn(queryClient, 'invalidateQueries');

      const { result } = renderHook(() => useAckChangelog(), {
        wrapper: createWrapper(),
      });

      result.current.mutate({ action: 'dismiss_temporary', opt_out: false });

      await waitFor(() => expect(result.current.isSuccess).toBe(true));

      expect(changelogApi.ackChangelog).toHaveBeenCalledWith({
        action: 'dismiss_temporary',
        opt_out: false,
      });
      expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ['changelog-status'] });
    });

    it('shows an error toast on failure', async () => {
      const toast = await import('../../utils/toast');
      vi.mocked(changelogApi.ackChangelog).mockRejectedValue(new Error('boom'));

      const { result } = renderHook(() => useAckChangelog(), {
        wrapper: createWrapper(),
      });

      result.current.mutate({ action: 'dismiss_permanent', opt_out: true });

      await waitFor(() => expect(result.current.isError).toBe(true));

      expect(toast.toast.error).toHaveBeenCalledWith('Failed to update changelog preferences');
    });
  });

  describe('useOptInChangelog', () => {
    it('invalidates changelog-status on success', async () => {
      vi.mocked(changelogApi.optInChangelog).mockResolvedValue(undefined);
      const invalidateSpy = vi.spyOn(queryClient, 'invalidateQueries');

      const { result } = renderHook(() => useOptInChangelog(), {
        wrapper: createWrapper(),
      });

      result.current.mutate();

      await waitFor(() => expect(result.current.isSuccess).toBe(true));

      expect(changelogApi.optInChangelog).toHaveBeenCalledTimes(1);
      expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ['changelog-status'] });
    });

    it('shows an error toast on failure', async () => {
      const toast = await import('../../utils/toast');
      vi.mocked(changelogApi.optInChangelog).mockRejectedValue(new Error('boom'));

      const { result } = renderHook(() => useOptInChangelog(), {
        wrapper: createWrapper(),
      });

      result.current.mutate();

      await waitFor(() => expect(result.current.isError).toBe(true));

      expect(toast.toast.error).toHaveBeenCalledWith('Failed to re-enable update notifications');
    });
  });
});
