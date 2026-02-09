import { describe, it, expect, vi, beforeEach } from 'vitest';
import { accessListsApi } from '../accessLists';
import client from '../client';
import type { AccessList } from '../accessLists';

// Mock the client module
vi.mock('../client', () => ({
  default: {
    get: vi.fn(),
    post: vi.fn(),
    put: vi.fn(),
    delete: vi.fn(),
  },
}));

describe('accessListsApi', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe('list', () => {
    it('should fetch all access lists', async () => {
      const mockLists: AccessList[] = [
        {
          id: 1,
          uuid: 'test-uuid',
          name: 'Test ACL',
          description: 'Test description',
          type: 'whitelist',
          ip_rules: '[{"cidr":"192.168.1.0/24"}]',
          country_codes: '',
          local_network_only: false,
          enabled: true,
          created_at: '2024-01-01T00:00:00Z',
          updated_at: '2024-01-01T00:00:00Z',
        },
      ];

      vi.mocked(client.get).mockResolvedValueOnce({ data: mockLists });

      const result = await accessListsApi.list();

      expect(client.get).toHaveBeenCalledWith<[string]>('/access-lists');
      expect(result).toEqual(mockLists);
    });
  });

  describe('get', () => {
    it('should fetch access list by ID', async () => {
      const mockList: AccessList = {
        id: 1,
        uuid: 'test-uuid',
        name: 'Test ACL',
        description: 'Test description',
        type: 'whitelist',
        ip_rules: '[{"cidr":"192.168.1.0/24"}]',
        country_codes: '',
        local_network_only: false,
        enabled: true,
        created_at: '2024-01-01T00:00:00Z',
        updated_at: '2024-01-01T00:00:00Z',
      };

      vi.mocked(client.get).mockResolvedValueOnce({ data: mockList });

      const result = await accessListsApi.get(1);

      expect(client.get).toHaveBeenCalledWith<[string]>('/access-lists/1');
      expect(result).toEqual(mockList);
    });
  });

  describe('create', () => {
    it('should create a new access list', async () => {
      const newList = {
        name: 'New ACL',
        description: 'New description',
        type: 'whitelist' as const,
        ip_rules: '[{"cidr":"10.0.0.0/8"}]',
        enabled: true,
      };

      const mockResponse: AccessList = {
        id: 1,
        uuid: 'new-uuid',
        ...newList,
        country_codes: '',
        local_network_only: false,
        created_at: '2024-01-01T00:00:00Z',
        updated_at: '2024-01-01T00:00:00Z',
      };

      vi.mocked(client.post).mockResolvedValueOnce({ data: mockResponse });

      const result = await accessListsApi.create(newList);

      expect(client.post).toHaveBeenCalledWith<[string, typeof newList]>('/access-lists', newList);
      expect(result).toEqual(mockResponse);
    });
  });

  describe('update', () => {
    it('should update an access list', async () => {
      const updates = {
        name: 'Updated ACL',
        enabled: false,
      };

      const mockResponse: AccessList = {
        id: 1,
        uuid: 'test-uuid',
        name: 'Updated ACL',
        description: 'Test description',
        type: 'whitelist',
        ip_rules: '[{"cidr":"192.168.1.0/24"}]',
        country_codes: '',
        local_network_only: false,
        enabled: false,
        created_at: '2024-01-01T00:00:00Z',
        updated_at: '2024-01-01T00:00:00Z',
      };

      vi.mocked(client.put).mockResolvedValueOnce({ data: mockResponse });

      const result = await accessListsApi.update(1, updates);

      expect(client.put).toHaveBeenCalledWith<[string, typeof updates]>('/access-lists/1', updates);
      expect(result).toEqual(mockResponse);
    });
  });

  describe('delete', () => {
    it('should delete an access list', async () => {
      vi.mocked(client.delete).mockResolvedValueOnce({ data: undefined });

      await accessListsApi.delete(1);

      expect(client.delete).toHaveBeenCalledWith<[string]>('/access-lists/1');
    });
  });

  describe('testIP', () => {
    it('should test an IP against an access list', async () => {
      const mockResponse = {
        allowed: true,
        reason: 'IP matches whitelist rule',
      };

      vi.mocked(client.post).mockResolvedValueOnce({ data: mockResponse });

      const result = await accessListsApi.testIP(1, '192.168.1.100');

      expect(client.post).toHaveBeenCalledWith<[string, { ip_address: string }]>('/access-lists/1/test', {
        ip_address: '192.168.1.100',
      });
      expect(result).toEqual(mockResponse);
    });
  });

  describe('getTemplates', () => {
    it('should fetch access list templates', async () => {
      const mockTemplates = [
        {
          name: 'Private Networks',
          description: 'RFC1918 private networks',
          type: 'whitelist' as const,
          ip_rules: '[{"cidr":"10.0.0.0/8"},{"cidr":"172.16.0.0/12"},{"cidr":"192.168.0.0/16"}]',
        },
      ];

      vi.mocked(client.get).mockResolvedValueOnce({ data: mockTemplates });

      const result = await accessListsApi.getTemplates();

      expect(client.get).toHaveBeenCalledWith<[string]>('/access-lists/templates');
      expect(result).toEqual(mockTemplates);
    });
  });
});
