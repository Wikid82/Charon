import { render, screen } from '@testing-library/react'
import { BrowserRouter } from 'react-router-dom'
import { describe, it, expect } from 'vitest'

import CertificateStatusCard from '../CertificateStatusCard'

import type { Certificate } from '../../api/certificates'
import type { ProxyHost } from '../../api/proxyHosts'

const mockCert: Certificate = {
  uuid: 'cert-1',
  name: 'Test Cert',
  domains: 'example.com',
  issuer: "Let's Encrypt",
  expires_at: new Date(Date.now() + 90 * 24 * 60 * 60 * 1000).toISOString(),
  status: 'valid',
  provider: 'letsencrypt',
  has_key: true,
  in_use: false,
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

// Helper to create a certificate with a specific domain
function mockCertWithDomain(domain: string, status: 'valid' | 'expiring' | 'expired' | 'untrusted' = 'valid'): Certificate {
  return {
    uuid: `cert-${Math.random().toString(36).slice(2, 8)}`,
    name: domain,
    domains: domain,
    issuer: "Let's Encrypt",
    expires_at: new Date(Date.now() + 90 * 24 * 60 * 60 * 1000).toISOString(),
    status,
    provider: 'letsencrypt',
    has_key: true,
    in_use: false,
  }
}

function renderWithRouter(ui: React.ReactNode) {
  return render(<BrowserRouter>{ui}</BrowserRouter>)
}

describe('CertificateStatusCard', () => {
  it('shows total certificate count', () => {
    const certs: Certificate[] = [mockCert, { ...mockCert, uuid: 'cert-2' }, { ...mockCert, uuid: 'cert-3' }]
    renderWithRouter(<CertificateStatusCard certificates={certs} hosts={[]} />)

    expect(screen.getByText('3')).toBeInTheDocument()
    expect(screen.getByText('SSL Certificates')).toBeInTheDocument()
  })

  it('shows valid certificate count', () => {
    const certs: Certificate[] = [
      { ...mockCert, status: 'valid' },
      { ...mockCert, uuid: 'cert-2', status: 'valid' },
      { ...mockCert, uuid: 'cert-3', status: 'expired' },
    ]
    renderWithRouter(<CertificateStatusCard certificates={certs} hosts={[]} />)

    expect(screen.getByText('2 valid')).toBeInTheDocument()
  })

  it('shows expiring count when certificates are expiring', () => {
    const certs: Certificate[] = [
      { ...mockCert, status: 'expiring' },
      { ...mockCert, uuid: 'cert-2', status: 'valid' },
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
      { ...mockCert, uuid: 'cert-2', status: 'untrusted' },
    ]
    renderWithRouter(<CertificateStatusCard certificates={certs} hosts={[]} />)

    expect(screen.getByText('2 staging')).toBeInTheDocument()
  })

  it('hides staging count when no untrusted certificates', () => {
    const certs: Certificate[] = [{ ...mockCert, status: 'valid' }]
    renderWithRouter(<CertificateStatusCard certificates={certs} hosts={[]} />)

    expect(screen.queryByText(/staging/)).not.toBeInTheDocument()
  })

  it('shows spinning loader icon when pending', () => {
    const hosts: ProxyHost[] = [
      { ...mockHost, domain_names: 'other.com', ssl_forced: true, certificate_id: null, enabled: true },
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
    expect(screen.getByText('No certificates')).toBeInTheDocument()
  })
})

describe('CertificateStatusCard - Domain Matching', () => {
  it('does not show pending when host domain matches certificate domain', () => {
    const certs: Certificate[] = [mockCertWithDomain('example.com')]
    const hosts: ProxyHost[] = [
      { ...mockHost, domain_names: 'example.com', ssl_forced: true, certificate_id: null, enabled: true }
    ]
    renderWithRouter(<CertificateStatusCard certificates={certs} hosts={hosts} />)

    // Should NOT show "awaiting certificate" since domain matches
    expect(screen.queryByText(/awaiting certificate/)).not.toBeInTheDocument()
  })

  it('shows pending when host domain has no matching certificate', () => {
    const certs: Certificate[] = [mockCertWithDomain('other.com')]
    const hosts: ProxyHost[] = [
      { ...mockHost, domain_names: 'example.com', ssl_forced: true, certificate_id: null, enabled: true }
    ]
    renderWithRouter(<CertificateStatusCard certificates={certs} hosts={hosts} />)

    expect(screen.getByText('1 host awaiting certificate')).toBeInTheDocument()
  })

  it('shows plural for multiple pending hosts', () => {
    const certs: Certificate[] = [mockCertWithDomain('has-cert.com')]
    const hosts: ProxyHost[] = [
      { ...mockHost, uuid: 'h1', domain_names: 'no-cert-1.com', ssl_forced: true, certificate_id: null, enabled: true },
      { ...mockHost, uuid: 'h2', domain_names: 'no-cert-2.com', ssl_forced: true, certificate_id: null, enabled: true },
      { ...mockHost, uuid: 'h3', domain_names: 'no-cert-3.com', ssl_forced: true, certificate_id: null, enabled: true },
    ]
    renderWithRouter(<CertificateStatusCard certificates={certs} hosts={hosts} />)

    expect(screen.getByText('3 hosts awaiting certificate')).toBeInTheDocument()
  })

  it('handles case-insensitive domain matching', () => {
    const certs: Certificate[] = [mockCertWithDomain('EXAMPLE.COM')]
    const hosts: ProxyHost[] = [
      { ...mockHost, domain_names: 'example.com', ssl_forced: true, certificate_id: null, enabled: true }
    ]
    renderWithRouter(<CertificateStatusCard certificates={certs} hosts={hosts} />)

    expect(screen.queryByText(/awaiting certificate/)).not.toBeInTheDocument()
  })

  it('handles case-insensitive matching with host uppercase', () => {
    const certs: Certificate[] = [mockCertWithDomain('example.com')]
    const hosts: ProxyHost[] = [
      { ...mockHost, domain_names: 'EXAMPLE.COM', ssl_forced: true, certificate_id: null, enabled: true }
    ]
    renderWithRouter(<CertificateStatusCard certificates={certs} hosts={hosts} />)

    expect(screen.queryByText(/awaiting certificate/)).not.toBeInTheDocument()
  })

  it('handles multi-domain hosts with partial certificate coverage', () => {
    // Host has two domains, but only one has a certificate - should be "covered"
    const certs: Certificate[] = [mockCertWithDomain('example.com')]
    const hosts: ProxyHost[] = [
      { ...mockHost, domain_names: 'example.com, www.example.com', ssl_forced: true, certificate_id: null, enabled: true }
    ]
    renderWithRouter(<CertificateStatusCard certificates={certs} hosts={hosts} />)

    // Host should be considered "covered" if any domain has a cert
    expect(screen.queryByText(/awaiting certificate/)).not.toBeInTheDocument()
  })

  it('handles comma-separated certificate domains', () => {
    const certs: Certificate[] = [{
      ...mockCertWithDomain('example.com'),
      domains: 'example.com, www.example.com'
    }]
    const hosts: ProxyHost[] = [
      { ...mockHost, domain_names: 'www.example.com', ssl_forced: true, certificate_id: null, enabled: true }
    ]
    renderWithRouter(<CertificateStatusCard certificates={certs} hosts={hosts} />)

    expect(screen.queryByText(/awaiting certificate/)).not.toBeInTheDocument()
  })

  it('ignores disabled hosts even without certificate', () => {
    const certs: Certificate[] = []
    const hosts: ProxyHost[] = [
      { ...mockHost, domain_names: 'example.com', ssl_forced: true, certificate_id: null, enabled: false }
    ]
    renderWithRouter(<CertificateStatusCard certificates={certs} hosts={hosts} />)

    expect(screen.queryByText(/awaiting certificate/)).not.toBeInTheDocument()
  })

  it('ignores hosts without SSL forced', () => {
    const certs: Certificate[] = []
    const hosts: ProxyHost[] = [
      { ...mockHost, domain_names: 'example.com', ssl_forced: false, certificate_id: null, enabled: true }
    ]
    renderWithRouter(<CertificateStatusCard certificates={certs} hosts={hosts} />)

    expect(screen.queryByText(/awaiting certificate/)).not.toBeInTheDocument()
  })

  it('calculates progress percentage with domain matching', () => {
    const certs: Certificate[] = [
      mockCertWithDomain('a.example.com'),
      mockCertWithDomain('b.example.com'),
    ]
    const hosts: ProxyHost[] = [
      { ...mockHost, uuid: 'h1', domain_names: 'a.example.com', ssl_forced: true, certificate_id: null, enabled: true },
      { ...mockHost, uuid: 'h2', domain_names: 'b.example.com', ssl_forced: true, certificate_id: null, enabled: true },
      { ...mockHost, uuid: 'h3', domain_names: 'c.example.com', ssl_forced: true, certificate_id: null, enabled: true },
      { ...mockHost, uuid: 'h4', domain_names: 'd.example.com', ssl_forced: true, certificate_id: null, enabled: true },
    ]
    renderWithRouter(<CertificateStatusCard certificates={certs} hosts={hosts} />)

    // 2 out of 4 hosts have matching certs = 50%
    expect(screen.getByText('50% provisioned')).toBeInTheDocument()
    expect(screen.getByText('2 hosts awaiting certificate')).toBeInTheDocument()
  })

  it('shows all pending when no certificates exist', () => {
    const certs: Certificate[] = []
    const hosts: ProxyHost[] = [
      { ...mockHost, uuid: 'h1', domain_names: 'a.example.com', ssl_forced: true, certificate_id: null, enabled: true },
      { ...mockHost, uuid: 'h2', domain_names: 'b.example.com', ssl_forced: true, certificate_id: null, enabled: true },
    ]
    renderWithRouter(<CertificateStatusCard certificates={certs} hosts={hosts} />)

    expect(screen.getByText('2 hosts awaiting certificate')).toBeInTheDocument()
    expect(screen.getByText('0% provisioned')).toBeInTheDocument()
  })

  it('shows 100% provisioned when all SSL hosts have matching certificates', () => {
    const certs: Certificate[] = [
      mockCertWithDomain('a.example.com'),
      mockCertWithDomain('b.example.com'),
    ]
    const hosts: ProxyHost[] = [
      { ...mockHost, uuid: 'h1', domain_names: 'a.example.com', ssl_forced: true, certificate_id: null, enabled: true },
      { ...mockHost, uuid: 'h2', domain_names: 'b.example.com', ssl_forced: true, certificate_id: null, enabled: true },
    ]
    renderWithRouter(<CertificateStatusCard certificates={certs} hosts={hosts} />)

    // Should NOT show awaiting indicator when all hosts are covered
    expect(screen.queryByText(/awaiting certificate/)).not.toBeInTheDocument()
    expect(screen.queryByText(/provisioned/)).not.toBeInTheDocument()
  })

  it('handles whitespace in domain names', () => {
    const certs: Certificate[] = [mockCertWithDomain('example.com')]
    const hosts: ProxyHost[] = [
      { ...mockHost, domain_names: '  example.com  ', ssl_forced: true, certificate_id: null, enabled: true }
    ]
    renderWithRouter(<CertificateStatusCard certificates={certs} hosts={hosts} />)

    expect(screen.queryByText(/awaiting certificate/)).not.toBeInTheDocument()
  })

  it('handles whitespace in certificate domains', () => {
    const certs: Certificate[] = [{
      ...mockCertWithDomain('example.com'),
      domains: '  example.com  '
    }]
    const hosts: ProxyHost[] = [
      { ...mockHost, domain_names: 'example.com', ssl_forced: true, certificate_id: null, enabled: true }
    ]
    renderWithRouter(<CertificateStatusCard certificates={certs} hosts={hosts} />)

    expect(screen.queryByText(/awaiting certificate/)).not.toBeInTheDocument()
  })

  it('correctly counts mix of covered and uncovered hosts', () => {
    const certs: Certificate[] = [mockCertWithDomain('covered.com')]
    const hosts: ProxyHost[] = [
      { ...mockHost, uuid: 'h1', domain_names: 'covered.com', ssl_forced: true, certificate_id: null, enabled: true },
      { ...mockHost, uuid: 'h2', domain_names: 'uncovered.com', ssl_forced: true, certificate_id: null, enabled: true },
      { ...mockHost, uuid: 'h3', domain_names: 'disabled.com', ssl_forced: true, certificate_id: null, enabled: false },
      { ...mockHost, uuid: 'h4', domain_names: 'no-ssl.com', ssl_forced: false, certificate_id: null, enabled: true },
    ]
    renderWithRouter(<CertificateStatusCard certificates={certs} hosts={hosts} />)

    // Only h1 and h2 are SSL hosts that are enabled
    // h1 is covered, h2 is not
    expect(screen.getByText('1 host awaiting certificate')).toBeInTheDocument()
    expect(screen.getByText('50% provisioned')).toBeInTheDocument()
  })
})
