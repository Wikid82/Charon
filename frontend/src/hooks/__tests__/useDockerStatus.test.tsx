import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { renderHook, waitFor } from '@testing-library/react';
import React from 'react';
import { vi, describe, it, expect, beforeEach } from 'vitest';

import { dockerApi } from '../../api/docker';
import { useDockerStatus } from '../useDockerStatus';

vi.mock('../../api/docker', () => ({
  dockerApi: {
    getStatus: vi.fn(),
  },
}));

const createWrapper = () => {
  const queryClient = new QueryClient({ defaultOptions: { queries: { gcTime: 0 } } });
  return ({ children }: { children: React.ReactNode }) =>
    React.createElement(QueryClientProvider, { client: queryClient }, children);
};

describe('useDockerStatus', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('is loading with no data initially', () => {
    vi.mocked(dockerApi.getStatus).mockReturnValue(new Promise(() => {}));
    const { result } = renderHook(() => useDockerStatus(), { wrapper: createWrapper() });

    expect(result.current.isLoading).toBe(true);
    expect(result.current.data).toBeUndefined();
  });

  it('returns the status on success', async () => {
    const status = { available: false, reason: 'socket_missing' as const, message: 'no socket' };
    vi.mocked(dockerApi.getStatus).mockResolvedValue(status);
    const { result } = renderHook(() => useDockerStatus(), { wrapper: createWrapper() });

    await waitFor(() => expect(result.current.data).toEqual(status));
  });

  it('surfaces errors without retrying', async () => {
    vi.mocked(dockerApi.getStatus).mockRejectedValue(new Error('boom'));
    const { result } = renderHook(() => useDockerStatus(), { wrapper: createWrapper() });

    await waitFor(() => expect(result.current.error).toBeTruthy());
    expect(result.current.data).toBeUndefined();
    expect(dockerApi.getStatus).toHaveBeenCalledTimes(1);
  });
});
