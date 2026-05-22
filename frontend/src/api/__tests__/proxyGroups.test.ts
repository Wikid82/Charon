import { describe, it, expect, vi, beforeEach } from 'vitest';

import client from '../client';
import { proxyGroupsApi, type ProxyGroup } from '../proxyGroups';

vi.mock('../client', () => ({
  default: {
    get: vi.fn(),
    post: vi.fn(),
    put: vi.fn(),
    delete: vi.fn(),
  },
}));

describe('proxyGroupsApi', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  const mockGroup: ProxyGroup = {
    uuid: 'abc-123',
    name: 'Test Group',
    description: 'A test group',
    color: '#ff0000',
    host_count: 2,
    created_at: '2026-01-01T00:00:00Z',
    updated_at: '2026-01-01T00:00:00Z',
  };

  it('list calls client.get /proxy-groups', async () => {
    vi.mocked(client.get).mockResolvedValue({ data: [mockGroup] });
    const result = await proxyGroupsApi.list();
    expect(client.get).toHaveBeenCalledWith('/proxy-groups');
    expect(result).toEqual([mockGroup]);
  });

  it('get calls client.get with uuid', async () => {
    vi.mocked(client.get).mockResolvedValue({ data: mockGroup });
    const result = await proxyGroupsApi.get('abc-123');
    expect(client.get).toHaveBeenCalledWith('/proxy-groups/abc-123');
    expect(result).toEqual(mockGroup);
  });

  it('create calls client.post with payload', async () => {
    vi.mocked(client.post).mockResolvedValue({ data: mockGroup });
    const payload = { name: 'Test Group', description: 'A test group', color: '#ff0000' };
    const result = await proxyGroupsApi.create(payload);
    expect(client.post).toHaveBeenCalledWith('/proxy-groups', payload);
    expect(result).toEqual(mockGroup);
  });

  it('update calls client.put with uuid and payload', async () => {
    vi.mocked(client.put).mockResolvedValue({ data: mockGroup });
    const payload = { name: 'Updated Group' };
    const result = await proxyGroupsApi.update('abc-123', payload);
    expect(client.put).toHaveBeenCalledWith('/proxy-groups/abc-123', payload);
    expect(result).toEqual(mockGroup);
  });

  it('delete calls client.delete with uuid', async () => {
    vi.mocked(client.delete).mockResolvedValue({ data: undefined });
    await proxyGroupsApi.delete('abc-123');
    expect(client.delete).toHaveBeenCalledWith('/proxy-groups/abc-123');
  });
});
