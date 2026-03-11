import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { renderHook, waitFor } from '@testing-library/react';
import React from 'react';
import { vi, describe, it, expect, beforeEach } from 'vitest';

import { dockerApi } from '../../api/docker';
import { useDocker } from '../useDocker';

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

  it('does not fetch when both host and serverId are undefined', async () => {
    const { result } = renderHook(() => useDocker(), {
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

  it('extracts details from 503 service unavailable error', async () => {
    const mockError = {
      response: {
        status: 503,
        data: {
          error: 'Docker daemon unavailable',
          details: 'Cannot connect to Docker. Please ensure Docker is running and the socket is accessible (e.g., /var/run/docker.sock is mounted).'
        }
      }
    };
    vi.mocked(dockerApi.listContainers).mockRejectedValue(mockError);

    const { result } = renderHook(() => useDocker('local'), {
      wrapper: createWrapper(),
    });

    await waitFor(
      () => {
        expect(result.current.isLoading).toBe(false);
      },
      { timeout: 3000 }
    );

    // Verify error message includes the details
    expect(result.current.error).toBeTruthy();
    const errorMessage = (result.current.error as Error)?.message;
    expect(errorMessage).toContain('Docker is running');
  });

  it('extracts supplemental-group details from 503 error', async () => {
    const mockError = {
      response: {
        status: 503,
        data: {
          error: 'Docker daemon unavailable',
          details: 'Process groups do not include socket gid 988; run container with matching supplemental group (e.g., --group-add 988).'
        }
      }
    };
    vi.mocked(dockerApi.listContainers).mockRejectedValue(mockError);

    const { result } = renderHook(() => useDocker('local'), {
      wrapper: createWrapper(),
    });

    await waitFor(
      () => {
        expect(result.current.isLoading).toBe(false);
      },
      { timeout: 3000 }
    );

    expect(result.current.error).toBeTruthy();
    const errorMessage = (result.current.error as Error)?.message;
    expect(errorMessage).toContain('--group-add');
    expect(errorMessage).toContain('supplemental group');
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
