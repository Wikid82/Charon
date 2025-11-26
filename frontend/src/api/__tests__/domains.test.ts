import { describe, it, expect, vi, beforeEach } from 'vitest';
import client from '../client';
import { getDomains, createDomain, deleteDomain, Domain } from '../domains';

vi.mock('../client', () => ({
  default: {
    get: vi.fn(),
    post: vi.fn(),
    delete: vi.fn(),
  },
}));

describe('domains API', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  const mockDomain: Domain = {
    id: 1,
    uuid: '123',
    name: 'example.com',
    created_at: '2023-01-01',
  };

  it('getDomains calls client.get', async () => {
    vi.mocked(client.get).mockResolvedValue({ data: [mockDomain] });
    const result = await getDomains();
    expect(client.get).toHaveBeenCalledWith('/domains');
    expect(result).toEqual([mockDomain]);
  });

  it('createDomain calls client.post', async () => {
    vi.mocked(client.post).mockResolvedValue({ data: mockDomain });
    const result = await createDomain('example.com');
    expect(client.post).toHaveBeenCalledWith('/domains', { name: 'example.com' });
    expect(result).toEqual(mockDomain);
  });

  it('deleteDomain calls client.delete', async () => {
    vi.mocked(client.delete).mockResolvedValue({ data: {} });
    await deleteDomain('123');
    expect(client.delete).toHaveBeenCalledWith('/domains/123');
  });
});
