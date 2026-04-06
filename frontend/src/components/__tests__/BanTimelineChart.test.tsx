import { render, screen } from '@testing-library/react'
import { describe, it, expect, vi } from 'vitest'

import { BanTimelineChart } from '../crowdsec/BanTimelineChart'

import type { TimelineBucket } from '../../api/crowdsecDashboard'

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

const mockBuckets: TimelineBucket[] = [
  { timestamp: '2025-01-01T00:00:00Z', bans: 10, captchas: 3 },
  { timestamp: '2025-01-01T01:00:00Z', bans: 15, captchas: 5 },
  { timestamp: '2025-01-01T02:00:00Z', bans: 8, captchas: 2 },
]

describe('BanTimelineChart', () => {
  it('renders loading skeleton', () => {
    render(
      <BanTimelineChart data={undefined} isLoading isError={false} range="24h" />,
    )

    expect(screen.queryByText('Decision Timeline')).not.toBeInTheDocument()
    expect(screen.queryByText('Failed to load timeline data.')).not.toBeInTheDocument()
  })

  it('renders error state', () => {
    render(<BanTimelineChart data={undefined} isLoading={false} isError range="24h" />)

    expect(screen.getByText('Failed to load timeline data.')).toBeInTheDocument()
  })

  it('renders empty state', () => {
    render(<BanTimelineChart data={[]} isLoading={false} isError={false} range="24h" />)

    expect(screen.getByText('No decision data for the selected period.')).toBeInTheDocument()
  })

  it('renders chart with data', () => {
    render(<BanTimelineChart data={mockBuckets} isLoading={false} isError={false} range="24h" />)

    expect(screen.getByText('Decision Timeline')).toBeInTheDocument()
    expect(screen.getByRole('img')).toHaveAttribute(
      'aria-label',
      'Area chart showing bans and captchas over time',
    )
  })
})
