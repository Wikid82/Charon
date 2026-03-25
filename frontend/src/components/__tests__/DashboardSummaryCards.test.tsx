import { render, screen } from '@testing-library/react'
import { describe, it, expect, vi } from 'vitest'

import { DashboardSummaryCards } from '../crowdsec/DashboardSummaryCards'

import type { DashboardSummary } from '../../api/crowdsecDashboard'

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (_key: string, fallback: string) => fallback ?? _key,
  }),
}))

const mockSummary: DashboardSummary = {
  total_decisions: 1234,
  active_decisions: 42,
  unique_ips: 89,
  top_scenario: 'crowdsecurity/http-bad-user-agent',
  decisions_trend: 12.5,
  range: '24h',
  cached: false,
  generated_at: '2025-01-01T00:00:00Z',
}

describe('DashboardSummaryCards', () => {
  it('renders loading skeletons', () => {
    render(<DashboardSummaryCards data={undefined} isLoading isError={false} />)

    expect(screen.getByTestId('dashboard-summary-cards')).toBeInTheDocument()
    expect(screen.queryByText('Total Decisions')).not.toBeInTheDocument()
  })

  it('renders error state', () => {
    render(<DashboardSummaryCards data={undefined} isLoading={false} isError />)

    expect(screen.getByText('Failed to load summary data.')).toBeInTheDocument()
  })

  it('renders summary data with correct values', () => {
    render(<DashboardSummaryCards data={mockSummary} isLoading={false} isError={false} />)

    expect(screen.getByText('1,234')).toBeInTheDocument()
    expect(screen.getByText('42')).toBeInTheDocument()
    expect(screen.getByText('89')).toBeInTheDocument()
    expect(screen.getByText('http-bad-user-agent')).toBeInTheDocument()
  })

  it('displays positive trend with + prefix', () => {
    render(<DashboardSummaryCards data={mockSummary} isLoading={false} isError={false} />)

    expect(screen.getByText('+12.5%')).toBeInTheDocument()
  })

  it('displays negative trend', () => {
    const data = { ...mockSummary, decisions_trend: -5.2 }
    render(<DashboardSummaryCards data={data} isLoading={false} isError={false} />)

    expect(screen.getByText('-5.2%')).toBeInTheDocument()
  })

  it('shows N/A when active_decisions is -1', () => {
    const data = { ...mockSummary, active_decisions: -1 }
    render(<DashboardSummaryCards data={data} isLoading={false} isError={false} />)

    expect(screen.getByText('N/A')).toBeInTheDocument()
    expect(screen.getByText('LAPI unavailable')).toBeInTheDocument()
  })

  it('has aria-live attribute for screen reader updates', () => {
    render(<DashboardSummaryCards data={mockSummary} isLoading={false} isError={false} />)

    expect(screen.getByTestId('dashboard-summary-cards')).toHaveAttribute('aria-live', 'polite')
  })
})
