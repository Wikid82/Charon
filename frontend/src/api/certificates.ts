import client from './client'

export interface Certificate {
  domain: string
  issuer: string
  expires_at: string
  status: 'valid' | 'expiring' | 'expired'
}

export async function getCertificates(): Promise<Certificate[]> {
  const response = await client.get<Certificate[]>('/certificates')
  return response.data
}
