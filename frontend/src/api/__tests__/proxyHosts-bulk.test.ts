import { describe, it, expect, vi, beforeEach } from 'vitest';
import { bulkUpdateACL } from '../proxyHosts';
import type { BulkUpdateACLResponse } from '../proxyHosts';

// Mock the client module
const mockPut = vi.fn();
vi.mock('../client', () => ({
  default: {
    put: (...args: unknown[]) => mockPut(...args),
  },
}));

describe('proxyHosts bulk operations', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe('bulkUpdateACL', () => {
    it('should apply ACL to multiple hosts', async () => {
      const mockResponse: BulkUpdateACLResponse = {
        updated: 3,
        errors: [],
      };
      mockPut.mockResolvedValue({ data: mockResponse });

      const hostUUIDs = ['uuid-1', 'uuid-2', 'uuid-3'];
      const accessListID = 42;
      const result = await bulkUpdateACL(hostUUIDs, accessListID);

      expect(mockPut).toHaveBeenCalledWith('/proxy-hosts/bulk-update-acl', {
        host_uuids: hostUUIDs,
        access_list_id: accessListID,
      });
      expect(result).toEqual(mockResponse);
    });

    it('should remove ACL from hosts when accessListID is null', async () => {
      const mockResponse: BulkUpdateACLResponse = {
        updated: 2,
        errors: [],
      };
      mockPut.mockResolvedValue({ data: mockResponse });

      const hostUUIDs = ['uuid-1', 'uuid-2'];
      const result = await bulkUpdateACL(hostUUIDs, null);

      expect(mockPut).toHaveBeenCalledWith('/proxy-hosts/bulk-update-acl', {
        host_uuids: hostUUIDs,
        access_list_id: null,
      });
      expect(result).toEqual(mockResponse);
    });

    it('should handle partial failures', async () => {
      const mockResponse: BulkUpdateACLResponse = {
        updated: 1,
        errors: [
          { uuid: 'invalid-uuid', error: 'proxy host not found' },
        ],
      };
      mockPut.mockResolvedValue({ data: mockResponse });

      const hostUUIDs = ['valid-uuid', 'invalid-uuid'];
      const accessListID = 10;
      const result = await bulkUpdateACL(hostUUIDs, accessListID);

      expect(result.updated).toBe(1);
      expect(result.errors).toHaveLength(1);
      expect(result.errors[0].uuid).toBe('invalid-uuid');
    });

    it('should handle empty host list', async () => {
      const mockResponse: BulkUpdateACLResponse = {
        updated: 0,
        errors: [],
      };
      mockPut.mockResolvedValue({ data: mockResponse });

      const result = await bulkUpdateACL([], 5);

      expect(mockPut).toHaveBeenCalledWith('/proxy-hosts/bulk-update-acl', {
        host_uuids: [],
        access_list_id: 5,
      });
      expect(result.updated).toBe(0);
    });

    it('should propagate API errors', async () => {
      const error = new Error('Network error');
      mockPut.mockRejectedValue(error);

      await expect(bulkUpdateACL(['uuid-1'], 1)).rejects.toThrow('Network error');
    });
  });
});
