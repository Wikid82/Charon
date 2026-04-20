import { render, screen } from '@testing-library/react'
import { describe, it, expect, vi } from 'vitest'

import type { ChainEntry } from '../../api/certificates'
import CertificateChainViewer from '../CertificateChainViewer'

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string) => key,
    i18n: { language: 'en', changeLanguage: vi.fn() },
  }),
}))

function makeChain(count: number): ChainEntry[] {
  return Array.from({ length: count }, (_, i) => ({
    subject: `Subject ${i}`,
    issuer: `Issuer ${i}`,
    expires_at: '2026-06-01T00:00:00Z',
  }))
}

describe('CertificateChainViewer', () => {
  it('renders empty state when chain is empty', () => {
    render(<CertificateChainViewer chain={[]} />)
    expect(screen.getByText('certificates.noChainData')).toBeTruthy()
  })

  it('renders single entry as leaf', () => {
    render(<CertificateChainViewer chain={makeChain(1)} />)
    expect(screen.getByText('certificates.chainLeaf')).toBeTruthy()
    expect(screen.getByText('Subject 0')).toBeTruthy()
  })

  it('renders two entries as leaf + root', () => {
    render(<CertificateChainViewer chain={makeChain(2)} />)
    expect(screen.getByText('certificates.chainLeaf')).toBeTruthy()
    expect(screen.getByText('certificates.chainRoot')).toBeTruthy()
  })

  it('renders three entries as leaf + intermediate + root', () => {
    render(<CertificateChainViewer chain={makeChain(3)} />)
    expect(screen.getByText('certificates.chainLeaf')).toBeTruthy()
    expect(screen.getByText('certificates.chainIntermediate')).toBeTruthy()
    expect(screen.getByText('certificates.chainRoot')).toBeTruthy()
  })

  it('displays issuer for each entry', () => {
    render(<CertificateChainViewer chain={makeChain(2)} />)
    expect(screen.getByText(/Issuer 0/)).toBeTruthy()
    expect(screen.getByText(/Issuer 1/)).toBeTruthy()
  })

  it('displays formatted expiration dates', () => {
    render(<CertificateChainViewer chain={makeChain(1)} />)
    const dateStr = new Date('2026-06-01T00:00:00Z').toLocaleDateString()
    expect(screen.getByText(new RegExp(dateStr))).toBeTruthy()
  })

  it('uses list role with list items', () => {
    render(<CertificateChainViewer chain={makeChain(2)} />)
    expect(screen.getByRole('list')).toBeTruthy()
    expect(screen.getAllByRole('listitem')).toHaveLength(2)
  })

  it('has aria-label on list', () => {
    render(<CertificateChainViewer chain={makeChain(1)} />)
    expect(screen.getByRole('list').getAttribute('aria-label')).toBe(
      'certificates.certificateChain',
    )
  })
})
