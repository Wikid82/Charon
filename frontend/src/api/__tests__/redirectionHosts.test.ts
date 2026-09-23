import { describe, it, expect, vi, beforeEach } from 'vitest';

import client from '../client';
import { redirectionHostsApi, REDIRECT_STATUS_CODES, type RedirectionHost } from '../redirectionHosts';

vi.mock('../client', () => ({
  default: {
    get: vi.fn(),
    post: vi.fn(),
    put: vi.fn(),
    delete: vi.fn(),
  },
}));

describe('redirectionHostsApi', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  const mockHost: RedirectionHost = {
    uuid: 'rh-1',
    name: 'Old blog redirect',
    domain_names: 'old-blog.example.com',
    target_url: 'https://newblog.example.com',
    status_code: 301,
    preserve_path: true,
    ssl_forced: true,
    http2_support: true,
    hsts_enabled: false,
    hsts_subdomains: false,
    enabled: true,
    certificate_id: null,
    certificate: null,
    dns_provider_id: null,
    dns_provider: null,
    use_dns_challenge: false,
    created_at: '2026-01-01T00:00:00Z',
    updated_at: '2026-01-01T00:00:00Z',
  };

  it('list calls client.get /redirection-hosts', async () => {
    vi.mocked(client.get).mockResolvedValue({ data: [mockHost] });
    const result = await redirectionHostsApi.list();
    expect(client.get).toHaveBeenCalledWith('/redirection-hosts');
    expect(result).toEqual([mockHost]);
  });

  it('get calls client.get with uuid', async () => {
    vi.mocked(client.get).mockResolvedValue({ data: mockHost });
    const result = await redirectionHostsApi.get('rh-1');
    expect(client.get).toHaveBeenCalledWith('/redirection-hosts/rh-1');
    expect(result).toEqual(mockHost);
  });

  it('create calls client.post with payload', async () => {
    vi.mocked(client.post).mockResolvedValue({ data: mockHost });
    const payload = { name: 'Old blog redirect', domain_names: 'old-blog.example.com', target_url: 'https://newblog.example.com', status_code: 301 as const };
    const result = await redirectionHostsApi.create(payload);
    expect(client.post).toHaveBeenCalledWith('/redirection-hosts', payload);
    expect(result).toEqual(mockHost);
  });

  it('update calls client.put with uuid and payload', async () => {
    vi.mocked(client.put).mockResolvedValue({ data: mockHost });
    const payload = { target_url: 'https://updated.example.com' };
    const result = await redirectionHostsApi.update('rh-1', payload);
    expect(client.put).toHaveBeenCalledWith('/redirection-hosts/rh-1', payload);
    expect(result).toEqual(mockHost);
  });

  it('delete calls client.delete with uuid', async () => {
    vi.mocked(client.delete).mockResolvedValue({ data: undefined });
    await redirectionHostsApi.delete('rh-1');
    expect(client.delete).toHaveBeenCalledWith('/redirection-hosts/rh-1');
  });
});

describe('REDIRECT_STATUS_CODES', () => {
  it('exposes exactly the four supported codes with plain-language labels', () => {
    expect(REDIRECT_STATUS_CODES.map((o) => o.value)).toEqual([301, 302, 307, 308]);
    expect(REDIRECT_STATUS_CODES.find((o) => o.value === 301)?.label).toMatch(/301.*permanent/i);
    expect(REDIRECT_STATUS_CODES.find((o) => o.value === 302)?.label).toMatch(/302.*temporary/i);
    expect(REDIRECT_STATUS_CODES.find((o) => o.value === 307)?.label).toMatch(/307.*temporary/i);
    expect(REDIRECT_STATUS_CODES.find((o) => o.value === 308)?.label).toMatch(/308.*permanent/i);
  });
});
