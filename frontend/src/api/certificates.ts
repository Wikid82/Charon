import client from './client'

export interface Certificate {
  id?: number
  name?: string
  domain: string
  issuer: string
  expires_at: string
  status: 'valid' | 'expiring' | 'expired'
  provider: string
}

export async function getCertificates(): Promise<Certificate[]> {
  const response = await client.get<Certificate[]>('/certificates')
  return response.data
}

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

export async function deleteCertificate(id: number): Promise<void> {
  await client.delete(`/certificates/${id}`)
}
