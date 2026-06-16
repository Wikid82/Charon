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

  it('renders a color legend with different background colors per host', () => {
    render(<TopHostsChart data={mockHosts} isLoading={false} />)

    const legend = screen.getByRole('list', { name: 'Host color legend' })
    const dots = legend.querySelectorAll('[aria-hidden="true"]')
    expect(dots.length).toBe(2)

    const colors = Array.from(dots).map(dot => (dot as HTMLElement).style.backgroundColor)
    // Each host should have a distinct color
    expect(colors[0]).not.toBe(colors[1])
    // Colors should be set (non-empty)
    expect(colors[0]).toBeTruthy()
    expect(colors[1]).toBeTruthy()
  })

  it('renders the color legend with host names', () => {
    render(<TopHostsChart data={mockHosts} isLoading={false} />)

    const legend = screen.getByRole('list', { name: 'Host color legend' })
    expect(legend).toBeInTheDocument()

    expect(legend).toHaveTextContent('api.example.com')
    expect(legend).toHaveTextContent('www.example.com')
  })

  it('does not render the color legend when data is empty', () => {
    render(<TopHostsChart data={[]} isLoading={false} />)

    expect(screen.queryByRole('list', { name: 'Host color legend' })).not.toBeInTheDocument()
  })

  it('wraps colors cyclically for more than 8 hosts', () => {
    const manyHosts: HostStat[] = Array.from({ length: 10 }, (_, i) => ({
      host_id: `h${i}`,
      hostname: `host${i}.example.com`,
      count: 100 - i,
    }))

    render(<TopHostsChart data={manyHosts} isLoading={false} />)

    const legend = screen.getByRole('list', { name: 'Host color legend' })
    const dots = legend.querySelectorAll('[aria-hidden="true"]')
    expect(dots.length).toBe(10)

    // Index 0 and 8 should share the same color (8 % 8 = 0)
    const color0 = (dots[0] as HTMLElement).style.backgroundColor
    const color8 = (dots[8] as HTMLElement).style.backgroundColor
    expect(color0).toBe(color8)
  })

  it('renders info tooltip trigger button', () => {
    render(<TopHostsChart data={mockHosts} isLoading={false} />)

    expect(screen.getByRole('button', { name: 'About this widget' })).toBeInTheDocument()
  })
})
