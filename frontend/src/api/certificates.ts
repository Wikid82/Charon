import client from './client'

export interface Certificate {
  uuid: string
  name?: string
  common_name?: string
  domains: string
  issuer: string
  issuer_org?: string
  fingerprint?: string
  serial_number?: string
  key_type?: string
  expires_at: string
  not_before?: string
  status: 'valid' | 'expiring' | 'expired' | 'untrusted'
  provider: string
  chain_depth?: number
  has_key: boolean
  in_use: boolean
  /** @deprecated Use uuid instead */
  id?: number
}

export interface AssignedHost {
  uuid: string
  name: string
  domain_names: string
}

export interface ChainEntry {
  subject: string
  issuer: string
  expires_at: string
}

export interface CertificateDetail extends Certificate {
  assigned_hosts: AssignedHost[]
  chain: ChainEntry[]
  auto_renew: boolean
  created_at: string
  updated_at: string
}

export interface ValidationResult {
  valid: boolean
  common_name: string
  domains: string[]
  issuer_org: string
  expires_at: string
  key_match: boolean
  chain_valid: boolean
  chain_depth: number
  warnings: string[]
  errors: string[]
}

export async function getCertificates(): Promise<Certificate[]> {
  const response = await client.get<Certificate[]>('/certificates')
  return response.data
}

export async function getCertificateDetail(uuid: string): Promise<CertificateDetail> {
  const response = await client.get<CertificateDetail>(`/certificates/${uuid}`)
  return response.data
}

export async function uploadCertificate(
  name: string,
  certFile: File,
  keyFile?: File,
  chainFile?: File,
): Promise<Certificate> {
  const formData = new FormData()
  formData.append('name', name)
  formData.append('certificate_file', certFile)
  if (keyFile) formData.append('key_file', keyFile)
  if (chainFile) formData.append('chain_file', chainFile)

  const response = await client.post<Certificate>('/certificates', formData, {
    headers: { 'Content-Type': 'multipart/form-data' },
  })
  return response.data
}

export async function updateCertificate(uuid: string, name: string): Promise<Certificate> {
  const response = await client.put<Certificate>(`/certificates/${uuid}`, { name })
  return response.data
}

export async function deleteCertificate(uuid: string): Promise<void> {
  await client.delete(`/certificates/${uuid}`)
}

export type BulkDeleteFailureReason = 'in_use' | 'not_found' | 'error'

export interface BulkDeleteFailure {
  uuid: string
  reason: BulkDeleteFailureReason
}

export interface BulkDeleteCertificatesResponse {
  deleted: string[]
  failed: BulkDeleteFailure[]
}

/**
 * The backend takes one backup before deleting the batch, which can take ~45s on
 * large databases, so this request overrides the global 30s client timeout.
 */
const BULK_DELETE_TIMEOUT_MS = 300000

/**
 * Maximum uuids the backend accepts in one bulk-delete request. Larger selections
 * are split into sequential chunks, each with its own request and backup.
 */
export const BULK_DELETE_MAX = 200

async function postBulkDeleteChunk(uuids: string[]): Promise<BulkDeleteCertificatesResponse> {
  const response = await client.post<BulkDeleteCertificatesResponse>(
    '/certificates/bulk-delete',
    { uuids },
    { timeout: BULK_DELETE_TIMEOUT_MS },
  )
  return response.data
}

/**
 * Deletes several certificates in sequential chunks of at most BULK_DELETE_MAX uuids.
 *
 * If the first chunk's request rejects, nothing was deleted and the error is rethrown.
 * If a later chunk rejects, earlier results are kept and every uuid in the rejected
 * chunk and in the remaining chunks is reported as failed with reason 'error'.
 */
export async function bulkDeleteCertificates(
  uuids: string[],
): Promise<BulkDeleteCertificatesResponse> {
  const merged: BulkDeleteCertificatesResponse = { deleted: [], failed: [] }

  for (let start = 0; start < uuids.length; start += BULK_DELETE_MAX) {
    const chunk = uuids.slice(start, start + BULK_DELETE_MAX)
    let response: BulkDeleteCertificatesResponse
    try {
      response = await postBulkDeleteChunk(chunk)
    } catch (error) {
      if (start === 0) throw error
      for (const uuid of uuids.slice(start)) {
        merged.failed.push({ uuid, reason: 'error' })
      }
      return merged
    }
    merged.deleted.push(...response.deleted)
    merged.failed.push(...response.failed)
  }

  return merged
}

export async function exportCertificate(
  uuid: string,
  format: string,
  includeKey: boolean,
  password?: string,
  pfxPassword?: string,
): Promise<Blob> {
  const response = await client.post(
    `/certificates/${uuid}/export`,
    { format, include_key: includeKey, password, pfx_password: pfxPassword },
    { responseType: 'blob' },
  )
  return response.data as Blob
}

export async function validateCertificate(
  certFile: File,
  keyFile?: File,
  chainFile?: File,
): Promise<ValidationResult> {
  const formData = new FormData()
  formData.append('certificate_file', certFile)
  if (keyFile) formData.append('key_file', keyFile)
  if (chainFile) formData.append('chain_file', chainFile)

  const response = await client.post<ValidationResult>('/certificates/validate', formData, {
    headers: { 'Content-Type': 'multipart/form-data' },
  })
  return response.data
}
