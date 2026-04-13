import { QueryClientProvider } from '@tanstack/react-query'
import { render, screen } from '@testing-library/react'
import { describe, it, expect, vi, beforeEach } from 'vitest'

import type { Certificate, CertificateDetail } from '../../../api/certificates'
import { useCertificateDetail } from '../../../hooks/useCertificates'
import { createTestQueryClient } from '../../../test/createTestQueryClient'
import CertificateDetailDialog from '../CertificateDetailDialog'

const mockDetail: CertificateDetail = {
  uuid: 'cert-1',
  name: 'My Cert',
  common_name: 'app.example.com',
  domains: 'app.example.com, api.example.com',
  issuer: 'Test CA',
  issuer_org: 'Test Org',
  fingerprint: 'AA:BB:CC:DD',
  serial_number: '1234567890',
  key_type: 'RSA 2048',
  expires_at: '2026-06-01T00:00:00Z',
  not_before: '2024-03-15T00:00:00Z',
  status: 'valid',
  provider: 'custom',
  has_key: true,
  in_use: true,
  auto_renew: false,
  created_at: '2024-01-01T00:00:00Z',
  updated_at: '2024-08-20T00:00:00Z',
  assigned_hosts: [
    { uuid: 'host-1', name: 'Web Server', domain_names: 'web.example.com' },
  ],
  chain: [
    { subject: 'app.example.com', issuer: 'Test CA', expires_at: '2026-06-01T00:00:00Z' },
    { subject: 'Test CA', issuer: 'Root CA', expires_at: '2030-01-01T00:00:00Z' },
  ],
}

vi.mock('../../../hooks/useCertificates', () => ({
  useCertificateDetail: vi.fn((uuid: string | null) => {
    if (!uuid) return { detail: undefined, isLoading: false }
    return { detail: mockDetail, isLoading: false }
  }),
}))

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string) => key,
    i18n: { language: 'en', changeLanguage: vi.fn() },
  }),
}))

const baseCert: Certificate = {
  uuid: 'cert-1',
  name: 'My Cert',
  domains: 'example.com',
  issuer: 'Test CA',
  expires_at: '2026-06-01T00:00:00Z',
  status: 'valid',
  provider: 'custom',
  has_key: true,
  in_use: true,
}

function renderDialog(
  certificate: Certificate | null = baseCert,
  open = true,
  onOpenChange = vi.fn(),
) {
  const qc = createTestQueryClient()
  return {
    onOpenChange,
    ...render(
      <QueryClientProvider client={qc}>
        <CertificateDetailDialog
          certificate={certificate}
          open={open}
          onOpenChange={onOpenChange}
        />
      </QueryClientProvider>,
    ),
  }
}

describe('CertificateDetailDialog', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('renders dialog with title when open', () => {
    renderDialog()
    expect(screen.getByTestId('certificate-detail-dialog')).toBeTruthy()
    expect(screen.getByText('certificates.detailTitle')).toBeTruthy()
  })

  it('does not render when closed', () => {
    renderDialog(baseCert, false)
    expect(screen.queryByTestId('certificate-detail-dialog')).toBeFalsy()
  })

  it('displays certificate name', () => {
    renderDialog()
    expect(screen.getByText('My Cert')).toBeTruthy()
  })

  it('displays common name', () => {
    renderDialog()
    const matches = screen.getAllByText(/app\.example\.com/)
    expect(matches.length).toBeGreaterThanOrEqual(1)
  })

  it('displays fingerprint', () => {
    renderDialog()
    expect(screen.getByText('AA:BB:CC:DD')).toBeTruthy()
  })

  it('displays serial number', () => {
    renderDialog()
    expect(screen.getByText('1234567890')).toBeTruthy()
  })

  it('displays key type', () => {
    renderDialog()
    expect(screen.getByText('RSA 2048')).toBeTruthy()
  })

  it('displays status', () => {
    renderDialog()
    expect(screen.getByText('valid')).toBeTruthy()
  })

  it('displays provider', () => {
    renderDialog()
    expect(screen.getByText('custom')).toBeTruthy()
  })

  it('displays assigned hosts section', () => {
    renderDialog()
    expect(screen.getByText('certificates.assignedHosts')).toBeTruthy()
    expect(screen.getByText('Web Server')).toBeTruthy()
  })

  it('displays certificate chain section', () => {
    renderDialog()
    expect(screen.getByText('certificates.certificateChain')).toBeTruthy()
  })

  it('shows auto renew status', () => {
    renderDialog()
    expect(screen.getByText('common.no')).toBeTruthy()
  })

  it('shows formatted dates', () => {
    renderDialog()
    const notBeforeDate = new Date('2024-03-15T00:00:00Z').toLocaleDateString()
    const updatedDate = new Date('2024-08-20T00:00:00Z').toLocaleDateString()
    expect(screen.getByText(notBeforeDate)).toBeTruthy()
    expect(screen.getByText(updatedDate)).toBeTruthy()
  })

  it('shows loading state', () => {
    vi.mocked(useCertificateDetail).mockReturnValue({
      detail: undefined as unknown as CertificateDetail,
      isLoading: true,
    })
    renderDialog()
    expect(screen.getByTestId('certificate-detail-dialog')).toBeTruthy()
    // Detail content should not be rendered while loading
    expect(screen.queryByText('My Cert')).toBeFalsy()
  })

  it('shows dash for missing optional fields', () => {
    const sparseDetail: CertificateDetail = {
      ...mockDetail,
      name: '',
      common_name: '',
      domains: '',
      issuer_org: '',
      issuer: '',
      fingerprint: '',
      serial_number: '',
      key_type: '',
      not_before: '',
      expires_at: '',
      created_at: '',
      updated_at: '',
      chain: [],
      assigned_hosts: [],
    }
    vi.mocked(useCertificateDetail).mockReturnValue({
      detail: sparseDetail,
      isLoading: false,
    })
    renderDialog()
    const dashes = screen.getAllByText('-')
    // Many fields should fall back to '-' when empty
    expect(dashes.length).toBeGreaterThanOrEqual(8)
  })

  it('shows no assigned hosts message when empty', () => {
    const noHostDetail: CertificateDetail = {
      ...mockDetail,
      assigned_hosts: [],
    }
    vi.mocked(useCertificateDetail).mockReturnValue({
      detail: noHostDetail,
      isLoading: false,
    })
    renderDialog()
    expect(screen.getByText('certificates.noAssignedHosts')).toBeTruthy()
  })

  it('shows auto renew yes when enabled', () => {
    const autoRenewDetail: CertificateDetail = {
      ...mockDetail,
      auto_renew: true,
    }
    vi.mocked(useCertificateDetail).mockReturnValue({
      detail: autoRenewDetail,
      isLoading: false,
    })
    renderDialog()
    expect(screen.getByText('common.yes')).toBeTruthy()
  })

  it('falls back to issuer when issuer_org is missing', () => {
    const noOrgDetail: CertificateDetail = {
      ...mockDetail,
      issuer_org: '',
      issuer: 'Fallback Issuer',
    }
    vi.mocked(useCertificateDetail).mockReturnValue({
      detail: noOrgDetail,
      isLoading: false,
    })
    renderDialog()
    expect(screen.getByText('Fallback Issuer')).toBeTruthy()
  })

  it('renders nothing when certificate is null', () => {
    vi.mocked(useCertificateDetail).mockReturnValue({
      detail: undefined as unknown as CertificateDetail,
      isLoading: false,
    })
    renderDialog(null)
    expect(screen.queryByText('My Cert')).toBeFalsy()
  })
})
