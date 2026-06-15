import { render, screen } from '@testing-library/react'
import { describe, it, expect } from 'vitest'

import { CertExpiryList } from '../CertExpiryList'

import type { CertExpiry } from '../../../api/stats'

const mockCerts: CertExpiry[] = [
  {
    host_id: 'h1',
    hostname: 'critical.example.com',
    expires_at: '2024-01-10T00:00:00Z',
    days_left: 3,
  },
  {
    host_id: 'h2',
    hostname: 'warning.example.com',
    expires_at: '2024-01-25T00:00:00Z',
    days_left: 20,
  },
  {
    host_id: 'h3',
    hostname: 'healthy.example.com',
    expires_at: '2024-03-01T00:00:00Z',
    days_left: 60,
  },
]

describe('CertExpiryList', () => {
  it('renders loading skeletons when isLoading is true', () => {
    render(<CertExpiryList data={undefined} isLoading />)

    expect(screen.queryByText('critical.example.com')).not.toBeInTheDocument()
    expect(screen.getByText('Certificate Expiry')).toBeInTheDocument()
  })

  it('renders empty state when data is empty', () => {
    render(<CertExpiryList data={[]} isLoading={false} />)

    expect(screen.getByText('No certificates expiring soon')).toBeInTheDocument()
  })

  it('renders hostnames when data is present', () => {
    render(<CertExpiryList data={mockCerts} isLoading={false} />)

    expect(screen.getByText('critical.example.com')).toBeInTheDocument()
    expect(screen.getByText('warning.example.com')).toBeInTheDocument()
    expect(screen.getByText('healthy.example.com')).toBeInTheDocument()
  })

  it('shows days left for each cert', () => {
    render(<CertExpiryList data={mockCerts} isLoading={false} />)

    expect(screen.getByText('3d')).toBeInTheDocument()
    expect(screen.getByText('20d')).toBeInTheDocument()
    expect(screen.getByText('60d')).toBeInTheDocument()
  })

  it('applies red class for certs expiring in ≤7 days', () => {
    render(<CertExpiryList data={mockCerts} isLoading={false} />)

    const redBadge = screen.getByText('3d')
    expect(redBadge).toHaveClass('text-error')
  })

  it('applies amber class for certs expiring in ≤30 days but >7 days', () => {
    render(<CertExpiryList data={mockCerts} isLoading={false} />)

    const amberBadge = screen.getByText('20d')
    expect(amberBadge).toHaveClass('text-warning')
  })

  it('applies green class for certs expiring in >30 days', () => {
    render(<CertExpiryList data={mockCerts} isLoading={false} />)

    const greenBadge = screen.getByText('60d')
    expect(greenBadge).toHaveClass('text-success')
  })

  it('renders exactly at the 7-day boundary as red', () => {
    const certs: CertExpiry[] = [
      { host_id: 'h1', hostname: 'edge.example.com', expires_at: '2024-01-08T00:00:00Z', days_left: 7 },
    ]
    render(<CertExpiryList data={certs} isLoading={false} />)

    expect(screen.getByText('7d')).toHaveClass('text-error')
  })

  it('renders exactly at the 30-day boundary as amber', () => {
    const certs: CertExpiry[] = [
      { host_id: 'h1', hostname: 'edge.example.com', expires_at: '2024-02-10T00:00:00Z', days_left: 30 },
    ]
    render(<CertExpiryList data={certs} isLoading={false} />)

    expect(screen.getByText('30d')).toHaveClass('text-warning')
  })

  it('renders table structure with role attributes', () => {
    render(<CertExpiryList data={mockCerts} isLoading={false} />)

    expect(screen.getByRole('table')).toBeInTheDocument()
    expect(screen.getAllByRole('row').length).toBeGreaterThan(1)
  })

  it('renders card title', () => {
    render(<CertExpiryList data={mockCerts} isLoading={false} />)

    expect(screen.getByText('Certificate Expiry')).toBeInTheDocument()
  })
})
