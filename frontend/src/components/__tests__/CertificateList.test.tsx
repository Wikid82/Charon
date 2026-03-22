import { QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, it, expect, vi, beforeEach } from 'vitest'

import { useCertificates } from '../../hooks/useCertificates'
import { useProxyHosts } from '../../hooks/useProxyHosts'
import { createTestQueryClient } from '../../test/createTestQueryClient'
import CertificateList, { isDeletable, isInUse } from '../CertificateList'

import type { Certificate } from '../../api/certificates'
import type { ProxyHost } from '../../api/proxyHosts'

vi.mock('../../hooks/useCertificates', () => ({
  useCertificates: vi.fn(),
}))

vi.mock('../../api/certificates', () => ({
  deleteCertificate: vi.fn(async () => {}),
}))

vi.mock('../../api/backups', () => ({
  createBackup: vi.fn(async () => ({ filename: 'backup-cert' })),
}))

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string) => key,
    i18n: { language: 'en', changeLanguage: vi.fn() },
  }),
}))

vi.mock('../../hooks/useProxyHosts', () => ({
  useProxyHosts: vi.fn(),
}))

vi.mock('../../utils/toast', () => ({
  toast: { success: vi.fn(), error: vi.fn(), loading: vi.fn(), dismiss: vi.fn() },
}))

function renderWithClient(ui: React.ReactNode) {
  const qc = createTestQueryClient()
  return render(<QueryClientProvider client={qc}>{ui}</QueryClientProvider>)
}

const createCertificatesValue = (overrides: Partial<ReturnType<typeof useCertificates>> = {}) => {
  const certificates: Certificate[] = [
    { id: 1, name: 'CustomCert', domain: 'example.com', issuer: 'Custom CA', expires_at: '2026-03-01T00:00:00Z', status: 'expired', provider: 'custom' },
    { id: 2, name: 'LE Staging', domain: 'staging.example.com', issuer: "Let's Encrypt Staging", expires_at: '2026-04-01T00:00:00Z', status: 'untrusted', provider: 'letsencrypt-staging' },
    { id: 3, name: 'ActiveCert', domain: 'active.example.com', issuer: 'Custom CA', expires_at: '2026-02-01T00:00:00Z', status: 'valid', provider: 'custom' },
    { id: 4, name: 'UnusedValidCert', domain: 'unused.example.com', issuer: 'Custom CA', expires_at: '2026-05-01T00:00:00Z', status: 'valid', provider: 'custom' },
    { id: 5, name: 'ExpiredLE', domain: 'expired-le.example.com', issuer: "Let's Encrypt", expires_at: '2025-01-01T00:00:00Z', status: 'expired', provider: 'letsencrypt' },
    { id: 6, name: 'ValidLE', domain: 'valid-le.example.com', issuer: "Let's Encrypt", expires_at: '2026-12-01T00:00:00Z', status: 'valid', provider: 'letsencrypt' },
  ]

  return {
    certificates,
    isLoading: false,
    error: null,
    refetch: vi.fn(),
    ...overrides,
  }
}

const createProxyHost = (overrides: Partial<ProxyHost> = {}): ProxyHost => ({
  uuid: 'h1',
  name: 'Host1',
  domain_names: 'host1.example.com',
  forward_scheme: 'http',
  forward_host: '127.0.0.1',
  forward_port: 80,
  ssl_forced: false,
  http2_support: true,
  hsts_enabled: false,
  hsts_subdomains: false,
  block_exploits: false,
  websocket_support: false,
  application: 'none',
  locations: [],
  enabled: true,
  created_at: '2026-02-01T00:00:00Z',
  updated_at: '2026-02-01T00:00:00Z',
  certificate_id: 3,
  ...overrides,
})

const createProxyHostsValue = (overrides: Partial<ReturnType<typeof useProxyHosts>> = {}): ReturnType<typeof useProxyHosts> => ({
  hosts: [
    createProxyHost(),
  ],
  loading: false,
  isFetching: false,
  error: null,
  createHost: vi.fn(),
  updateHost: vi.fn(),
  deleteHost: vi.fn(),
  bulkUpdateACL: vi.fn(),
  bulkUpdateSecurityHeaders: vi.fn(),
  isCreating: false,
  isUpdating: false,
  isDeleting: false,
  isBulkUpdating: false,
  ...overrides,
})

const getRowNames = () =>
  screen
    .getAllByRole('row')
    .slice(1)
    .map(row => row.querySelector('td')?.textContent?.trim() ?? '')

beforeEach(() => {
  vi.clearAllMocks()
  vi.mocked(useCertificates).mockReturnValue(createCertificatesValue())
  vi.mocked(useProxyHosts).mockReturnValue(createProxyHostsValue())
})

