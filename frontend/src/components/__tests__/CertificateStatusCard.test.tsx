import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import { BrowserRouter } from 'react-router-dom'
import CertificateStatusCard from '../CertificateStatusCard'
import type { Certificate } from '../../api/certificates'
import type { ProxyHost } from '../../api/proxyHosts'

const mockCert: Certificate = {
  id: 1,
  name: 'Test Cert',
  domain: 'example.com',
  issuer: "Let's Encrypt",
  expires_at: new Date(Date.now() + 90 * 24 * 60 * 60 * 1000).toISOString(),
  status: 'valid',
  provider: 'letsencrypt',
}

const mockHost: ProxyHost = {
  uuid: 'test-uuid',
  name: 'Test Host',
  domain_names: 'example.com',
  forward_scheme: 'http',
  forward_host: 'localhost',
  forward_port: 8080,
  ssl_forced: false,
  enabled: true,
  certificate_id: null,
  access_list_id: null,
  created_at: new Date().toISOString(),
  updated_at: new Date().toISOString(),
  http2_support: false,
  hsts_enabled: false,
  hsts_subdomains: false,
  block_exploits: false,
  websocket_support: false,
  application: 'none',
  locations: [],
}

function renderWithRouter(ui: React.ReactNode) {
  return render(<BrowserRouter>{ui}</BrowserRouter>)
}

