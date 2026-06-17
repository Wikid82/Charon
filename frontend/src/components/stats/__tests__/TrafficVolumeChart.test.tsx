import { render, screen } from '@testing-library/react'
import { describe, it, expect, vi } from 'vitest'

import { TrafficVolumeChart } from '../TrafficVolumeChart'

import type { TrafficBucket } from '../../../api/stats'

// Recharts v3 ESM exports don't reliably survive { ...importActual, override } in Vitest —
// the real LineChart renders instead of the mock, so children (Line, Tooltip) never mount.
// Mock only the named exports the component uses; no importActual needed.
vi.mock('recharts', () => ({
  ResponsiveContainer: ({ children }: { children: React.ReactNode }) => (
    <div data-testid="responsive-container">{children}</div>
  ),
  LineChart: ({ children }: { children: React.ReactNode }) => (
    <div data-testid="line-chart">{children}</div>
  ),
  // Expose Line props via data attributes for stroke regression testing.
  Line: ({ stroke, strokeWidth, dataKey }: { stroke?: string; strokeWidth?: number; dataKey?: string }) => (
    <div data-testid="line" data-stroke={stroke} data-stroke-width={String(strokeWidth ?? '')} data-datakey={dataKey ?? ''} />
  ),
  XAxis: () => null,
  CartesianGrid: () => null,
  // Exercise all three formatBytes branches via tickFormatter: MB, KB, B.
  YAxis: ({ tickFormatter }: { tickFormatter?: (value: number) => string }) => (
    <div data-testid="y-axis">
      <span>{tickFormatter?.(2_097_152)}</span>
      <span>{tickFormatter?.(2_048)}</span>
      <span>{tickFormatter?.(500)}</span>
    </div>
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
}))

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

    expect(screen.getByText('No data available yet')).toBeInTheDocument()
    expect(screen.getByText(/Data is being collected/)).toBeInTheDocument()
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

  it('renders info tooltip trigger button', () => {
    render(<TrafficVolumeChart data={mockBuckets} isLoading={false} bucket="1h" />)

    expect(screen.getByRole('button', { name: 'About this widget' })).toBeInTheDocument()
  })

  it('passes a valid hex color to the Line stroke prop', () => {
    render(<TrafficVolumeChart data={mockBuckets} isLoading={false} bucket="1h" />)

    const line = screen.getByTestId('line')
    const stroke = line.getAttribute('data-stroke')

    expect(stroke).toMatch(/^#[0-9a-f]{6}$/i)
    expect(stroke).toBe('#3b82f6')
  })

  it('tooltip renders bytes value correctly when line has valid stroke', () => {
    render(<TrafficVolumeChart data={mockBuckets} isLoading={false} bucket="1h" />)

    expect(screen.getByText(/1\.0 MB sent/i)).toBeInTheDocument()
  })
})
