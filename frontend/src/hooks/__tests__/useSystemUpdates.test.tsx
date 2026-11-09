import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { act, renderHook, waitFor } from '@testing-library/react';
import React from 'react';
import { vi, describe, it, expect, beforeEach } from 'vitest';

import { checkUpdates, type UpdateInfo } from '../../api/system';
import { SYSTEM_UPDATES_QUERY_KEY, useSystemUpdates } from '../useSystemUpdates';

vi.mock('../../api/system', () => ({
  checkUpdates: vi.fn(),
}));

const updateInfo: UpdateInfo = {
  current_version: '0.46.5',
  available: true,
  latest_version: '0.46.6',
  changelog_url: 'https://github.com/Wikid82/Charon/releases/tag/v0.46.6',
};

const createClient = () => new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: 0 } } });

const createWrapper = (queryClient: QueryClient) => {
  return ({ children }: { children: React.ReactNode }) =>
    React.createElement(QueryClientProvider, { client: queryClient }, children);
};

describe('useSystemUpdates', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(checkUpdates).mockResolvedValue(updateInfo);
  });

  it('uses the shared system-updates query key', () => {
    expect(SYSTEM_UPDATES_QUERY_KEY).toEqual(['system-updates']);
  });

  it('fetches automatically by default', async () => {
    const queryClient = createClient();
    const { result } = renderHook(() => useSystemUpdates(), { wrapper: createWrapper(queryClient) });

    await waitFor(() => expect(result.current.data).toEqual(updateInfo));
    expect(checkUpdates).toHaveBeenCalledTimes(1);
  });

  it('fetches automatically when enabled is explicitly true', async () => {
    const queryClient = createClient();
    const { result } = renderHook(() => useSystemUpdates({ enabled: true }), {
      wrapper: createWrapper(queryClient),
    });

    await waitFor(() => expect(result.current.data).toEqual(updateInfo));
    expect(checkUpdates).toHaveBeenCalledTimes(1);
  });

  it('does not fetch on mount when enabled is false', () => {
    const queryClient = createClient();
    const { result } = renderHook(() => useSystemUpdates({ enabled: false }), {
      wrapper: createWrapper(queryClient),
    });

    expect(checkUpdates).not.toHaveBeenCalled();
    expect(result.current.data).toBeUndefined();
    expect(result.current.fetchStatus).toBe('idle');
  });

  it('fetches when the manual refetch is invoked in enabled:false mode', async () => {
    const queryClient = createClient();
    const { result } = renderHook(() => useSystemUpdates({ enabled: false }), {
      wrapper: createWrapper(queryClient),
    });

    act(() => {
      void result.current.refetch();
    });

    await waitFor(() => expect(result.current.data).toEqual(updateInfo));
    expect(checkUpdates).toHaveBeenCalledTimes(1);
  });

  it('reuses data cached by an automatic consumer without refetching when enabled is false', async () => {
    const queryClient = createClient();
    const { result: autoResult } = renderHook(() => useSystemUpdates(), { wrapper: createWrapper(queryClient) });
    await waitFor(() => expect(autoResult.current.data).toEqual(updateInfo));

    const { result: manualResult } = renderHook(() => useSystemUpdates({ enabled: false }), {
      wrapper: createWrapper(queryClient),
    });

    expect(manualResult.current.data).toEqual(updateInfo);
    expect(checkUpdates).toHaveBeenCalledTimes(1);
  });

  it('surfaces errors from checkUpdates', async () => {
    vi.mocked(checkUpdates).mockRejectedValue(new Error('network down'));
    const queryClient = createClient();
    const { result } = renderHook(() => useSystemUpdates(), { wrapper: createWrapper(queryClient) });

    await waitFor(() => expect(result.current.error).toBeTruthy());
    expect(result.current.data).toBeUndefined();
  });
});
