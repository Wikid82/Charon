import { describe, it, expect, vi, beforeEach } from 'vitest';
import client from '../client';
import {
  getPlugins,
  getPlugin,
  enablePlugin,
  disablePlugin,
  reloadPlugins,
  type PluginInfo,
} from '../plugins';

// Mock the API client
vi.mock('../client', () => ({
  default: {
    get: vi.fn(),
    post: vi.fn(),
  },
}));

describe('Plugins API', () => {
  const mockPlugins: PluginInfo[] = [
    {
      id: 1,
      uuid: 'plugin-1',
      name: 'Test Plugin 1',
      type: 'auth',
      enabled: true,
      status: 'loaded',
      is_built_in: false,
      created_at: '2023-01-01',
      updated_at: '2023-01-01',
    },
    {
      id: 2,
      uuid: 'plugin-2',
      name: 'Test Plugin 2',
      type: 'notification',
      enabled: false,
      status: 'pending',
      is_built_in: true,
      created_at: '2023-01-01',
      updated_at: '2023-01-01',
    },
  ];

  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe('getPlugins', () => {
    it('fetches all plugins successfully', async () => {
      vi.mocked(client.get).mockResolvedValueOnce({ data: mockPlugins });

      const result = await getPlugins();

      expect(client.get).toHaveBeenCalledWith('/admin/plugins');
      expect(result).toEqual(mockPlugins);
    });

    it('propagates error when request fails', async () => {
      const error = new Error('API Error');
      vi.mocked(client.get).mockRejectedValueOnce(error);

      await expect(getPlugins()).rejects.toThrow(error);
    });
  });

  describe('getPlugin', () => {
    it('fetches a single plugin successfully', async () => {
      const plugin = mockPlugins[0];
      vi.mocked(client.get).mockResolvedValueOnce({ data: plugin });

      const result = await getPlugin(1);

      expect(client.get).toHaveBeenCalledWith('/admin/plugins/1');
      expect(result).toEqual(plugin);
    });

    it('propagates error when plugin not found', async () => {
      const error = new Error('Not Found');
      vi.mocked(client.get).mockRejectedValueOnce(error);

      await expect(getPlugin(999)).rejects.toThrow(error);
    });
  });

  describe('enablePlugin', () => {
    it('enables a plugin successfully', async () => {
      const response = { message: 'Plugin enabled' };
      vi.mocked(client.post).mockResolvedValueOnce({ data: response });

      const result = await enablePlugin(1);

      expect(client.post).toHaveBeenCalledWith('/admin/plugins/1/enable');
      expect(result).toEqual(response);
    });
  });

  describe('disablePlugin', () => {
    it('disables a plugin successfully', async () => {
      const response = { message: 'Plugin disabled' };
      vi.mocked(client.post).mockResolvedValueOnce({ data: response });

      const result = await disablePlugin(1);

      expect(client.post).toHaveBeenCalledWith('/admin/plugins/1/disable');
      expect(result).toEqual(response);
    });
  });

  describe('reloadPlugins', () => {
    it('reloads plugins successfully', async () => {
      const response = { message: 'Plugins reloaded', count: 5 };
      vi.mocked(client.post).mockResolvedValueOnce({ data: response });

      const result = await reloadPlugins();

      expect(client.post).toHaveBeenCalledWith('/admin/plugins/reload');
      expect(result).toEqual(response);
    });
  });
});
