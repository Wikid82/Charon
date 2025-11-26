import { describe, it, expect, vi, beforeEach } from 'vitest';
import client from '../client';
import {
  getProxyHosts,
  getProxyHost,
  createProxyHost,
  updateProxyHost,
  deleteProxyHost,
  testProxyHostConnection,
  ProxyHost
} from '../proxyHosts';

vi.mock('../client', () => ({
  default: {
    get: vi.fn(),
    post: vi.fn(),
    put: vi.fn(),
    delete: vi.fn(),
  },
}));

describe('proxyHosts API', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  const mockHost: ProxyHost = {
    uuid: '123',
    name: 'Example Host',
    domain_names: 'example.com',
    forward_scheme: 'http',
    forward_host: 'localhost',
    forward_port: 8080,
    ssl_forced: true,
    http2_support: true,
    hsts_enabled: true,
    hsts_subdomains: false,
    block_exploits: false,
    websocket_support: false,
    locations: [],
    enabled: true,
    created_at: '2023-01-01',
    updated_at: '2023-01-01',
  };

  it('getProxyHosts calls client.get', async () => {
    vi.mocked(client.get).mockResolvedValue({ data: [mockHost] });
    const result = await getProxyHosts();
    expect(client.get).toHaveBeenCalledWith('/proxy-hosts');
    expect(result).toEqual([mockHost]);
  });

  it('getProxyHost calls client.get with uuid', async () => {
    vi.mocked(client.get).mockResolvedValue({ data: mockHost });
    const result = await getProxyHost('123');
    expect(client.get).toHaveBeenCalledWith('/proxy-hosts/123');
    expect(result).toEqual(mockHost);
  });

  it('createProxyHost calls client.post', async () => {
    vi.mocked(client.post).mockResolvedValue({ data: mockHost });
    const newHost = { domain_names: 'example.com' };
    const result = await createProxyHost(newHost);
    expect(client.post).toHaveBeenCalledWith('/proxy-hosts', newHost);
    expect(result).toEqual(mockHost);
  });

  it('updateProxyHost calls client.put', async () => {
    vi.mocked(client.put).mockResolvedValue({ data: mockHost });
    const updates = { enabled: false };
    const result = await updateProxyHost('123', updates);
    expect(client.put).toHaveBeenCalledWith('/proxy-hosts/123', updates);
    expect(result).toEqual(mockHost);
  });

  it('deleteProxyHost calls client.delete', async () => {
    vi.mocked(client.delete).mockResolvedValue({ data: {} });
    await deleteProxyHost('123');
    expect(client.delete).toHaveBeenCalledWith('/proxy-hosts/123');
  });

  it('testProxyHostConnection calls client.post', async () => {
    vi.mocked(client.post).mockResolvedValue({ data: {} });
    await testProxyHostConnection('localhost', 8080);
    expect(client.post).toHaveBeenCalledWith('/proxy-hosts/test', {
      forward_host: 'localhost',
      forward_port: 8080,
    });
  });
});
