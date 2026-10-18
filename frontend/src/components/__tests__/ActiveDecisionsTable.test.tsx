import { render, screen, within } from '@testing-library/react'
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

  it('shows dash for empty country when another row has one', () => {
    const data: TopIP[] = [
      { ip: '1.2.3.4', count: 1, last_seen: new Date().toISOString(), country: '' },
      { ip: '5.6.7.8', count: 2, last_seen: new Date().toISOString(), country: 'FR' },
    ]
    render(<ActiveDecisionsTable data={data} isLoading={false} isError={false} />)

    expect(screen.getByRole('button', { name: /Sort by Country/i })).toBeInTheDocument()
    expect(screen.getByText('FR')).toBeInTheDocument()
    expect(screen.getByText('—')).toBeInTheDocument()
  })

  it('hides the country column when every row has an empty country', () => {
    const data: TopIP[] = [
      { ip: '1.2.3.4', count: 1, last_seen: new Date().toISOString(), country: '' },
      { ip: '5.6.7.8', count: 2, last_seen: new Date().toISOString(), country: '' },
    ]
    render(<ActiveDecisionsTable data={data} isLoading={false} isError={false} />)

    expect(screen.queryByRole('button', { name: /Sort by Country/i })).not.toBeInTheDocument()
    expect(screen.getAllByRole('columnheader')).toHaveLength(3)
    expect(screen.queryByText('—')).not.toBeInTheDocument()
    expect(screen.getAllByRole('cell')).toHaveLength(6)
  })

  it('renders the last seen time', () => {
    const data: TopIP[] = [
      { ip: '1.2.3.4', count: 1, last_seen: new Date(Date.now() - 3 * 3600_000).toISOString(), country: '' },
    ]
    render(<ActiveDecisionsTable data={data} isLoading={false} isError={false} />)

    expect(screen.getByText('3h ago')).toBeInTheDocument()
  })

  it.each(['', 'not-a-date'])('shows a dash and no tooltip for last_seen %j', (lastSeen) => {
    render(
      <ActiveDecisionsTable
        data={[{ ip: '1.2.3.4', count: 3, last_seen: lastSeen, country: 'US' }]}
        isLoading={false}
        isError={false}
      />,
    )

    const time = document.querySelector('time')
    expect(time).toHaveTextContent('—')
    expect(time).not.toHaveAttribute('title')
    expect(screen.queryByText(/NaN|Invalid Date/)).not.toBeInTheDocument()
  })

  it('shows relative time and a tooltip for a valid last_seen', () => {
    render(<ActiveDecisionsTable data={mockTopIPs} isLoading={false} isError={false} />)

    const time = document.querySelector('time')
    expect(time).toHaveTextContent(/ago$/)
    expect(time).toHaveAttribute('title')
  })

  it('falls back to the default sort when the sorted Country column becomes hidden', async () => {
    const user = userEvent.setup()
    const now = new Date().toISOString()
    const withCountry: TopIP[] = [
      { ip: '1.1.1.1', count: 5, last_seen: now, country: 'AA' },
      { ip: '2.2.2.2', count: 50, last_seen: now, country: 'ZZ' },
      { ip: '3.3.3.3', count: 20, last_seen: now, country: 'MM' },
    ]
    const { rerender } = render(
      <ActiveDecisionsTable data={withCountry} isLoading={false} isError={false} />,
    )

    await user.click(screen.getByRole('button', { name: 'Sort by Country' }))
    expect(screen.getByRole('columnheader', { name: /Country/ })).toHaveAttribute('aria-sort', 'descending')
    const ipsOf = () => screen.getAllByRole('row').slice(1).map((r) => within(r).getAllByRole('cell')[0].textContent)
    expect(ipsOf()).toEqual(['2.2.2.2', '3.3.3.3', '1.1.1.1'])

    const noCountry = withCountry.map((r) => ({ ...r, country: '' }))
    rerender(<ActiveDecisionsTable data={noCountry} isLoading={false} isError={false} />)

    expect(screen.queryByRole('columnheader', { name: /Country/ })).not.toBeInTheDocument()
    expect(ipsOf()).toEqual(['2.2.2.2', '3.3.3.3', '1.1.1.1'])
    expect(screen.getByRole('columnheader', { name: /Alerts/ })).toHaveAttribute('aria-sort', 'descending')
    expect(screen.getByRole('columnheader', { name: /^IP/ })).toHaveAttribute('aria-sort', 'none')
  })
})
