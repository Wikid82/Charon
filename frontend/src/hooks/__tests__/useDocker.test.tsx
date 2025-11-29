import { renderHook, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { vi, describe, it, expect, beforeEach } from 'vitest';
import { useDocker } from '../useDocker';
import { dockerApi } from '../../api/docker';
import React from 'react';

vi.mock('../../api/docker', () => ({
  dockerApi: {
    listContainers: vi.fn(),
  },
}));

const createWrapper = () => {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: {
        retry: false,
        gcTime: 0,
      },
    },
  });
  return ({ children }: { children: React.ReactNode }) =>
    React.createElement(QueryClientProvider, { client: queryClient }, children);
};

describe('useDocker', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  const mockContainers = [
    {
      id: 'abc123',
      names: ['/nginx'],
      image: 'nginx:latest',
      state: 'running',
      status: 'Up 2 hours',
      network: 'bridge',
      ip: '172.17.0.2',
      ports: [{ private_port: 80, public_port: 8080, type: 'tcp' }],
    },
  ];

  it('fetches containers when host is provided', async () => {
    vi.mocked(dockerApi.listContainers).mockResolvedValue(mockContainers);

    const { result } = renderHook(() => useDocker('192.168.1.100'), {
      wrapper: createWrapper(),
    });

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    expect(dockerApi.listContainers).toHaveBeenCalledWith('192.168.1.100', undefined);
    expect(result.current.containers).toEqual(mockContainers);
  });

  it('fetches containers when serverId is provided', async () => {
    vi.mocked(dockerApi.listContainers).mockResolvedValue(mockContainers);

    const { result } = renderHook(() => useDocker(undefined, 'server-123'), {
      wrapper: createWrapper(),
    });

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    expect(dockerApi.listContainers).toHaveBeenCalledWith(undefined, 'server-123');
    expect(result.current.containers).toEqual(mockContainers);
  });

  it('does not fetch when both host and serverId are null', async () => {
    const { result } = renderHook(() => useDocker(null, null), {
      wrapper: createWrapper(),
    });

    expect(dockerApi.listContainers).not.toHaveBeenCalled();
    expect(result.current.containers).toEqual([]);
  });

  it('returns empty array as default when no data', async () => {
    vi.mocked(dockerApi.listContainers).mockResolvedValue([]);

    const { result } = renderHook(() => useDocker('localhost'), {
      wrapper: createWrapper(),
    });

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    expect(result.current.containers).toEqual([]);
  });

  it('handles API errors', async () => {
    vi.mocked(dockerApi.listContainers).mockRejectedValue(new Error('Docker not available'));

    const { result } = renderHook(() => useDocker('localhost'), {
      wrapper: createWrapper(),
    });

    // Wait for the query to complete (with retry)
    await waitFor(
      () => {
        expect(result.current.isLoading).toBe(false);
      },
      { timeout: 3000 }
    );

    // After retries, containers should still be empty array
    expect(result.current.containers).toEqual([]);
  });

  it('provides refetch function', async () => {
    vi.mocked(dockerApi.listContainers).mockResolvedValue(mockContainers);

    const { result } = renderHook(() => useDocker('localhost'), {
      wrapper: createWrapper(),
    });

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    expect(typeof result.current.refetch).toBe('function');

    // Call refetch
    await result.current.refetch();

    expect(dockerApi.listContainers).toHaveBeenCalledTimes(2);
  });
});
