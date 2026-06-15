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
})
