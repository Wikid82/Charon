import { render, screen } from '@testing-library/react'
import { describe, it, expect, vi } from 'vitest'

import { ScenarioBreakdownChart } from '../crowdsec/ScenarioBreakdownChart'

import type { ScenarioEntry } from '../../api/crowdsecDashboard'

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (_key: string, fallback: string) => fallback ?? _key,
  }),
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

const mockScenarios: ScenarioEntry[] = [
  { name: 'crowdsecurity/http-bad-user-agent', count: 100, percentage: 50 },
  { name: 'crowdsecurity/ssh-bf', count: 60, percentage: 30 },
  { name: 'crowdsecurity/http-probing', count: 40, percentage: 20 },
]

describe('ScenarioBreakdownChart', () => {
  it('renders loading skeleton', () => {
    render(
      <ScenarioBreakdownChart data={undefined} isLoading isError={false} />,
    )

    expect(screen.queryByText('Scenario Breakdown')).not.toBeInTheDocument()
    expect(screen.queryByText('Failed to load scenario data.')).not.toBeInTheDocument()
  })

  it('renders error state', () => {
    render(<ScenarioBreakdownChart data={undefined} isLoading={false} isError />)

    expect(screen.getByText('Failed to load scenario data.')).toBeInTheDocument()
  })

  it('renders empty state', () => {
    render(<ScenarioBreakdownChart data={[]} isLoading={false} isError={false} />)

    expect(screen.getByText('No scenario data for the selected period.')).toBeInTheDocument()
  })

  it('renders chart and legend with data', () => {
    render(<ScenarioBreakdownChart data={mockScenarios} isLoading={false} isError={false} />)

    expect(screen.getByText('Scenario Breakdown')).toBeInTheDocument()
    expect(screen.getByRole('img')).toHaveAttribute(
      'aria-label',
      'Donut chart showing distribution of scenarios by decision count',
    )
    expect(screen.getByText('http-bad-user-agent')).toBeInTheDocument()
    expect(screen.getByText('ssh-bf')).toBeInTheDocument()
    expect(screen.getByText('http-probing')).toBeInTheDocument()
  })

  it('renders legend with counts and percentages', () => {
    render(<ScenarioBreakdownChart data={mockScenarios} isLoading={false} isError={false} />)

    expect(screen.getByText('100')).toBeInTheDocument()
    expect(screen.getByText('50.0%')).toBeInTheDocument()
    expect(screen.getByText('60')).toBeInTheDocument()
    expect(screen.getByText('30.0%')).toBeInTheDocument()
  })
})
