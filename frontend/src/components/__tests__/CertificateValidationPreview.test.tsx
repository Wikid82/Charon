import { render, screen } from '@testing-library/react'
import { describe, it, expect, vi } from 'vitest'

import type { ValidationResult } from '../../api/certificates'
import CertificateValidationPreview from '../CertificateValidationPreview'

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string) => key,
    i18n: { language: 'en', changeLanguage: vi.fn() },
  }),
}))

function makeResult(overrides: Partial<ValidationResult> = {}): ValidationResult {
  return {
    valid: true,
    common_name: 'example.com',
    domains: ['example.com', 'www.example.com'],
    issuer_org: 'Test CA',
    expires_at: '2026-06-01T00:00:00Z',
    key_match: true,
    chain_valid: true,
    chain_depth: 2,
    warnings: [],
    errors: [],
    ...overrides,
  }
}

describe('CertificateValidationPreview', () => {
  it('renders valid certificate state', () => {
    render(<CertificateValidationPreview result={makeResult()} />)
    expect(screen.getByText('certificates.validCertificate')).toBeTruthy()
    expect(screen.getByTestId('certificate-validation-preview')).toBeTruthy()
  })

  it('renders invalid certificate state', () => {
    render(
      <CertificateValidationPreview result={makeResult({ valid: false })} />,
    )
    expect(screen.getByText('certificates.invalidCertificate')).toBeTruthy()
  })

  it('displays common name', () => {
    render(<CertificateValidationPreview result={makeResult()} />)
    expect(screen.getByText('example.com')).toBeTruthy()
  })

  it('displays domains joined by comma', () => {
    render(<CertificateValidationPreview result={makeResult()} />)
    expect(screen.getByText('example.com, www.example.com')).toBeTruthy()
  })

  it('displays dash when no domains provided', () => {
    render(
      <CertificateValidationPreview result={makeResult({ domains: [] })} />,
    )
    const dashes = screen.getAllByText('-')
    expect(dashes.length).toBeGreaterThan(0)
  })

  it('displays issuer organization', () => {
    render(<CertificateValidationPreview result={makeResult()} />)
    expect(screen.getByText('Test CA')).toBeTruthy()
  })

  it('displays formatted expiration date', () => {
    render(<CertificateValidationPreview result={makeResult()} />)
    const dateStr = new Date('2026-06-01T00:00:00Z').toLocaleDateString()
    expect(screen.getByText(dateStr)).toBeTruthy()
  })

  it('shows Yes for key match', () => {
    render(<CertificateValidationPreview result={makeResult({ key_match: true, chain_valid: false })} />)
    expect(screen.getByText('Yes')).toBeTruthy()
  })

  it('shows No key provided when no key match', () => {
    render(
      <CertificateValidationPreview result={makeResult({ key_match: false })} />,
    )
    expect(screen.getByText('No key provided')).toBeTruthy()
  })

  it('shows chain depth when > 0', () => {
    render(
      <CertificateValidationPreview result={makeResult({ chain_depth: 3 })} />,
    )
    expect(screen.getByText('3')).toBeTruthy()
  })

  it('does not show chain depth when 0', () => {
    render(
      <CertificateValidationPreview result={makeResult({ chain_depth: 0 })} />,
    )
    expect(screen.queryByText('certificates.chainDepth')).toBeFalsy()
  })

  it('renders warnings when present', () => {
    render(
      <CertificateValidationPreview
        result={makeResult({ warnings: ['Expiring soon', 'Weak key'] })}
      />,
    )
    expect(screen.getByText('certificates.warnings')).toBeTruthy()
    expect(screen.getByText('Expiring soon')).toBeTruthy()
    expect(screen.getByText('Weak key')).toBeTruthy()
  })

  it('does not render warnings section when empty', () => {
    render(<CertificateValidationPreview result={makeResult({ warnings: [] })} />)
    expect(screen.queryByText('certificates.warnings')).toBeFalsy()
  })

  it('renders errors when present', () => {
    render(
      <CertificateValidationPreview
        result={makeResult({ errors: ['Certificate revoked'] })}
      />,
    )
    expect(screen.getByText('certificates.errors')).toBeTruthy()
    expect(screen.getByText('Certificate revoked')).toBeTruthy()
  })

  it('does not render errors section when empty', () => {
    render(<CertificateValidationPreview result={makeResult({ errors: [] })} />)
    expect(screen.queryByText('certificates.errors')).toBeFalsy()
  })

  it('has correct region role and aria-label', () => {
    render(<CertificateValidationPreview result={makeResult()} />)
    const region = screen.getByRole('region')
    expect(region.getAttribute('aria-label')).toBe('certificates.validationPreview')
  })
})
