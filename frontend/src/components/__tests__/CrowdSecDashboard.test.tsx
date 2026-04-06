import { screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, it, expect, vi, beforeEach } from 'vitest'

import { renderWithQueryClient } from '../../test-utils/renderWithQueryClient'
import { CrowdSecDashboard } from '../crowdsec/CrowdSecDashboard'

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (_key: string, fallback: string) => fallback ?? _key,
  }),
}))

const mockSummary = {
  data: {
    total_decisions: 100,
    active_decisions: 10,
    unique_ips: 50,
    top_scenario: 'crowdsecurity/http-bad-user-agent',
    decisions_trend: 0,
    range: '24h',
    cached: false,
    generated_at: '2025-01-01T00:00:00Z',
  },
  isLoading: false,
  isError: false,
}

const mockTimeline = {
  data: { buckets: [], range: '24h', interval: '1h', cached: false },
  isLoading: false,
  isError: false,
}

const mockTopIPs = {
  data: { ips: [], range: '24h', cached: false },
  isLoading: false,
  isError: false,
}

const mockScenarios = {
  data: { scenarios: [], total: 0, range: '24h', cached: false },
  isLoading: false,
  isError: false,
}

vi.mock('../../hooks/useCrowdsecDashboard', () => ({
  useDashboardSummary: () => mockSummary,
  useDashboardTimeline: () => mockTimeline,
  useDashboardTopIPs: () => mockTopIPs,
  useDashboardScenarios: () => mockScenarios,
  useAlerts: () => ({ data: undefined, isLoading: false, isError: false }),
}))

vi.mock('recharts', async () => {
  const Original = await vi.importActual('recharts')
  return {
    ...Original,
    ResponsiveContainer: ({ children }: { children: React.ReactNode }) => (
      <div data-testid="responsive-container">{children}</div>
    ),
  }
})

describe('CrowdSecDashboard', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('renders the time range selector', () => {
    renderWithQueryClient(<CrowdSecDashboard />)

    expect(screen.getByRole('radiogroup')).toBeInTheDocument()
    expect(screen.getByRole('radio', { name: '24H' })).toHaveAttribute('aria-checked', 'true')
  })

  it('renders the refresh button', () => {
    renderWithQueryClient(<CrowdSecDashboard />)

    expect(screen.getByRole('button', { name: /Refresh/i })).toBeInTheDocument()
  })

  it('renders summary cards section', () => {
    renderWithQueryClient(<CrowdSecDashboard />)

    expect(screen.getByText('100')).toBeInTheDocument()
    expect(screen.getByText('10')).toBeInTheDocument()
  })

  it('switches time range when selector is clicked', async () => {
    const user = userEvent.setup()
    renderWithQueryClient(<CrowdSecDashboard />)

    await user.click(screen.getByRole('radio', { name: '1H' }))

    expect(screen.getByRole('radio', { name: '1H' })).toHaveAttribute('aria-checked', 'true')
    expect(screen.getByRole('radio', { name: '24H' })).toHaveAttribute('aria-checked', 'false')
  })
})
