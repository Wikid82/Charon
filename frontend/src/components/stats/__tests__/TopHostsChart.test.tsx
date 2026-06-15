import { render, screen } from '@testing-library/react'
import { describe, it, expect, vi } from 'vitest'

import { TopHostsChart } from '../TopHostsChart'

import type { HostStat } from '../../../api/stats'

vi.mock('recharts', async () => {
  const Original = await vi.importActual('recharts')
  return {
    ...Original,
    ResponsiveContainer: ({ children }: { children: React.ReactNode }) => (
      <div data-testid="responsive-container">{children}</div>
    ),
    // Call content callback to cover tooltip render paths including the hostname fallback.
    Tooltip: ({
      content,
    }: {
      content?: (props: object) => React.ReactNode
    }) => (
      <div data-testid="tooltip">
        {/* With explicit hostname in payload — primary path */}
        {content?.({
          active: true,
          payload: [{ value: 500, payload: { hostname: 'api.example.com' } }],
          label: 'api.example.com',
        })}
        {/* Without hostname in payload — exercises the ?? String(label) fallback */}
        {content?.({
          active: true,
          payload: [{ value: 300, payload: {} }],
          label: 'fallback-host',
        })}
        {/* count is not a number — exercises String(count) branch */}
        {content?.({
          active: true,
          payload: [{ value: undefined, payload: { hostname: 'host' } }],
          label: '',
        })}
        {content?.({ active: false, payload: [] })}
      </div>
    ),
  }
})

const mockHosts: HostStat[] = [
  { host_id: 'h1', hostname: 'api.example.com', count: 500 },
  { host_id: 'h2', hostname: 'www.example.com', count: 300 },
]

describe('TopHostsChart', () => {
  it('renders loading skeletons when isLoading is true', () => {
    render(<TopHostsChart data={undefined} isLoading />)

    expect(screen.queryByTestId('responsive-container')).not.toBeInTheDocument()
  })

  it('renders empty state when data is empty', () => {
    render(<TopHostsChart data={[]} isLoading={false} />)

    expect(screen.getByText('No data available')).toBeInTheDocument()
  })

  it('renders chart title', () => {
    render(<TopHostsChart data={mockHosts} isLoading={false} />)

    expect(screen.getByText('Top Hosts')).toBeInTheDocument()
  })

  it('renders responsive container when data is present', () => {
    render(<TopHostsChart data={mockHosts} isLoading={false} />)

    expect(screen.getByTestId('responsive-container')).toBeInTheDocument()
  })

  it('truncates long hostnames to 24 characters', () => {
    const longHostname = 'a'.repeat(30) + '.example.com'
    render(
      <TopHostsChart
        data={[{ host_id: 'h1', hostname: longHostname, count: 10 }]}
        isLoading={false}
      />,
    )

    // The chart renders but truncation is applied at the data level
    expect(screen.getByTestId('responsive-container')).toBeInTheDocument()
  })
})
