import { renderHook, waitFor, act } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { vi, describe, it, expect, beforeEach } from 'vitest';
import { useDomains } from '../useDomains';
import * as api from '../../api/domains';
import React from 'react';

vi.mock('../../api/domains', () => ({
  getDomains: vi.fn(),
  createDomain: vi.fn(),
  deleteDomain: vi.fn(),
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

describe('useDomains', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  const mockDomains = [
    { id: 1, uuid: 'uuid-1', name: 'example.com', created_at: '2024-01-01T00:00:00Z' },
    { id: 2, uuid: 'uuid-2', name: 'test.com', created_at: '2024-01-02T00:00:00Z' },
  ];

  it('fetches domains on mount', async () => {
    vi.mocked(api.getDomains).mockResolvedValue(mockDomains);

    const { result } = renderHook(() => useDomains(), {
      wrapper: createWrapper(),
    });

    expect(result.current.isLoading).toBe(true);

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    expect(api.getDomains).toHaveBeenCalled();
    expect(result.current.domains).toEqual(mockDomains);
  });

  it('returns empty array as default', async () => {
    vi.mocked(api.getDomains).mockResolvedValue([]);

    const { result } = renderHook(() => useDomains(), {
      wrapper: createWrapper(),
    });

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    expect(result.current.domains).toEqual([]);
  });

  it('creates a new domain', async () => {
    vi.mocked(api.getDomains).mockResolvedValue(mockDomains);
    vi.mocked(api.createDomain).mockResolvedValue({
      id: 3,
      uuid: 'uuid-3',
      name: 'new.com',
      created_at: '2024-01-03T00:00:00Z',
    });

    const { result } = renderHook(() => useDomains(), {
      wrapper: createWrapper(),
    });

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    await act(async () => {
      await result.current.createDomain('new.com');
    });

    // Check that createDomain was called with the correct first argument
    expect(api.createDomain).toHaveBeenCalled();
    expect(vi.mocked(api.createDomain).mock.calls[0][0]).toBe('new.com');
  });

  it('deletes a domain', async () => {
    vi.mocked(api.getDomains).mockResolvedValue(mockDomains);
    vi.mocked(api.deleteDomain).mockResolvedValue(undefined);

    const { result } = renderHook(() => useDomains(), {
      wrapper: createWrapper(),
    });

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    await act(async () => {
      await result.current.deleteDomain('uuid-1');
    });

    // Check that deleteDomain was called with the correct first argument
    expect(api.deleteDomain).toHaveBeenCalled();
    expect(vi.mocked(api.deleteDomain).mock.calls[0][0]).toBe('uuid-1');
  });

  it('handles API errors', async () => {
    vi.mocked(api.getDomains).mockRejectedValue(new Error('API Error'));

    const { result } = renderHook(() => useDomains(), {
      wrapper: createWrapper(),
    });

    await waitFor(() => {
      expect(result.current.error).toBeTruthy();
    });

    expect(result.current.domains).toEqual([]);
  });

  it('provides isFetching state', async () => {
    vi.mocked(api.getDomains).mockResolvedValue(mockDomains);

    const { result } = renderHook(() => useDomains(), {
      wrapper: createWrapper(),
    });

    // Initially fetching
    expect(result.current.isFetching).toBe(true);

    await waitFor(() => {
      expect(result.current.isFetching).toBe(false);
    });
  });
});
