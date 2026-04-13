import { describe, it, expect, vi, beforeEach } from 'vitest';

import {
  getCertificates,
  getCertificateDetail,
  uploadCertificate,
  updateCertificate,
  deleteCertificate,
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
});
