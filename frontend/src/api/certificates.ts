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
