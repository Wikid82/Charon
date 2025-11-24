import { describe, it, expect, vi, beforeEach } from 'vitest';
import client from '../client';
import { getCertificates, uploadCertificate, deleteCertificate, Certificate } from '../certificates';

vi.mock('../client', () => ({
  default: {
    get: vi.fn(),
    post: vi.fn(),
    delete: vi.fn(),
  },
}));

describe('certificates API', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  const mockCert: Certificate = {
    id: 1,
    domain: 'example.com',
    issuer: 'Let\'s Encrypt',
    expires_at: '2023-01-01',
    status: 'valid',
    provider: 'letsencrypt',
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
    await deleteCertificate(1);
    expect(client.delete).toHaveBeenCalledWith('/certificates/1');
  });
});
