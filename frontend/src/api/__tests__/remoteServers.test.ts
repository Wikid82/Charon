import { vi, describe, it, expect, beforeEach } from 'vitest';
import {
  getRemoteServers,
  getRemoteServer,
  createRemoteServer,
  updateRemoteServer,
  deleteRemoteServer,
  testRemoteServerConnection,
  testCustomRemoteServerConnection,
} from '../remoteServers';
import client from '../client';

vi.mock('../client', () => ({
  default: {
    get: vi.fn(),
    post: vi.fn(),
    put: vi.fn(),
    delete: vi.fn(),
  },
}));

describe('remoteServers API', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  const mockServer = {
    uuid: 'server-123',
    name: 'Test Server',
    provider: 'docker',
    host: '192.168.1.100',
    port: 2375,
    username: 'admin',
    enabled: true,
    reachable: true,
    last_check: '2024-01-01T12:00:00Z',
    created_at: '2024-01-01T00:00:00Z',
    updated_at: '2024-01-01T12:00:00Z',
  };

  describe('getRemoteServers', () => {
    it('fetches all servers', async () => {
      vi.mocked(client.get).mockResolvedValue({ data: [mockServer] });

      const result = await getRemoteServers();

      expect(client.get).toHaveBeenCalledWith('/remote-servers', { params: {} });
      expect(result).toEqual([mockServer]);
    });

    it('fetches enabled servers only', async () => {
      vi.mocked(client.get).mockResolvedValue({ data: [mockServer] });

      const result = await getRemoteServers(true);

      expect(client.get).toHaveBeenCalledWith('/remote-servers', { params: { enabled: true } });
      expect(result).toEqual([mockServer]);
    });
  });

  describe('getRemoteServer', () => {
    it('fetches a single server by UUID', async () => {
      vi.mocked(client.get).mockResolvedValue({ data: mockServer });

      const result = await getRemoteServer('server-123');

      expect(client.get).toHaveBeenCalledWith('/remote-servers/server-123');
      expect(result).toEqual(mockServer);
    });
  });

  describe('createRemoteServer', () => {
    it('creates a new server', async () => {
      const newServer = {
        name: 'New Server',
        provider: 'docker',
        host: '10.0.0.1',
        port: 2375,
      };
      vi.mocked(client.post).mockResolvedValue({ data: { ...mockServer, ...newServer } });

      const result = await createRemoteServer(newServer);

      expect(client.post).toHaveBeenCalledWith('/remote-servers', newServer);
      expect(result.name).toBe('New Server');
    });
  });

  describe('updateRemoteServer', () => {
    it('updates an existing server', async () => {
      const updates = { name: 'Updated Server', enabled: false };
      vi.mocked(client.put).mockResolvedValue({ data: { ...mockServer, ...updates } });

      const result = await updateRemoteServer('server-123', updates);

      expect(client.put).toHaveBeenCalledWith('/remote-servers/server-123', updates);
      expect(result.name).toBe('Updated Server');
      expect(result.enabled).toBe(false);
    });
  });

  describe('deleteRemoteServer', () => {
    it('deletes a server', async () => {
      vi.mocked(client.delete).mockResolvedValue({});

      await deleteRemoteServer('server-123');

      expect(client.delete).toHaveBeenCalledWith('/remote-servers/server-123');
    });
  });

  describe('testRemoteServerConnection', () => {
    it('tests connection to an existing server', async () => {
      vi.mocked(client.post).mockResolvedValue({ data: { address: '192.168.1.100:2375' } });

      const result = await testRemoteServerConnection('server-123');

      expect(client.post).toHaveBeenCalledWith('/remote-servers/server-123/test');
      expect(result.address).toBe('192.168.1.100:2375');
    });
  });

  describe('testCustomRemoteServerConnection', () => {
    it('tests connection to a custom host and port', async () => {
      vi.mocked(client.post).mockResolvedValue({
        data: { address: '10.0.0.1:2375', reachable: true },
      });

      const result = await testCustomRemoteServerConnection('10.0.0.1', 2375);

      expect(client.post).toHaveBeenCalledWith('/remote-servers/test', { host: '10.0.0.1', port: 2375 });
      expect(result.reachable).toBe(true);
    });

    it('handles unreachable server', async () => {
      vi.mocked(client.post).mockResolvedValue({
        data: { address: '10.0.0.1:2375', reachable: false, error: 'Connection refused' },
      });

      const result = await testCustomRemoteServerConnection('10.0.0.1', 2375);

      expect(result.reachable).toBe(false);
      expect(result.error).toBe('Connection refused');
    });
  });
});
