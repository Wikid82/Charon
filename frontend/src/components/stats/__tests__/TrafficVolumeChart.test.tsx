import { render, screen } from '@testing-library/react'
import { describe, it, expect, vi } from 'vitest'

import { TrafficVolumeChart } from '../TrafficVolumeChart'

import type { TrafficBucket } from '../../../api/stats'

vi.mock('recharts', async () => {
  const Original = await vi.importActual('recharts')
  return {
    ...Original,
    ResponsiveContainer: ({ children }: { children: React.ReactNode }) => (
      <div data-testid="responsive-container">{children}</div>
    ),
    // Call tickFormatter and content callbacks to cover formatBytes and tooltip paths.
    YAxis: ({ tickFormatter }: { tickFormatter?: (value: number) => string }) => (
      <g data-testid="y-axis">
        {/* Exercise all three formatBytes branches: MB, KB, B */}
        <text>{tickFormatter?.(2_097_152)}</text>
        <text>{tickFormatter?.(2_048)}</text>
        <text>{tickFormatter?.(500)}</text>
      </g>
    ),
    Tooltip: ({
      content,
    }: {
      content?: (props: object) => React.ReactNode
    }) => (
      <div data-testid="tooltip">
        {content?.({
          active: true,
          payload: [{ value: 1_048_576, payload: {} }],
          label: '10:00',
        })}
        {content?.({ active: true, payload: [{ value: 'not-a-number', payload: {} }], label: '' })}
        {content?.({ active: false, payload: [] })}
      </div>
    ),
  }
})

const mockBuckets: TrafficBucket[] = [
  { bucket: '2024-01-15T10:00:00Z', bytes_sent: 1_048_576 },
  { bucket: '2024-01-15T11:00:00Z', bytes_sent: 2_097_152 },
]

describe('TrafficVolumeChart', () => {
  it('renders loading skeleton when isLoading is true', () => {
    render(<TrafficVolumeChart data={undefined} isLoading bucket="1h" />)

    expect(screen.queryByTestId('responsive-container')).not.toBeInTheDocument()
  })

  it('renders empty state when data is empty', () => {
    render(<TrafficVolumeChart data={[]} isLoading={false} bucket="1h" />)

    expect(screen.getByText('No data available')).toBeInTheDocument()
  })

  it('renders chart title', () => {
    render(<TrafficVolumeChart data={mockBuckets} isLoading={false} bucket="1h" />)

    expect(screen.getByText('Traffic Volume')).toBeInTheDocument()
  })

  it('renders responsive container when data is present', () => {
    render(<TrafficVolumeChart data={mockBuckets} isLoading={false} bucket="1h" />)

    expect(screen.getByTestId('responsive-container')).toBeInTheDocument()
  })

  it('renders with 6h bucket granularity', () => {
    render(<TrafficVolumeChart data={mockBuckets} isLoading={false} bucket="6h" />)

    expect(screen.getByTestId('responsive-container')).toBeInTheDocument()
  })

  it('renders with 1d bucket granularity', () => {
    render(<TrafficVolumeChart data={mockBuckets} isLoading={false} bucket="1d" />)

    expect(screen.getByTestId('responsive-container')).toBeInTheDocument()
  })
})
