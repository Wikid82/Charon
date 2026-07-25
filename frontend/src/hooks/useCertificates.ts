import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'

import {
  getCertificates,
  getCertificateDetail,
  uploadCertificate,
  updateCertificate,
  deleteCertificate,
  exportCertificate,
  validateCertificate,
} from '../api/certificates'

import type { CertificateDetail } from '../api/certificates'

interface UseCertificatesOptions {
  refetchInterval?: number | false
}

export function useCertificates(options?: UseCertificatesOptions) {
  const { data, isLoading, error, refetch } = useQuery({
    queryKey: ['certificates'],
    queryFn: getCertificates,
    refetchInterval: options?.refetchInterval,
  })

  return {
    certificates: data || [],
    isLoading,
    error,
    refetch,
  }
}

export function useCertificateDetail(uuid: string | null) {
  const { data, isLoading, error } = useQuery({
    queryKey: ['certificates', uuid],
    queryFn: () => getCertificateDetail(uuid!),
    enabled: !!uuid,
  })

  return {
    detail: data as CertificateDetail | undefined,
    isLoading,
    error,
  }
}

export function useUploadCertificate() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (params: {
      name: string
      certFile: File
      keyFile?: File
      chainFile?: File
    }) => uploadCertificate(params.name, params.certFile, params.keyFile, params.chainFile),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['certificates'] })
    },
  })
}

export function useUpdateCertificate() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (params: { uuid: string; name: string }) =>
      updateCertificate(params.uuid, params.name),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['certificates'] })
    },
  })
}

export function useDeleteCertificate() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (uuid: string) => deleteCertificate(uuid),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['certificates'] })
      queryClient.invalidateQueries({ queryKey: ['proxyHosts'] })
    },
  })
}

export function useExportCertificate() {
  return useMutation({
    mutationFn: (params: {
      uuid: string
      format: string
      includeKey: boolean
      password?: string
      pfxPassword?: string
    }) =>
      exportCertificate(
        params.uuid,
        params.format,
        params.includeKey,
        params.password,
        params.pfxPassword,
      ),
  })
}

export function useValidateCertificate() {
  return useMutation({
    mutationFn: (params: {
      certFile: File
      keyFile?: File
      chainFile?: File
    }) => validateCertificate(params.certFile, params.keyFile, params.chainFile),
  })
}

export function useBulkDeleteCertificates() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: async (uuids: string[]) => {
      // Delete sequentially, not concurrently (Promise.allSettled(uuids.map(...))).
      // Each certificate delete triggers a backup on the backend
      // (CertificateHandler.Delete -> BackupService.CreateBackup), which is
      // guarded by a non-blocking TryLock (BackupService.CreateBackupWithOptions):
      // only one backup can run at a time, and a losing concurrent request fails
      // immediately with ErrBackupInProgress rather than queueing. Firing all
      // deletes at once meant only the first ever succeeded; the rest 500'd on
      // the backup step. Awaiting them one at a time lets each backup finish
      // (and release the lock) before the next delete starts.
      let succeeded = 0
      let failed = 0
      for (const uuid of uuids) {
        try {
          await deleteCertificate(uuid)
          succeeded++
        } catch {
          failed++
        }
      }
      return { succeeded, failed }
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['certificates'] })
      queryClient.invalidateQueries({ queryKey: ['proxyHosts'] })
    },
  })
}