describe('CertificateList', () => {
  describe('isDeletable', () => {
    const noHosts: ProxyHost[] = []
    const withHost = (certId: number): ProxyHost[] => [createProxyHost({ certificate_id: certId })]

    it('returns true for custom cert not in use', () => {
      const cert: Certificate = { id: 1, name: 'C', domain: 'd', issuer: 'X', expires_at: '', status: 'valid', provider: 'custom' }
      expect(isDeletable(cert, noHosts)).toBe(true)
    })

    it('returns true for staging cert not in use', () => {
      const cert: Certificate = { id: 2, name: 'S', domain: 'd', issuer: 'X', expires_at: '', status: 'untrusted', provider: 'letsencrypt-staging' }
      expect(isDeletable(cert, noHosts)).toBe(true)
    })

    it('returns true for expired LE cert not in use', () => {
      const cert: Certificate = { id: 3, name: 'E', domain: 'd', issuer: 'LE', expires_at: '', status: 'expired', provider: 'letsencrypt' }
      expect(isDeletable(cert, noHosts)).toBe(true)
    })

    it('returns false for valid LE cert not in use', () => {
      const cert: Certificate = { id: 4, name: 'V', domain: 'd', issuer: 'LE', expires_at: '', status: 'valid', provider: 'letsencrypt' }
      expect(isDeletable(cert, noHosts)).toBe(false)
    })

    it('returns false for cert in use', () => {
      const cert: Certificate = { id: 5, name: 'U', domain: 'd', issuer: 'X', expires_at: '', status: 'valid', provider: 'custom' }
      expect(isDeletable(cert, withHost(5))).toBe(false)
    })

    it('returns false for cert without id', () => {
      const cert: Certificate = { domain: 'd', issuer: 'X', expires_at: '', status: 'valid', provider: 'custom' }
      expect(isDeletable(cert, noHosts)).toBe(false)
    })

    it('returns false for expiring LE cert not in use', () => {
      const cert: Certificate = { id: 7, name: 'Exp', domain: 'd', issuer: 'LE', expires_at: '', status: 'expiring', provider: 'letsencrypt' }
      expect(isDeletable(cert, noHosts)).toBe(false)
    })
  })

  describe('isInUse', () => {
    it('returns true when host references cert by certificate_id', () => {
      const cert: Certificate = { id: 10, domain: 'd', issuer: 'X', expires_at: '', status: 'valid', provider: 'custom' }
      expect(isInUse(cert, [createProxyHost({ certificate_id: 10 })])).toBe(true)
    })

    it('returns true when host references cert via certificate.id', () => {
      const cert: Certificate = { id: 10, domain: 'd', issuer: 'X', expires_at: '', status: 'valid', provider: 'custom' }
      const host = createProxyHost({ certificate_id: undefined, certificate: { id: 10, uuid: 'u', name: 'c', provider: 'custom', domains: 'd', expires_at: '' } })
      expect(isInUse(cert, [host])).toBe(true)
    })

    it('returns false when no host references cert', () => {
      const cert: Certificate = { id: 99, domain: 'd', issuer: 'X', expires_at: '', status: 'valid', provider: 'custom' }
      expect(isInUse(cert, [createProxyHost({ certificate_id: 3 })])).toBe(false)
    })
  })

  it('renders delete button for deletable certs', async () => {
    renderWithClient(<CertificateList />)
    const rows = await screen.findAllByRole('row')
    const customRow = rows.find(r => r.querySelector('td')?.textContent?.includes('CustomCert'))!
    expect(within(customRow).getByRole('button', { name: 'certificates.deleteTitle' })).toBeInTheDocument()
  })

  it('renders delete button for expired LE cert not in use', async () => {
    renderWithClient(<CertificateList />)
    const rows = await screen.findAllByRole('row')
    const expiredLeRow = rows.find(r => r.querySelector('td')?.textContent?.includes('ExpiredLE'))!
    expect(within(expiredLeRow).getByRole('button', { name: 'certificates.deleteTitle' })).toBeInTheDocument()
  })

  it('renders aria-disabled delete button for in-use cert', async () => {
    renderWithClient(<CertificateList />)
    const rows = await screen.findAllByRole('row')
    const activeRow = rows.find(r => r.querySelector('td')?.textContent?.includes('ActiveCert'))!
    const btn = within(activeRow).getByRole('button', { name: 'certificates.deleteTitle' })
    expect(btn).toHaveAttribute('aria-disabled', 'true')
  })

  it('hides delete button for valid production LE cert', async () => {
    renderWithClient(<CertificateList />)
    const rows = await screen.findAllByRole('row')
    const validLeRow = rows.find(r => r.querySelector('td')?.textContent?.includes('ValidLE'))!
    expect(within(validLeRow).queryByRole('button', { name: 'certificates.deleteTitle' })).not.toBeInTheDocument()
  })

  it('opens dialog and deletes cert on confirm', async () => {
    const { deleteCertificate } = await import('../../api/certificates')
    const user = userEvent.setup()

    renderWithClient(<CertificateList />)
    const rows = await screen.findAllByRole('row')
    const customRow = rows.find(r => r.querySelector('td')?.textContent?.includes('CustomCert'))!
    await user.click(within(customRow).getByRole('button', { name: 'certificates.deleteTitle' }))

    const dialog = await screen.findByRole('dialog')
    expect(dialog).toBeInTheDocument()
    expect(within(dialog).getByText('certificates.deleteTitle')).toBeInTheDocument()

    await user.click(within(dialog).getByRole('button', { name: 'certificates.deleteButton' }))
    await waitFor(() => expect(deleteCertificate).toHaveBeenCalledWith(1))
  })

  it('does not call createBackup on delete (server handles it)', async () => {
    const { createBackup } = await import('../../api/backups')
    const user = userEvent.setup()

    renderWithClient(<CertificateList />)
    const rows = await screen.findAllByRole('row')
    const customRow = rows.find(r => r.querySelector('td')?.textContent?.includes('CustomCert'))!
    await user.click(within(customRow).getByRole('button', { name: 'certificates.deleteTitle' }))

    const dialog = await screen.findByRole('dialog')
    await user.click(within(dialog).getByRole('button', { name: 'certificates.deleteButton' }))
    await waitFor(() => expect(createBackup).not.toHaveBeenCalled())
  })

  it('renders empty state when no certificates exist', async () => {
    vi.mocked(useCertificates).mockReturnValue(createCertificatesValue({ certificates: [] }))
    renderWithClient(<CertificateList />)
    expect(await screen.findByText('No certificates found.')).toBeInTheDocument()
  })

  it('shows error state when certificate load fails', async () => {
    vi.mocked(useCertificates).mockReturnValue(createCertificatesValue({ error: new Error('boom') }))
    renderWithClient(<CertificateList />)
    expect(await screen.findByText('Failed to load certificates')).toBeInTheDocument()
  })

  it('shows error toast when delete mutation fails', async () => {
    const { deleteCertificate } = await import('../../api/certificates')
    const { toast } = await import('../../utils/toast')
    vi.mocked(deleteCertificate).mockRejectedValueOnce(new Error('Network error'))
    const user = userEvent.setup()

    renderWithClient(<CertificateList />)
    const rows = await screen.findAllByRole('row')
    const customRow = rows.find(r => r.querySelector('td')?.textContent?.includes('CustomCert'))!
    await user.click(within(customRow).getByRole('button', { name: 'certificates.deleteTitle' }))

    const dialog = await screen.findByRole('dialog')
    await user.click(within(dialog).getByRole('button', { name: 'certificates.deleteButton' }))

    await waitFor(() => expect(toast.error).toHaveBeenCalledWith('certificates.deleteFailed: Network error'))
  })

  it('clicking disabled delete button for in-use cert does not open dialog', async () => {
    const user = userEvent.setup()
    renderWithClient(<CertificateList />)
    const rows = await screen.findAllByRole('row')
    const activeRow = rows.find(r => r.querySelector('td')?.textContent?.includes('ActiveCert'))!
    const btn = within(activeRow).getByRole('button', { name: 'certificates.deleteTitle' })

    await user.click(btn)
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  })

  it('closes delete dialog when cancel is clicked', async () => {
    const user = userEvent.setup()
    renderWithClient(<CertificateList />)
    const rows = await screen.findAllByRole('row')
    const customRow = rows.find(r => r.querySelector('td')?.textContent?.includes('CustomCert'))!
    await user.click(within(customRow).getByRole('button', { name: 'certificates.deleteTitle' }))

    const dialog = await screen.findByRole('dialog')
    expect(dialog).toBeInTheDocument()

    await user.click(within(dialog).getByRole('button', { name: 'common.cancel' }))
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
  })

  it('sorts certificates by name and expiry when headers are clicked', async () => {
    const certificates: Certificate[] = [
      { id: 10, name: 'Zulu', domain: 'z.example.com', issuer: 'Custom CA', expires_at: '2026-03-01T00:00:00Z', status: 'valid', provider: 'custom' },
      { id: 11, name: 'Alpha', domain: 'a.example.com', issuer: 'Custom CA', expires_at: '2026-01-01T00:00:00Z', status: 'valid', provider: 'custom' },
    ]

    const user = userEvent.setup()

    vi.mocked(useCertificates).mockReturnValue(createCertificatesValue({ certificates }))
    renderWithClient(<CertificateList />)

    expect(getRowNames()).toEqual(['Alpha', 'Zulu'])

    await user.click(screen.getByText('Expires'))
    expect(getRowNames()).toEqual(['Alpha', 'Zulu'])

    await user.click(screen.getByText('Expires'))
    expect(getRowNames()).toEqual(['Zulu', 'Alpha'])
  })
})
