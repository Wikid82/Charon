import { describe, it, expect, vi, beforeEach } from 'vitest';
import { renderHook, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { useAccessLists, useAccessList, useCreateAccessList, useUpdateAccessList, useDeleteAccessList, useTestIP } from '../useAccessLists';
import { accessListsApi } from '../../api/accessLists';
import type { AccessList } from '../../api/accessLists';

// Mock the API module
vi.mock('../../api/accessLists');

// Create a wrapper with QueryClient
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

describe('useAccessLists hooks', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe('useAccessLists', () => {
    it('should fetch all access lists', async () => {
      const mockLists: AccessList[] = [
        {
          id: 1,
          uuid: 'test-uuid',
          name: 'Test ACL',
          description: 'Test',
          type: 'whitelist',
          ip_rules: '[]',
          country_codes: '',
          local_network_only: false,
          enabled: true,
          created_at: '2024-01-01',
          updated_at: '2024-01-01',
        },
      ];

      vi.mocked(accessListsApi.list).mockResolvedValueOnce(mockLists);

      const { result } = renderHook(() => useAccessLists(), {
        wrapper: createWrapper(),
      });

      await waitFor(() => expect(result.current.isSuccess).toBe(true));
      expect(result.current.data).toEqual(mockLists);
    });
  });

  describe('useAccessList', () => {
    it('should fetch a single access list', async () => {
      const mockList: AccessList = {
        id: 1,
        uuid: 'test-uuid',
        name: 'Test ACL',
        description: 'Test',
        type: 'whitelist',
        ip_rules: '[]',
        country_codes: '',
        local_network_only: false,
        enabled: true,
        created_at: '2024-01-01',
        updated_at: '2024-01-01',
      };

      vi.mocked(accessListsApi.get).mockResolvedValueOnce(mockList);

      const { result } = renderHook(() => useAccessList(1), {
        wrapper: createWrapper(),
      });

      await waitFor(() => expect(result.current.isSuccess).toBe(true));
      expect(result.current.data).toEqual(mockList);
    });
  });

  describe('useCreateAccessList', () => {
    it('should create a new access list', async () => {
      const newList = {
        name: 'New ACL',
        description: 'New',
        type: 'whitelist' as const,
        ip_rules: '[]',
        enabled: true,
      };

      const mockResponse: AccessList = {
        id: 1,
        uuid: 'new-uuid',
        ...newList,
        country_codes: '',
        local_network_only: false,
        created_at: '2024-01-01',
        updated_at: '2024-01-01',
      };

      vi.mocked(accessListsApi.create).mockResolvedValueOnce(mockResponse);

      const { result } = renderHook(() => useCreateAccessList(), {
        wrapper: createWrapper(),
      });

      result.current.mutate(newList);

      await waitFor(() => expect(result.current.isSuccess).toBe(true));
      expect(result.current.data).toEqual(mockResponse);
    });
  });

  describe('useUpdateAccessList', () => {
    it('should update an access list', async () => {
      const updates = { name: 'Updated ACL' };
      const mockResponse: AccessList = {
        id: 1,
        uuid: 'test-uuid',
        name: 'Updated ACL',
        description: 'Test',
        type: 'whitelist',
        ip_rules: '[]',
        country_codes: '',
        local_network_only: false,
        enabled: true,
        created_at: '2024-01-01',
        updated_at: '2024-01-01',
      };

      vi.mocked(accessListsApi.update).mockResolvedValueOnce(mockResponse);

      const { result } = renderHook(() => useUpdateAccessList(), {
        wrapper: createWrapper(),
      });

      result.current.mutate({ id: 1, data: updates });

      await waitFor(() => expect(result.current.isSuccess).toBe(true));
      expect(result.current.data).toEqual(mockResponse);
    });
  });

  describe('useDeleteAccessList', () => {
    it('should delete an access list', async () => {
      vi.mocked(accessListsApi.delete).mockResolvedValueOnce(undefined);

      const { result } = renderHook(() => useDeleteAccessList(), {
        wrapper: createWrapper(),
      });

      result.current.mutate(1);

      await waitFor(() => expect(result.current.isSuccess).toBe(true));
      expect(accessListsApi.delete).toHaveBeenCalledWith(1);
    });
  });

  describe('useTestIP', () => {
    it('should test an IP against an access list', async () => {
      const mockResponse = { allowed: true, reason: 'Test' };

      vi.mocked(accessListsApi.testIP).mockResolvedValueOnce(mockResponse);

      const { result } = renderHook(() => useTestIP(), {
        wrapper: createWrapper(),
      });

      result.current.mutate({ id: 1, ipAddress: '192.168.1.1' });

      await waitFor(() => expect(result.current.isSuccess).toBe(true));
      expect(result.current.data).toEqual(mockResponse);
    });
  });
});
