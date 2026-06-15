import { render, screen } from '@testing-library/react'
import { describe, it, expect } from 'vitest'

import { RequestCountWidget } from '../RequestCountWidget'

import type { StatsSummary } from '../../../api/stats'

const mockSummary: StatsSummary = {
  requests_last_24h: 1234,
  requests_last_7d: 8765,
  requests_last_30d: 42000,
}

describe('RequestCountWidget', () => {
  it('renders loading skeletons when isLoading is true', () => {
    render(<RequestCountWidget summary={undefined} isLoading />)

    expect(screen.getByText('Last 24h')).toBeInTheDocument()
    expect(screen.getByText('Last 7 days')).toBeInTheDocument()
    expect(screen.getByText('Last 30 days')).toBeInTheDocument()
    // Values should not be present (skeletons instead)
    expect(screen.queryByText('1,234')).not.toBeInTheDocument()
  })

  it('renders stat values when data is provided', () => {
    render(<RequestCountWidget summary={mockSummary} isLoading={false} />)

    expect(screen.getByText('1,234')).toBeInTheDocument()
    expect(screen.getByText('8,765')).toBeInTheDocument()
    expect(screen.getByText('42,000')).toBeInTheDocument()
  })

  it('renders em-dash when summary is undefined and not loading', () => {
    render(<RequestCountWidget summary={undefined} isLoading={false} />)

    const dashes = screen.getAllByText('—')
    expect(dashes).toHaveLength(3)
  })

  it('renders card title', () => {
    render(<RequestCountWidget summary={mockSummary} isLoading={false} />)

    expect(screen.getByText('Request Counts')).toBeInTheDocument()
  })

  it('renders all three period labels', () => {
    render(<RequestCountWidget summary={mockSummary} isLoading={false} />)

    expect(screen.getByText('Last 24h')).toBeInTheDocument()
    expect(screen.getByText('Last 7 days')).toBeInTheDocument()
    expect(screen.getByText('Last 30 days')).toBeInTheDocument()
  })
})
