import { describe, it, expect, vi, beforeEach } from 'vitest';

import {
  getCertificates,
  getCertificateDetail,
  uploadCertificate,
  updateCertificate,
  deleteCertificate,
  bulkDeleteCertificates,
  BULK_DELETE_MAX,
  exportCertificate,
  validateCertificate,
  type Certificate,
  type CertificateDetail,
} from '../certificates';
import client from '../client';

vi.mock('../client', () => ({
  default: {
    get: vi.fn(),
    post: vi.fn(),
    put: vi.fn(),
    delete: vi.fn(),
  },
}));

describe('certificates API', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  const mockCert: Certificate = {
    uuid: 'abc-123',
    domains: 'example.com',
    issuer: 'Let\'s Encrypt',
    expires_at: '2023-01-01',
    status: 'valid',
    provider: 'letsencrypt',
    has_key: true,
    in_use: false,
  };

  it('getCertificates calls client.get', async () => {
    vi.mocked(client.get).mockResolvedValue({ data: [mockCert] });
    const result = await getCertificates();
    expect(client.get).toHaveBeenCalledWith('/certificates');
    expect(result).toEqual([mockCert]);
  });

  it('uploadCertificate calls client.post with FormData', async () => {
    vi.mocked(client.post).mockResolvedValue({ data: mockCert });
    const certFile = new File(['cert'], 'cert.pem', { type: 'text/plain' });
    const keyFile = new File(['key'], 'key.pem', { type: 'text/plain' });

    const result = await uploadCertificate('My Cert', certFile, keyFile);

    expect(client.post).toHaveBeenCalledWith('/certificates', expect.any(FormData), {
      headers: { 'Content-Type': 'multipart/form-data' },
    });
    expect(result).toEqual(mockCert);
  });

  it('deleteCertificate calls client.delete', async () => {
    vi.mocked(client.delete).mockResolvedValue({ data: {} });
    await deleteCertificate('abc-123');
    expect(client.delete).toHaveBeenCalledWith('/certificates/abc-123');
  });

  it('getCertificateDetail calls client.get with uuid', async () => {
    const detail: CertificateDetail = {
      ...mockCert,
      assigned_hosts: [],
      chain: [],
      auto_renew: false,
      created_at: '2023-01-01',
      updated_at: '2023-01-01',
    };
    vi.mocked(client.get).mockResolvedValue({ data: detail });
    const result = await getCertificateDetail('abc-123');
    expect(client.get).toHaveBeenCalledWith('/certificates/abc-123');
    expect(result).toEqual(detail);
  });

  it('updateCertificate calls client.put with name', async () => {
    vi.mocked(client.put).mockResolvedValue({ data: mockCert });
    const result = await updateCertificate('abc-123', 'New Name');
    expect(client.put).toHaveBeenCalledWith('/certificates/abc-123', { name: 'New Name' });
    expect(result).toEqual(mockCert);
  });

  it('exportCertificate calls client.post with blob response type', async () => {
    const blob = new Blob(['data']);
    vi.mocked(client.post).mockResolvedValue({ data: blob });
    const result = await exportCertificate('abc-123', 'pem', true, 'pass', 'pfx-pass');
    expect(client.post).toHaveBeenCalledWith(
      '/certificates/abc-123/export',
      { format: 'pem', include_key: true, password: 'pass', pfx_password: 'pfx-pass' },
      { responseType: 'blob' },
    );
    expect(result).toEqual(blob);
  });

  it('validateCertificate calls client.post with FormData', async () => {
    const validation = { valid: true, common_name: 'example.com', domains: ['example.com'], issuer_org: 'LE', expires_at: '2024-01-01', key_match: true, chain_valid: true, chain_depth: 1, warnings: [], errors: [] };
    vi.mocked(client.post).mockResolvedValue({ data: validation });
    const certFile = new File(['cert'], 'cert.pem', { type: 'text/plain' });
    const keyFile = new File(['key'], 'key.pem', { type: 'text/plain' });

    const result = await validateCertificate(certFile, keyFile);
    expect(client.post).toHaveBeenCalledWith('/certificates/validate', expect.any(FormData), {
      headers: { 'Content-Type': 'multipart/form-data' },
    });
    expect(result).toEqual(validation);
  });

  it('uploadCertificate includes chain file when provided', async () => {
    vi.mocked(client.post).mockResolvedValue({ data: mockCert });
    const certFile = new File(['cert'], 'cert.pem');
    const keyFile = new File(['key'], 'key.pem');
    const chainFile = new File(['chain'], 'chain.pem');

    await uploadCertificate('My Cert', certFile, keyFile, chainFile);
    const formData = vi.mocked(client.post).mock.calls[0][1] as FormData;
    expect(formData.get('chain_file')).toBeTruthy();
  });

  it('validateCertificate includes chain file when provided', async () => {
    vi.mocked(client.post).mockResolvedValue({ data: {} });
    const certFile = new File(['cert'], 'cert.pem');
    const chainFile = new File(['chain'], 'chain.pem');

    await validateCertificate(certFile, undefined, chainFile);
    const formData = vi.mocked(client.post).mock.calls[0][1] as FormData;
    expect(formData.get('chain_file')).toBeTruthy();
    expect(formData.get('key_file')).toBeNull();
  });

  describe('bulkDeleteCertificates', () => {
    it('posts all uuids in one request with a 300s timeout and returns the response', async () => {
      const response = {
        deleted: ['uuid-1', 'uuid-2'],
        failed: [{ uuid: 'uuid-3', reason: 'in_use' as const }],
      };
      vi.mocked(client.post).mockResolvedValue({ data: response });

      const result = await bulkDeleteCertificates(['uuid-1', 'uuid-2', 'uuid-3']);

      expect(client.post).toHaveBeenCalledTimes(1);
      expect(client.post).toHaveBeenCalledWith(
        '/certificates/bulk-delete',
        { uuids: ['uuid-1', 'uuid-2', 'uuid-3'] },
        { timeout: 300000 },
      );
      expect(result).toEqual(response);
    });

    it('propagates request errors so the caller sees a rejection', async () => {
      const error = new Error('failed to create backup before deletion');
      vi.mocked(client.post).mockRejectedValue(error);

      await expect(bulkDeleteCertificates(['uuid-1'])).rejects.toBe(error);
    });

    it('sends at most BULK_DELETE_MAX uuids per request and merges chunk results', async () => {
      const uuids = Array.from({ length: 450 }, (_, i) => `uuid-${i}`);
      vi.mocked(client.post).mockImplementation(async (_url, body) => {
        const sent = (body as { uuids: string[] }).uuids;
        return {
          data: {
            deleted: sent.slice(0, 1),
            failed: [{ uuid: sent[1], reason: 'in_use' as const }],
          },
        };
      });

      const result = await bulkDeleteCertificates(uuids);

      expect(BULK_DELETE_MAX).toBe(200);
      expect(client.post).toHaveBeenCalledTimes(3);
      const sentChunks = vi.mocked(client.post).mock.calls.map(
        (call) => (call[1] as { uuids: string[] }).uuids,
      );
      expect(sentChunks.map((chunk) => chunk.length)).toEqual([200, 200, 50]);
      expect(sentChunks.flat()).toEqual(uuids);
      for (const call of vi.mocked(client.post).mock.calls) {
        expect(call[0]).toBe('/certificates/bulk-delete');
        expect(call[2]).toEqual({ timeout: 300000 });
      }
      expect(result).toEqual({
        deleted: ['uuid-0', 'uuid-200', 'uuid-400'],
        failed: [
          { uuid: 'uuid-1', reason: 'in_use' },
          { uuid: 'uuid-201', reason: 'in_use' },
          { uuid: 'uuid-401', reason: 'in_use' },
        ],
      });
    });

    it('returns a merged partial result when a later chunk rejects', async () => {
      const uuids = Array.from({ length: 450 }, (_, i) => `uuid-${i}`);
      vi.mocked(client.post)
        .mockResolvedValueOnce({
          data: { deleted: ['uuid-0'], failed: [{ uuid: 'uuid-1', reason: 'in_use' }] },
        })
        .mockRejectedValueOnce(new Error('backup failed'));

      const result = await bulkDeleteCertificates(uuids);

      expect(client.post).toHaveBeenCalledTimes(2);
      expect(result.deleted).toEqual(['uuid-0']);
      expect(result.failed).toEqual([
        { uuid: 'uuid-1', reason: 'in_use' },
        ...uuids.slice(200).map((uuid) => ({ uuid, reason: 'error' })),
      ]);
    });

    it('rethrows when the first chunk rejects and sends no further requests', async () => {
      const uuids = Array.from({ length: 250 }, (_, i) => `uuid-${i}`);
      const error = new Error('backup failed');
      vi.mocked(client.post).mockRejectedValueOnce(error);

      await expect(bulkDeleteCertificates(uuids)).rejects.toBe(error);
      expect(client.post).toHaveBeenCalledTimes(1);
    });
  });
});
