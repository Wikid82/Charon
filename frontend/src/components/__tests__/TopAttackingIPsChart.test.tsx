import { render, screen } from '@testing-library/react'
import { describe, it, expect, vi } from 'vitest'

import { TopAttackingIPsChart } from '../crowdsec/TopAttackingIPsChart'

import type { TopIP } from '../../api/crowdsecDashboard'

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

const mockIPs: TopIP[] = [
  { ip: '192.168.1.1', count: 50, last_seen: '2025-01-01T00:00:00Z', country: 'US' },
  { ip: '10.0.0.1', count: 30, last_seen: '2025-01-01T01:00:00Z', country: 'CN' },
]

describe('TopAttackingIPsChart', () => {
  it('renders loading skeleton', () => {
    render(
      <TopAttackingIPsChart data={undefined} isLoading isError={false} />,
    )

    expect(screen.queryByText('Top Attacking IPs')).not.toBeInTheDocument()
    expect(screen.queryByText('Failed to load top IPs data.')).not.toBeInTheDocument()
  })

  it('renders error state', () => {
    render(<TopAttackingIPsChart data={undefined} isLoading={false} isError />)

    expect(screen.getByText('Failed to load top IPs data.')).toBeInTheDocument()
  })

  it('renders empty state', () => {
    render(<TopAttackingIPsChart data={[]} isLoading={false} isError={false} />)

    expect(screen.getByText('No attacking IPs in the selected period.')).toBeInTheDocument()
  })

  it('renders chart with data and accessible label', () => {
    render(<TopAttackingIPsChart data={mockIPs} isLoading={false} isError={false} />)

    expect(screen.getByText('Top Attacking IPs')).toBeInTheDocument()
    expect(screen.getByRole('img')).toHaveAttribute(
      'aria-label',
      'Horizontal bar chart showing top attacking IPs by decision count',
    )
  })
})
