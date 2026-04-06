import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, it, expect, vi } from 'vitest'

import { ActiveDecisionsTable } from '../crowdsec/ActiveDecisionsTable'

import type { TopIP } from '../../api/crowdsecDashboard'

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (_key: string, fallback: string) => fallback ?? _key,
  }),
}))

const mockTopIPs: TopIP[] = [
  {
    ip: '192.168.1.1',
    count: 25,
    last_seen: new Date(Date.now() - 3600_000).toISOString(),
    country: 'US',
  },
  {
    ip: '10.0.0.1',
    count: 12,
    last_seen: new Date(Date.now() - 7200_000).toISOString(),
    country: 'DE',
  },
]

describe('ActiveDecisionsTable', () => {
  it('renders loading skeleton', () => {
    render(
      <ActiveDecisionsTable data={undefined} isLoading isError={false} />,
    )

    expect(screen.queryByText('Active Decisions')).not.toBeInTheDocument()
    expect(screen.queryByText('Failed to load decisions.')).not.toBeInTheDocument()
  })

  it('renders error state', () => {
    render(<ActiveDecisionsTable data={undefined} isLoading={false} isError />)

    expect(screen.getByText('Failed to load decisions.')).toBeInTheDocument()
  })

  it('renders empty state', () => {
    render(<ActiveDecisionsTable data={[]} isLoading={false} isError={false} />)

    expect(screen.getByText('No active decisions.')).toBeInTheDocument()
  })

  it('renders table with top IP data', () => {
    render(<ActiveDecisionsTable data={mockTopIPs} isLoading={false} isError={false} />)

    expect(screen.getByText('Active Decisions')).toBeInTheDocument()
    expect(screen.getByText('192.168.1.1')).toBeInTheDocument()
    expect(screen.getByText('10.0.0.1')).toBeInTheDocument()
    expect(screen.getByText('25')).toBeInTheDocument()
    expect(screen.getByText('12')).toBeInTheDocument()
    expect(screen.getByText('US')).toBeInTheDocument()
    expect(screen.getByText('DE')).toBeInTheDocument()
  })

  it('renders column headers with sort buttons', () => {
    render(<ActiveDecisionsTable data={mockTopIPs} isLoading={false} isError={false} />)

    expect(screen.getByRole('button', { name: /Sort by IP/i })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /Sort by Alerts/i })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /Sort by Last Seen/i })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /Sort by Country/i })).toBeInTheDocument()
  })

  it('toggles sort direction when clicking the same column', async () => {
    const user = userEvent.setup()
    render(<ActiveDecisionsTable data={mockTopIPs} isLoading={false} isError={false} />)

    const ipSortButton = screen.getByRole('button', { name: /Sort by IP/i })
    await user.click(ipSortButton)

    const ipHeader = screen.getByRole('columnheader', { name: /IP/i })
    expect(ipHeader).toHaveAttribute('aria-sort', 'descending')

    await user.click(ipSortButton)
    expect(ipHeader).toHaveAttribute('aria-sort', 'ascending')
  })

  it('shows dash for missing country', () => {
    const data: TopIP[] = [{ ip: '1.2.3.4', count: 1, last_seen: new Date().toISOString(), country: '' }]
    render(<ActiveDecisionsTable data={data} isLoading={false} isError={false} />)

    expect(screen.getByText('—')).toBeInTheDocument()
  })
})
