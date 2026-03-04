import { render, screen } from '@testing-library/react'
import { describe, it, expect } from 'vitest'
import { Users } from 'lucide-react'
import { StatsCard, type StatsCardChange } from '../StatsCard'

describe('StatsCard', () => {
  it('renders with title and value', () => {
    render(<StatsCard title="Total Users" value={1234} />)

    expect(screen.getByText('Total Users')).toBeInTheDocument()
    expect(screen.getByText('1234')).toBeInTheDocument()
  })

  it('renders with string value', () => {
    render(<StatsCard title="Revenue" value="$10,000" />)

    expect(screen.getByText('Revenue')).toBeInTheDocument()
    expect(screen.getByText('$10,000')).toBeInTheDocument()
  })

  it('renders with icon', () => {
    render(
      <StatsCard
        title="Users"
        value={100}
        icon={<Users data-testid="users-icon" />}
      />
    )

    expect(screen.getByTestId('users-icon')).toBeInTheDocument()
    // Icon container should have brand styling
    const iconContainer = screen.getByTestId('users-icon').parentElement
    expect(iconContainer).toHaveClass('bg-brand-500/10')
    expect(iconContainer).toHaveClass('text-brand-500')
  })

  it('renders as link when href is provided', () => {
    render(<StatsCard title="Dashboard" value={50} href="/dashboard" />)

    const link = screen.getByRole('link')
    expect(link).toBeInTheDocument()
    expect(link).toHaveAttribute('href', '/dashboard')
  })

  it('renders as div when href is not provided', () => {
    render(<StatsCard title="Static Card" value={25} />)

    expect(screen.queryByRole('link')).not.toBeInTheDocument()
    const card = screen.getByText('Static Card').closest('div')
    expect(card).toBeInTheDocument()
  })

  it('renders with upward trend', () => {
    const change: StatsCardChange = {
      value: 12,
      trend: 'up',
    }

    render(<StatsCard title="Growth" value={100} change={change} />)

    expect(screen.getByText('12%')).toBeInTheDocument()
    // Should have success color for upward trend
    const trendContainer = screen.getByText('12%').closest('div')
    expect(trendContainer).toHaveClass('text-success')
  })

  it('renders with downward trend', () => {
    const change: StatsCardChange = {
      value: 8,
      trend: 'down',
    }

    render(<StatsCard title="Decline" value={50} change={change} />)

    expect(screen.getByText('8%')).toBeInTheDocument()
    // Should have error color for downward trend
    const trendContainer = screen.getByText('8%').closest('div')
    expect(trendContainer).toHaveClass('text-error')
  })

  it('renders with neutral trend', () => {
    const change: StatsCardChange = {
      value: 0,
      trend: 'neutral',
    }

    render(<StatsCard title="Stable" value={75} change={change} />)

    expect(screen.getByText('0%')).toBeInTheDocument()
    // Should have muted color for neutral trend
    const trendContainer = screen.getByText('0%').closest('div')
    expect(trendContainer).toHaveClass('text-content-muted')
  })

  it('renders trend with label', () => {
    const change: StatsCardChange = {
      value: 15,
      trend: 'up',
      label: 'from last month',
    }

    render(<StatsCard title="Monthly Growth" value={200} change={change} />)

    expect(screen.getByText('15%')).toBeInTheDocument()
    expect(screen.getByText('from last month')).toBeInTheDocument()
  })

  it('applies custom className', () => {
    const { container } = render(
      <StatsCard title="Custom" value={10} className="custom-class" />
    )

    const card = container.firstChild
    expect(card).toHaveClass('custom-class')
  })

  it('has hover styles when href is provided', () => {
    render(<StatsCard title="Hoverable" value={30} href="/test" />)

    const link = screen.getByRole('link')
    expect(link).toHaveClass('hover:shadow-md')
    expect(link).toHaveClass('hover:border-brand-500/50')
    expect(link).toHaveClass('cursor-pointer')
  })

  it('does not have interactive styles when href is not provided', () => {
    const { container } = render(<StatsCard title="Static" value={40} />)

    const card = container.firstChild
    expect(card).not.toHaveClass('cursor-pointer')
  })

  it('has focus styles for accessibility when interactive', () => {
    render(<StatsCard title="Focusable" value={60} href="/link" />)

    const link = screen.getByRole('link')
    expect(link).toHaveClass('focus:outline-none')
    expect(link).toHaveClass('focus-visible:ring-2')
  })

  it('renders all elements together correctly', () => {
    const change: StatsCardChange = {
      value: 5,
      trend: 'up',
      label: 'vs yesterday',
    }

    render(
      <StatsCard
        title="Complete Card"
        value="99.9%"
        change={change}
        icon={<Users data-testid="icon" />}
        href="/stats"
        className="test-class"
      />
    )

    expect(screen.getByText('Complete Card')).toBeInTheDocument()
    expect(screen.getByText('99.9%')).toBeInTheDocument()
    expect(screen.getByText('5%')).toBeInTheDocument()
    expect(screen.getByText('vs yesterday')).toBeInTheDocument()
    expect(screen.getByTestId('icon')).toBeInTheDocument()
    expect(screen.getByRole('link')).toHaveAttribute('href', '/stats')
    expect(screen.getByRole('link')).toHaveClass('test-class')
  })
})
