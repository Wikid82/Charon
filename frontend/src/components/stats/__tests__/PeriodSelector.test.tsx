import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, it, expect, vi } from 'vitest'

import { PeriodSelector } from '../PeriodSelector'

describe('PeriodSelector', () => {
  it('renders all three period options', () => {
    render(<PeriodSelector value="24h" onChange={vi.fn()} />)

    expect(screen.getByText('24h')).toBeInTheDocument()
    expect(screen.getByText('7d')).toBeInTheDocument()
    expect(screen.getByText('30d')).toBeInTheDocument()
  })

  it('marks the active period as checked', () => {
    render(<PeriodSelector value="7d" onChange={vi.fn()} />)

    expect(screen.getByText('7d')).toHaveAttribute('aria-checked', 'true')
    expect(screen.getByText('24h')).toHaveAttribute('aria-checked', 'false')
    expect(screen.getByText('30d')).toHaveAttribute('aria-checked', 'false')
  })

  it('calls onChange with "24h" when 24h button is clicked', async () => {
    const user = userEvent.setup()
    const onChange = vi.fn()
    render(<PeriodSelector value="7d" onChange={onChange} />)

    await user.click(screen.getByText('24h'))

    expect(onChange).toHaveBeenCalledExactlyOnceWith('24h')
  })

  it('calls onChange with "7d" when 7d button is clicked', async () => {
    const user = userEvent.setup()
    const onChange = vi.fn()
    render(<PeriodSelector value="24h" onChange={onChange} />)

    await user.click(screen.getByText('7d'))

    expect(onChange).toHaveBeenCalledWith('7d')
  })

  it('calls onChange with "30d" when 30d button is clicked', async () => {
    const user = userEvent.setup()
    const onChange = vi.fn()
    render(<PeriodSelector value="24h" onChange={onChange} />)

    await user.click(screen.getByText('30d'))

    expect(onChange).toHaveBeenCalledWith('30d')
  })

  it('renders a group with accessible label', () => {
    render(<PeriodSelector value="24h" onChange={vi.fn()} />)

    expect(screen.getByRole('group', { name: 'Select time period' })).toBeInTheDocument()
  })

  it('all buttons are keyboard accessible (type=button)', () => {
    render(<PeriodSelector value="24h" onChange={vi.fn()} />)

    const buttons = screen.getAllByRole('radio')
    expect(buttons).toHaveLength(3)
    for (const btn of buttons) {
      expect(btn.tagName).toBe('BUTTON')
    }
  })
})
