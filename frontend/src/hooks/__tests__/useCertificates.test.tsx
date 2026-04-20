import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { renderHook, waitFor } from '@testing-library/react';
import { describe, it, expect, vi, beforeEach } from 'vitest';

import * as api from '../../api/certificates';
import type { Certificate, CertificateDetail } from '../../api/certificates';
import {
  useCertificates,
  useCertificateDetail,
  useUploadCertificate,
  useUpdateCertificate,
  useDeleteCertificate,
  useExportCertificate,
  useValidateCertificate,
  useBulkDeleteCertificates,
} from '../useCertificates';

vi.mock('../../api/certificates');

const createWrapper = () => {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false },
    },
  });
  return ({ children }: { children: React.ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );
};

const mockCert: Certificate = {
  uuid: 'abc-123',
  domains: 'example.com',
  issuer: "Let's Encrypt",
  expires_at: '2025-01-01',
  status: 'valid',
  provider: 'letsencrypt',
  has_key: true,
  in_use: false,
};

const mockDetail: CertificateDetail = {
  ...mockCert,
  assigned_hosts: [],
  chain: [],
  auto_renew: false,
  created_at: '2024-01-01',
  updated_at: '2024-01-01',
};

describe('useCertificates hooks', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe('useCertificates', () => {
    it('fetches certificate list', async () => {
      vi.mocked(api.getCertificates).mockResolvedValue([mockCert]);

      const { result } = renderHook(() => useCertificates(), {
        wrapper: createWrapper(),
      });

      await waitFor(() => expect(result.current.isLoading).toBe(false));
      expect(result.current.certificates).toEqual([mockCert]);
    });

    it('returns empty array when no data', async () => {
      vi.mocked(api.getCertificates).mockResolvedValue([]);

      const { result } = renderHook(() => useCertificates(), {
        wrapper: createWrapper(),
      });

      await waitFor(() => expect(result.current.isLoading).toBe(false));
      expect(result.current.certificates).toEqual([]);
    });
  });

  describe('useCertificateDetail', () => {
    it('fetches certificate detail by uuid', async () => {
      vi.mocked(api.getCertificateDetail).mockResolvedValue(mockDetail);

      const { result } = renderHook(() => useCertificateDetail('abc-123'), {
        wrapper: createWrapper(),
      });

      await waitFor(() => expect(result.current.isLoading).toBe(false));
      expect(result.current.detail).toEqual(mockDetail);
    });

    it('does not fetch when uuid is null', () => {
      const { result } = renderHook(() => useCertificateDetail(null), {
        wrapper: createWrapper(),
      });

      expect(api.getCertificateDetail).not.toHaveBeenCalled();
      expect(result.current.detail).toBeUndefined();
    });
  });

  describe('useUploadCertificate', () => {
    it('uploads certificate and invalidates cache', async () => {
      vi.mocked(api.uploadCertificate).mockResolvedValue(mockCert);

      const { result } = renderHook(() => useUploadCertificate(), {
        wrapper: createWrapper(),
      });

      result.current.mutate({
        name: 'My Cert',
        certFile: new File(['cert'], 'cert.pem'),
        keyFile: new File(['key'], 'key.pem'),
      });

      await waitFor(() => expect(result.current.isSuccess).toBe(true));
      expect(api.uploadCertificate).toHaveBeenCalledWith(
        'My Cert',
        expect.any(File),
        expect.any(File),
        undefined,
      );
    });
  });

  describe('useUpdateCertificate', () => {
    it('updates certificate name', async () => {
      vi.mocked(api.updateCertificate).mockResolvedValue(mockCert);

      const { result } = renderHook(() => useUpdateCertificate(), {
        wrapper: createWrapper(),
      });

      result.current.mutate({ uuid: 'abc-123', name: 'Updated' });

      await waitFor(() => expect(result.current.isSuccess).toBe(true));
      expect(api.updateCertificate).toHaveBeenCalledWith('abc-123', 'Updated');
    });
  });

  describe('useDeleteCertificate', () => {
    it('deletes certificate and invalidates cache', async () => {
      vi.mocked(api.deleteCertificate).mockResolvedValue(undefined);

      const { result } = renderHook(() => useDeleteCertificate(), {
        wrapper: createWrapper(),
      });

      result.current.mutate('abc-123');

      await waitFor(() => expect(result.current.isSuccess).toBe(true));
      expect(api.deleteCertificate).toHaveBeenCalledWith('abc-123');
    });
  });

  describe('useExportCertificate', () => {
    it('exports certificate as blob', async () => {
      const blob = new Blob(['data']);
      vi.mocked(api.exportCertificate).mockResolvedValue(blob);

      const { result } = renderHook(() => useExportCertificate(), {
        wrapper: createWrapper(),
      });

      result.current.mutate({
        uuid: 'abc-123',
        format: 'pem',
        includeKey: true,
        password: 'pass',
      });

      await waitFor(() => expect(result.current.isSuccess).toBe(true));
      expect(api.exportCertificate).toHaveBeenCalledWith('abc-123', 'pem', true, 'pass', undefined);
    });
  });

  describe('useValidateCertificate', () => {
    it('validates certificate files', async () => {
      const validation = {
        valid: true,
        common_name: 'example.com',
        domains: ['example.com'],
        issuer_org: 'LE',
        expires_at: '2025-01-01',
        key_match: true,
        chain_valid: true,
        chain_depth: 1,
        warnings: [],
        errors: [],
      };
      vi.mocked(api.validateCertificate).mockResolvedValue(validation);

      const { result } = renderHook(() => useValidateCertificate(), {
        wrapper: createWrapper(),
      });

      result.current.mutate({
        certFile: new File(['cert'], 'cert.pem'),
        keyFile: new File(['key'], 'key.pem'),
      });

      await waitFor(() => expect(result.current.isSuccess).toBe(true));
      expect(api.validateCertificate).toHaveBeenCalled();
    });
  });

  describe('useBulkDeleteCertificates', () => {
    it('deletes multiple certificates', async () => {
      vi.mocked(api.deleteCertificate).mockResolvedValue(undefined);

      const { result } = renderHook(() => useBulkDeleteCertificates(), {
        wrapper: createWrapper(),
      });

      result.current.mutate(['uuid-1', 'uuid-2']);

      await waitFor(() => expect(result.current.isSuccess).toBe(true));
      expect(api.deleteCertificate).toHaveBeenCalledTimes(2);
      expect(result.current.data).toEqual({ succeeded: 2, failed: 0 });
    });

    it('reports partial failures', async () => {
      vi.mocked(api.deleteCertificate)
        .mockResolvedValueOnce(undefined)
        .mockRejectedValueOnce(new Error('fail'));

      const { result } = renderHook(() => useBulkDeleteCertificates(), {
        wrapper: createWrapper(),
      });

      result.current.mutate(['uuid-1', 'uuid-2']);

      await waitFor(() => expect(result.current.isSuccess).toBe(true));
      expect(result.current.data).toEqual({ succeeded: 1, failed: 1 });
    });
  });
});
