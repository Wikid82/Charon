import { QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, it, expect, vi, beforeEach } from 'vitest'

import { useCertificates } from '../../hooks/useCertificates'
import { useProxyHosts } from '../../hooks/useProxyHosts'
import { createTestQueryClient } from '../../test/createTestQueryClient'
import CertificateList from '../CertificateList'

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
  it('deletes custom certificate when confirmed', async () => {
    const confirmSpy = vi.spyOn(window, 'confirm').mockImplementation(() => true)
    const { deleteCertificate } = await import('../../api/certificates')
    const { createBackup } = await import('../../api/backups')
    const { toast } = await import('../../utils/toast')
    const user = userEvent.setup()

    renderWithClient(<CertificateList />)
    const rows = await screen.findAllByRole('row')
    const customRow = rows.find(r => r.querySelector('td')?.textContent?.includes('CustomCert')) as HTMLElement
    expect(customRow).toBeTruthy()
    const customBtn = customRow.querySelector('button[title="Delete Certificate"]') as HTMLButtonElement
    expect(customBtn).toBeTruthy()
    await user.click(customBtn)

    await waitFor(() => expect(createBackup).toHaveBeenCalled())
    await waitFor(() => expect(deleteCertificate).toHaveBeenCalledWith(1))
    await waitFor(() => expect(toast.success).toHaveBeenCalledWith('Certificate deleted'))
    confirmSpy.mockRestore()
  })

  it('deletes staging certificate when confirmed', async () => {
    const confirmSpy = vi.spyOn(window, 'confirm').mockImplementation(() => true)
    const { deleteCertificate } = await import('../../api/certificates')
    const user = userEvent.setup()

    renderWithClient(<CertificateList />)
    const stagingButtons = await screen.findAllByTitle('Delete Staging Certificate')
    expect(stagingButtons.length).toBeGreaterThan(0)
    await user.click(stagingButtons[0])

    await waitFor(() => expect(deleteCertificate).toHaveBeenCalledWith(2))
    confirmSpy.mockRestore()
  })

  it('deletes valid custom certificate when not in use', async () => {
    const confirmSpy = vi.spyOn(window, 'confirm').mockImplementation(() => true)
    const { deleteCertificate } = await import('../../api/certificates')
    const { createBackup } = await import('../../api/backups')
    const user = userEvent.setup()

    renderWithClient(<CertificateList />)
    const rows = await screen.findAllByRole('row')
    const unusedRow = rows.find(r => r.querySelector('td')?.textContent?.includes('UnusedValidCert')) as HTMLElement
    expect(unusedRow).toBeTruthy()
    const unusedButton = unusedRow.querySelector('button[title="Delete Certificate"]') as HTMLButtonElement
    expect(unusedButton).toBeTruthy()
    await user.click(unusedButton)

    await waitFor(() => expect(createBackup).toHaveBeenCalled())
    await waitFor(() => expect(deleteCertificate).toHaveBeenCalledWith(4))
    confirmSpy.mockRestore()
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