describe('CertificateStatusCard', () => {
  it('shows total certificate count', () => {
    const certs: Certificate[] = [mockCert, { ...mockCert, id: 2 }, { ...mockCert, id: 3 }]
    renderWithRouter(<CertificateStatusCard certificates={certs} hosts={[]} />)

    expect(screen.getByText('3')).toBeInTheDocument()
    expect(screen.getByText('SSL Certificates')).toBeInTheDocument()
  })

  it('shows valid certificate count', () => {
    const certs: Certificate[] = [
      { ...mockCert, status: 'valid' },
      { ...mockCert, id: 2, status: 'valid' },
      { ...mockCert, id: 3, status: 'expired' },
    ]
    renderWithRouter(<CertificateStatusCard certificates={certs} hosts={[]} />)

    expect(screen.getByText('2 valid')).toBeInTheDocument()
  })

  it('shows expiring count when certificates are expiring', () => {
    const certs: Certificate[] = [
      { ...mockCert, status: 'expiring' },
      { ...mockCert, id: 2, status: 'valid' },
    ]
    renderWithRouter(<CertificateStatusCard certificates={certs} hosts={[]} />)

    expect(screen.getByText('1 expiring')).toBeInTheDocument()
  })

  it('hides expiring count when no certificates are expiring', () => {
    const certs: Certificate[] = [{ ...mockCert, status: 'valid' }]
    renderWithRouter(<CertificateStatusCard certificates={certs} hosts={[]} />)

    expect(screen.queryByText(/expiring/)).not.toBeInTheDocument()
  })

  it('shows staging count for untrusted certificates', () => {
    const certs: Certificate[] = [
      { ...mockCert, status: 'untrusted' },
      { ...mockCert, id: 2, status: 'untrusted' },
    ]
    renderWithRouter(<CertificateStatusCard certificates={certs} hosts={[]} />)

    expect(screen.getByText('2 staging')).toBeInTheDocument()
  })

  it('hides staging count when no untrusted certificates', () => {
    const certs: Certificate[] = [{ ...mockCert, status: 'valid' }]
    renderWithRouter(<CertificateStatusCard certificates={certs} hosts={[]} />)

    expect(screen.queryByText(/staging/)).not.toBeInTheDocument()
  })

  it('shows pending indicator when hosts lack certificates', () => {
    const hosts: ProxyHost[] = [
      { ...mockHost, ssl_forced: true, certificate_id: null, enabled: true },
      { ...mockHost, uuid: 'h2', ssl_forced: true, certificate_id: 1, enabled: true },
    ]
    renderWithRouter(<CertificateStatusCard certificates={[mockCert]} hosts={hosts} />)

    expect(screen.getByText('1 host awaiting certificate')).toBeInTheDocument()
  })

  it('shows plural for multiple pending hosts', () => {
    const hosts: ProxyHost[] = [
      { ...mockHost, uuid: 'h1', ssl_forced: true, certificate_id: null, enabled: true },
      { ...mockHost, uuid: 'h2', ssl_forced: true, certificate_id: null, enabled: true },
      { ...mockHost, uuid: 'h3', ssl_forced: true, certificate_id: null, enabled: true },
    ]
    renderWithRouter(<CertificateStatusCard certificates={[mockCert]} hosts={hosts} />)

    expect(screen.getByText('3 hosts awaiting certificate')).toBeInTheDocument()
  })

  it('hides pending indicator when all hosts have certificates', () => {
    const hosts: ProxyHost[] = [
      { ...mockHost, ssl_forced: true, certificate_id: 1, enabled: true },
    ]
    renderWithRouter(<CertificateStatusCard certificates={[mockCert]} hosts={hosts} />)

    expect(screen.queryByText(/awaiting certificate/)).not.toBeInTheDocument()
  })

  it('ignores disabled hosts when calculating pending', () => {
    const hosts: ProxyHost[] = [
      { ...mockHost, uuid: 'h1', ssl_forced: true, certificate_id: null, enabled: false },
      { ...mockHost, uuid: 'h2', ssl_forced: true, certificate_id: 1, enabled: true },
    ]
    renderWithRouter(<CertificateStatusCard certificates={[mockCert]} hosts={hosts} />)

    expect(screen.queryByText(/awaiting certificate/)).not.toBeInTheDocument()
  })

  it('ignores hosts without SSL when calculating pending', () => {
    const hosts: ProxyHost[] = [
      { ...mockHost, uuid: 'h1', ssl_forced: false, certificate_id: null, enabled: true },
      { ...mockHost, uuid: 'h2', ssl_forced: true, certificate_id: 1, enabled: true },
    ]
    renderWithRouter(<CertificateStatusCard certificates={[mockCert]} hosts={hosts} />)

    expect(screen.queryByText(/awaiting certificate/)).not.toBeInTheDocument()
  })

  it('calculates progress percentage correctly', () => {
    const hosts: ProxyHost[] = [
      { ...mockHost, uuid: 'h1', ssl_forced: true, certificate_id: 1, enabled: true },
      { ...mockHost, uuid: 'h2', ssl_forced: true, certificate_id: null, enabled: true },
      { ...mockHost, uuid: 'h3', ssl_forced: true, certificate_id: 1, enabled: true },
      { ...mockHost, uuid: 'h4', ssl_forced: true, certificate_id: null, enabled: true },
    ]
    renderWithRouter(<CertificateStatusCard certificates={[mockCert]} hosts={hosts} />)

    // 2 out of 4 = 50%
    expect(screen.getByText('50% provisioned')).toBeInTheDocument()
  })

  it('shows spinning loader icon when pending', () => {
    const hosts: ProxyHost[] = [
      { ...mockHost, ssl_forced: true, certificate_id: null, enabled: true },
    ]
    const { container } = renderWithRouter(
      <CertificateStatusCard certificates={[mockCert]} hosts={hosts} />
    )

    const spinner = container.querySelector('.animate-spin')
    expect(spinner).toBeInTheDocument()
  })

  it('links to certificates page', () => {
    renderWithRouter(<CertificateStatusCard certificates={[mockCert]} hosts={[]} />)

    const link = screen.getByRole('link')
    expect(link).toHaveAttribute('href', '/certificates')
  })

  it('handles empty certificates array', () => {
    renderWithRouter(<CertificateStatusCard certificates={[]} hosts={[]} />)

    expect(screen.getByText('0')).toBeInTheDocument()
    expect(screen.getByText('0 valid')).toBeInTheDocument()
  })
})
