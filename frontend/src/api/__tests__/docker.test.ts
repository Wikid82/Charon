import { vi, describe, it, expect, beforeEach } from 'vitest';
import { dockerApi } from '../docker';
import client from '../client';

vi.mock('../client', () => ({
  default: {
    get: vi.fn(),
  },
}));

describe('dockerApi', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe('listContainers', () => {
    const mockContainers = [
      {
        id: 'abc123',
        names: ['/container1'],
        image: 'nginx:latest',
        state: 'running',
        status: 'Up 2 hours',
        network: 'bridge',
        ip: '172.17.0.2',
        ports: [{ private_port: 80, public_port: 8080, type: 'tcp' }],
      },
      {
        id: 'def456',
        names: ['/container2'],
        image: 'redis:alpine',
        state: 'running',
        status: 'Up 1 hour',
        network: 'bridge',
        ip: '172.17.0.3',
        ports: [],
      },
    ];

    it('fetches containers without parameters', async () => {
      vi.mocked(client.get).mockResolvedValue({ data: mockContainers });

      const result = await dockerApi.listContainers();

      expect(client.get).toHaveBeenCalledWith('/docker/containers', { params: {} });
      expect(result).toEqual(mockContainers);
    });

    it('fetches containers with host parameter', async () => {
      vi.mocked(client.get).mockResolvedValue({ data: mockContainers });

      const result = await dockerApi.listContainers('192.168.1.100');

      expect(client.get).toHaveBeenCalledWith('/docker/containers', {
        params: { host: '192.168.1.100' },
      });
      expect(result).toEqual(mockContainers);
    });

    it('fetches containers with serverId parameter', async () => {
      vi.mocked(client.get).mockResolvedValue({ data: mockContainers });

      const result = await dockerApi.listContainers(undefined, 'server-uuid-123');

      expect(client.get).toHaveBeenCalledWith('/docker/containers', {
        params: { server_id: 'server-uuid-123' },
      });
      expect(result).toEqual(mockContainers);
    });

    it('fetches containers with both host and serverId parameters', async () => {
      vi.mocked(client.get).mockResolvedValue({ data: mockContainers });

      const result = await dockerApi.listContainers('192.168.1.100', 'server-uuid-123');

      expect(client.get).toHaveBeenCalledWith('/docker/containers', {
        params: { host: '192.168.1.100', server_id: 'server-uuid-123' },
      });
      expect(result).toEqual(mockContainers);
    });

    it('returns empty array when no containers', async () => {
      vi.mocked(client.get).mockResolvedValue({ data: [] });

      const result = await dockerApi.listContainers();

      expect(result).toEqual([]);
    });

    it('handles API error', async () => {
      vi.mocked(client.get).mockRejectedValue(new Error('Network error'));

      await expect(dockerApi.listContainers()).rejects.toThrow('Network error');
    });
  });
});
