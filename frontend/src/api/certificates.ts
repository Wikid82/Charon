import client from './client'

/** Represents an SSL/TLS certificate. */
export interface Certificate {
  id?: number
  name?: string
  domain: string
  issuer: string
  expires_at: string
  status: 'valid' | 'expiring' | 'expired' | 'untrusted'
  provider: string
}

/**
 * Fetches all SSL certificates.
 * @returns Promise resolving to array of Certificate objects
 * @throws {AxiosError} If the request fails
 */
export async function getCertificates(): Promise<Certificate[]> {
  const response = await client.get<Certificate[]>('/certificates')
  return response.data
}

/**
 * Uploads a new SSL certificate with its private key.
 * @param name - Display name for the certificate
 * @param certFile - The certificate file (PEM format)
 * @param keyFile - The private key file (PEM format)
 * @returns Promise resolving to the created Certificate
 * @throws {AxiosError} If upload fails or certificate is invalid
 */
export async function uploadCertificate(name: string, certFile: File, keyFile: File): Promise<Certificate> {
  const formData = new FormData()
  formData.append('name', name)
  formData.append('certificate_file', certFile)
  formData.append('key_file', keyFile)

  const response = await client.post<Certificate>('/certificates', formData, {
    headers: {
      'Content-Type': 'multipart/form-data',
    },
  })
  return response.data
}

/**
 * Deletes an SSL certificate.
 * @param id - The ID of the certificate to delete
 * @throws {AxiosError} If deletion fails or certificate not found
 */
export async function deleteCertificate(id: number): Promise<void> {
  await client.delete(`/certificates/${id}`)
}
