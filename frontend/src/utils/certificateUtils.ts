import { type Certificate } from '../api/certificates'

export function isInUse(cert: Certificate): boolean {
  return cert.in_use
}

export function isDeletable(cert: Certificate): boolean {
  if (cert.in_use) return false
  return (
    cert.provider === 'custom' ||
    cert.provider === 'letsencrypt-staging' ||
    cert.status === 'expired' ||
    cert.status === 'expiring'
  )
}
