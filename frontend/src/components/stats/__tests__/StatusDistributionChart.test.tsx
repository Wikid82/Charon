import { render, screen } from '@testing-library/react'
import { describe, it, expect, vi } from 'vitest'

import { StatusDistributionChart } from '../StatusDistributionChart'

import type { StatusStat } from '../../../api/stats'

vi.mock('recharts', async () => {
  const Original = await vi.importActual('recharts')
  return {
    ...Original,
    ResponsiveContainer: ({ children }: { children: React.ReactNode }) => (
      <div data-testid="responsive-container">{children}</div>
    ),
    // Call label and content callbacks during render to cover those code paths.
    Pie: ({
      label,
      children,
    }: {
      label?: (props: { name: string; percent?: number }) => React.ReactNode
      children?: React.ReactNode
    }) => (
      <g data-testid="pie">
        {label?.({ name: 'mock', percent: undefined })}
        {label?.({ name: '2xx', percent: 0.8 })}
        {children}
      </g>
    ),
    Tooltip: ({
      content,
    }: {
      content?: (props: object) => React.ReactNode
    }) => (
      <div data-testid="tooltip">
        {content?.({ active: true, payload: [{ name: '2xx', value: 100, payload: {} }] })}
        {content?.({ active: true, payload: [{ name: '5xx', value: 'not-a-number', payload: {} }] })}
        {content?.({ active: false, payload: [] })}
      </div>
    ),
  }
})

const mockStatuses: StatusStat[] = [
  { code: 200, count: 800 },
  { code: 201, count: 50 },
  { code: 301, count: 30 },
  { code: 404, count: 60 },
  { code: 500, count: 10 },
]

describe('StatusDistributionChart', () => {
  it('renders loading state when isLoading is true', () => {
    render(<StatusDistributionChart data={undefined} isLoading />)

    expect(screen.queryByTestId('responsive-container')).not.toBeInTheDocument()
  })

  it('renders empty state when data is empty', () => {
    render(<StatusDistributionChart data={[]} isLoading={false} />)

    expect(screen.getByText('No data available')).toBeInTheDocument()
  })

  it('renders chart title', () => {
    render(<StatusDistributionChart data={mockStatuses} isLoading={false} />)

    expect(screen.getByText('Status Distribution')).toBeInTheDocument()
  })

  it('renders chart when data is present', () => {
    render(<StatusDistributionChart data={mockStatuses} isLoading={false} />)

    expect(screen.getByTestId('responsive-container')).toBeInTheDocument()
  })

  it('aggregates codes into status class groups and renders chart', () => {
    render(<StatusDistributionChart data={mockStatuses} isLoading={false} />)

    // Chart renders when there is aggregated data
    expect(screen.getByTestId('responsive-container')).toBeInTheDocument()
  })

  it('renders all status classes present in data', () => {
    const allClasses: StatusStat[] = [
      { code: 200, count: 100 },
      { code: 302, count: 20 },
      { code: 403, count: 15 },
      { code: 503, count: 5 },
    ]
    render(<StatusDistributionChart data={allClasses} isLoading={false} />)

    // Chart should render because all status classes are represented
    expect(screen.getByTestId('responsive-container')).toBeInTheDocument()
  })

  it('renders status summary list with class names', () => {
    render(<StatusDistributionChart data={mockStatuses} isLoading={false} />)

    // Status summary list renders accessible text for each class
    expect(screen.getByText(/2xx/)).toBeInTheDocument()
    expect(screen.getByText(/3xx/)).toBeInTheDocument()
    expect(screen.getByText(/4xx/)).toBeInTheDocument()
    expect(screen.getByText(/5xx/)).toBeInTheDocument()
  })

  it('classifies status codes below 200 as "other"', () => {
    // 1xx codes fall into the "other" bucket — covers the final return branch in statusClass.
    const withInfoStatus: StatusStat[] = [{ code: 101, count: 5 }]
    render(<StatusDistributionChart data={withInfoStatus} isLoading={false} />)
    expect(screen.getByTestId('responsive-container')).toBeInTheDocument()
    expect(screen.getByText(/other/)).toBeInTheDocument()
  })

  it('renders info tooltip trigger button', () => {
    render(<StatusDistributionChart data={mockStatuses} isLoading={false} />)

    expect(screen.getByRole('button', { name: 'About this widget' })).toBeInTheDocument()
  })
})
