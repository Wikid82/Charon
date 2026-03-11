import { describe, it, expect, vi, beforeEach } from 'vitest';

import client from '../client';
import { getWebSocketConnections, getWebSocketStats } from '../websocket';

vi.mock('../client');

describe('WebSocket API', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe('getWebSocketConnections', () => {
    it('should fetch WebSocket connections', async () => {
      const mockResponse = {
        connections: [
          {
            id: 'test-conn-1',
            type: 'logs',
            connected_at: '2024-01-15T10:00:00Z',
            last_activity_at: '2024-01-15T10:05:00Z',
            remote_addr: '192.168.1.1:12345',
            user_agent: 'Mozilla/5.0',
            filters: 'level=error',
          },
          {
            id: 'test-conn-2',
            type: 'cerberus',
            connected_at: '2024-01-15T10:02:00Z',
            last_activity_at: '2024-01-15T10:06:00Z',
            remote_addr: '192.168.1.2:54321',
            user_agent: 'Chrome/90.0',
            filters: 'source=waf',
          },
        ],
        count: 2,
      };

      vi.mocked(client.get).mockResolvedValue({ data: mockResponse });

      const result = await getWebSocketConnections();

      expect(client.get).toHaveBeenCalledWith('/websocket/connections');
      expect(result).toEqual(mockResponse);
      expect(result.count).toBe(2);
      expect(result.connections).toHaveLength(2);
    });

    it('should handle empty connections', async () => {
      const mockResponse = {
        connections: [],
        count: 0,
      };

      vi.mocked(client.get).mockResolvedValue({ data: mockResponse });

      const result = await getWebSocketConnections();

      expect(result.connections).toHaveLength(0);
      expect(result.count).toBe(0);
    });

    it('should handle API errors', async () => {
      vi.mocked(client.get).mockRejectedValue(new Error('Network error'));

      await expect(getWebSocketConnections()).rejects.toThrow('Network error');
    });
  });

  describe('getWebSocketStats', () => {
    it('should fetch WebSocket statistics', async () => {
      const mockResponse = {
        total_active: 3,
        logs_connections: 2,
        cerberus_connections: 1,
        oldest_connection: '2024-01-15T09:55:00Z',
        last_updated: '2024-01-15T10:10:00Z',
      };

      vi.mocked(client.get).mockResolvedValue({ data: mockResponse });

      const result = await getWebSocketStats();

      expect(client.get).toHaveBeenCalledWith('/websocket/stats');
      expect(result).toEqual(mockResponse);
      expect(result.total_active).toBe(3);
      expect(result.logs_connections).toBe(2);
      expect(result.cerberus_connections).toBe(1);
    });

    it('should handle stats with no connections', async () => {
      const mockResponse = {
        total_active: 0,
        logs_connections: 0,
        cerberus_connections: 0,
        last_updated: '2024-01-15T10:10:00Z',
      };

      vi.mocked(client.get).mockResolvedValue({ data: mockResponse });

      const result = await getWebSocketStats();

      expect(result.total_active).toBe(0);
      expect(result.oldest_connection).toBeUndefined();
    });

    it('should handle API errors', async () => {
      vi.mocked(client.get).mockRejectedValue(new Error('Server error'));

      await expect(getWebSocketStats()).rejects.toThrow('Server error');
    });
  });
});
