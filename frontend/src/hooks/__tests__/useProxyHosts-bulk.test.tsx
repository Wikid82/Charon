import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { renderHook, waitFor } from '@testing-library/react';
import { describe, it, expect, vi, beforeEach } from 'vitest';

import * as proxyHostsApi from '../../api/proxyHosts';
import { useProxyHosts } from '../useProxyHosts';

// Mock the API module
vi.mock('../../api/proxyHosts');

const createWrapper = () => {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false },
    },
  });
  return ({ children }: { children: React.ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );
};

describe('useProxyHosts bulk operations', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe('bulkUpdateACL', () => {
    it('should apply ACL to multiple hosts', async () => {
      const mockResponse = {
        updated: 2,
        errors: [],
      };
      vi.mocked(proxyHostsApi.getProxyHosts).mockResolvedValue([]);
      vi.mocked(proxyHostsApi.bulkUpdateACL).mockResolvedValue(mockResponse);

      const { result } = renderHook(() => useProxyHosts(), {
        wrapper: createWrapper(),
      });

      await waitFor(() => expect(result.current.loading).toBe(false));

      const hostUUIDs = ['uuid-1', 'uuid-2'];
      const accessListID = 5;

      const response = await result.current.bulkUpdateACL(hostUUIDs, accessListID);

      expect(proxyHostsApi.bulkUpdateACL).toHaveBeenCalledWith(hostUUIDs, accessListID);
      expect(response.updated).toBe(2);
      expect(response.errors).toEqual([]);
    });

    it('should remove ACL from hosts', async () => {
      const mockResponse = {
        updated: 1,
        errors: [],
      };
      vi.mocked(proxyHostsApi.getProxyHosts).mockResolvedValue([]);
      vi.mocked(proxyHostsApi.bulkUpdateACL).mockResolvedValue(mockResponse);

      const { result } = renderHook(() => useProxyHosts(), {
        wrapper: createWrapper(),
      });

      await waitFor(() => expect(result.current.loading).toBe(false));

      const response = await result.current.bulkUpdateACL(['uuid-1'], null);

      expect(proxyHostsApi.bulkUpdateACL).toHaveBeenCalledWith(['uuid-1'], null);
      expect(response.updated).toBe(1);
    });

    it('should invalidate queries after successful bulk update', async () => {
      const mockHosts = [
        {
          uuid: 'uuid-1',
          name: 'Host 1',
          domain_names: 'host1.example.com',
          forward_scheme: 'http',
          forward_host: 'localhost',
          forward_port: 8001,
          ssl_forced: false,
          http2_support: false,
          hsts_enabled: false,
          hsts_subdomains: false,
          block_exploits: true,
          websocket_support: false,
          application: 'none' as const,
          locations: [],
          enabled: true,
          access_list_id: null,
          certificate_id: null,
          created_at: '2025-01-01T00:00:00Z',
          updated_at: '2025-01-01T00:00:00Z',
        },
      ];

      vi.mocked(proxyHostsApi.getProxyHosts)
        .mockResolvedValueOnce([])
        .mockResolvedValueOnce(mockHosts);

      vi.mocked(proxyHostsApi.bulkUpdateACL).mockResolvedValue({
        updated: 1,
        errors: [],
      });

      const { result } = renderHook(() => useProxyHosts(), {
        wrapper: createWrapper(),
      });

      await waitFor(() => expect(result.current.loading).toBe(false));

      expect(result.current.hosts).toEqual([]);

      await result.current.bulkUpdateACL(['uuid-1'], 10);

      // Query should be invalidated and refetch
      await waitFor(() => expect(result.current.hosts).toEqual(mockHosts));
    });

    it('should handle bulk update errors', async () => {
      const error = new Error('Bulk update failed');
      vi.mocked(proxyHostsApi.getProxyHosts).mockResolvedValue([]);
      vi.mocked(proxyHostsApi.bulkUpdateACL).mockRejectedValue(error);

      const { result } = renderHook(() => useProxyHosts(), {
        wrapper: createWrapper(),
      });

      await waitFor(() => expect(result.current.loading).toBe(false));

      await expect(result.current.bulkUpdateACL(['uuid-1'], 5)).rejects.toThrow(
        'Bulk update failed'
      );
    });

    it('should track bulk updating state', async () => {
      vi.mocked(proxyHostsApi.getProxyHosts).mockResolvedValue([]);
      vi.mocked(proxyHostsApi.bulkUpdateACL).mockImplementation(
        () => new Promise((resolve) => setTimeout(() => resolve({ updated: 1, errors: [] }), 100))
      );

      const { result } = renderHook(() => useProxyHosts(), {
        wrapper: createWrapper(),
      });

      await waitFor(() => expect(result.current.loading).toBe(false));

      expect(result.current.isBulkUpdating).toBe(false);

      const promise = result.current.bulkUpdateACL(['uuid-1'], 1);

      await waitFor(() => expect(result.current.isBulkUpdating).toBe(true));

      await promise;

      await waitFor(() => expect(result.current.isBulkUpdating).toBe(false));
    });
  });

  describe('bulkUpdateGroup', () => {
    it('calls the API and returns the result', async () => {
      const mockResult = { updated: 1, errors: [] };
      vi.mocked(proxyHostsApi.getProxyHosts).mockResolvedValue([]);
      vi.mocked(proxyHostsApi.bulkUpdateGroup).mockResolvedValue(mockResult);

      const { result } = renderHook(() => useProxyHosts(), {
        wrapper: createWrapper(),
      });

      await waitFor(() => expect(result.current.loading).toBe(false));

      const response = await result.current.bulkUpdateGroup(['uuid-1'], 'group-uuid');

      expect(proxyHostsApi.bulkUpdateGroup).toHaveBeenCalledWith(['uuid-1'], 'group-uuid');
      expect(response).toEqual(mockResult);
    });

    it('sends null proxyGroupId when ungrouping', async () => {
      const mockResult = { updated: 2, errors: [] };
      vi.mocked(proxyHostsApi.getProxyHosts).mockResolvedValue([]);
      vi.mocked(proxyHostsApi.bulkUpdateGroup).mockResolvedValue(mockResult);

      const { result } = renderHook(() => useProxyHosts(), {
        wrapper: createWrapper(),
      });

      await waitFor(() => expect(result.current.loading).toBe(false));

      await result.current.bulkUpdateGroup(['uuid-1', 'uuid-2'], null);

      expect(proxyHostsApi.bulkUpdateGroup).toHaveBeenCalledWith(['uuid-1', 'uuid-2'], null);
    });

    it('tracks isBulkUpdatingGroup state', async () => {
      vi.mocked(proxyHostsApi.getProxyHosts).mockResolvedValue([]);
      vi.mocked(proxyHostsApi.bulkUpdateGroup).mockImplementation(
        () => new Promise((resolve) => setTimeout(() => resolve({ updated: 1, errors: [] }), 50)),
      );

      const { result } = renderHook(() => useProxyHosts(), {
        wrapper: createWrapper(),
      });

      await waitFor(() => expect(result.current.loading).toBe(false));

      expect(result.current.isBulkUpdatingGroup).toBe(false);

      const promise = result.current.bulkUpdateGroup(['uuid-1'], 'group-uuid');

      await waitFor(() => expect(result.current.isBulkUpdatingGroup).toBe(true));

      await promise;

      await waitFor(() => expect(result.current.isBulkUpdatingGroup).toBe(false));
    });
  });
});
