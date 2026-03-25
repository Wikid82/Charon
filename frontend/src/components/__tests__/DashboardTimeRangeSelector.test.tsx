import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, it, expect, vi } from 'vitest'

import { DashboardTimeRangeSelector } from '../crowdsec/DashboardTimeRangeSelector'

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string, fallback: string) => fallback ?? key,
  }),
}))

describe('DashboardTimeRangeSelector', () => {
  it('renders all time range options', () => {
    render(<DashboardTimeRangeSelector value="24h" onChange={vi.fn()} />)

    expect(screen.getByRole('radio', { name: '1H' })).toBeInTheDocument()
    expect(screen.getByRole('radio', { name: '6H' })).toBeInTheDocument()
    expect(screen.getByRole('radio', { name: '24H' })).toBeInTheDocument()
    expect(screen.getByRole('radio', { name: '7D' })).toBeInTheDocument()
    expect(screen.getByRole('radio', { name: '30D' })).toBeInTheDocument()
  })

  it('marks the selected range as aria-checked', () => {
    render(<DashboardTimeRangeSelector value="7d" onChange={vi.fn()} />)

    expect(screen.getByRole('radio', { name: '7D' })).toHaveAttribute('aria-checked', 'true')
    expect(screen.getByRole('radio', { name: '24H' })).toHaveAttribute('aria-checked', 'false')
  })

  it('applies roving tabindex — only selected has tabIndex 0', () => {
    render(<DashboardTimeRangeSelector value="24h" onChange={vi.fn()} />)

    expect(screen.getByRole('radio', { name: '24H' })).toHaveAttribute('tabindex', '0')
    expect(screen.getByRole('radio', { name: '1H' })).toHaveAttribute('tabindex', '-1')
    expect(screen.getByRole('radio', { name: '30D' })).toHaveAttribute('tabindex', '-1')
  })

  it('calls onChange when a range is clicked', async () => {
    const user = userEvent.setup()
    const onChange = vi.fn()
    render(<DashboardTimeRangeSelector value="24h" onChange={onChange} />)

    await user.click(screen.getByRole('radio', { name: '1H' }))

    expect(onChange).toHaveBeenCalledWith('1h')
  })

  it('uses radiogroup role on the container', () => {
    render(<DashboardTimeRangeSelector value="24h" onChange={vi.fn()} />)

    expect(screen.getByRole('radiogroup')).toBeInTheDocument()
  })

  it('navigates forward with ArrowRight', async () => {
    const user = userEvent.setup()
    const onChange = vi.fn()
    render(<DashboardTimeRangeSelector value="24h" onChange={onChange} />)

    screen.getByRole('radio', { name: '24H' }).focus()
    await user.keyboard('{ArrowRight}')

    expect(onChange).toHaveBeenCalledWith('7d')
  })

  it('navigates backward with ArrowLeft', async () => {
    const user = userEvent.setup()
    const onChange = vi.fn()
    render(<DashboardTimeRangeSelector value="24h" onChange={onChange} />)

    screen.getByRole('radio', { name: '24H' }).focus()
    await user.keyboard('{ArrowLeft}')

    expect(onChange).toHaveBeenCalledWith('6h')
  })

  it('wraps around from last to first with ArrowRight', async () => {
    const user = userEvent.setup()
    const onChange = vi.fn()
    render(<DashboardTimeRangeSelector value="30d" onChange={onChange} />)

    screen.getByRole('radio', { name: '30D' }).focus()
    await user.keyboard('{ArrowRight}')

    expect(onChange).toHaveBeenCalledWith('1h')
  })

  it('jumps to first with Home key', async () => {
    const user = userEvent.setup()
    const onChange = vi.fn()
    render(<DashboardTimeRangeSelector value="7d" onChange={onChange} />)

    screen.getByRole('radio', { name: '7D' }).focus()
    await user.keyboard('{Home}')

    expect(onChange).toHaveBeenCalledWith('1h')
  })

  it('jumps to last with End key', async () => {
    const user = userEvent.setup()
    const onChange = vi.fn()
    render(<DashboardTimeRangeSelector value="1h" onChange={onChange} />)

    screen.getByRole('radio', { name: '1H' }).focus()
    await user.keyboard('{End}')

    expect(onChange).toHaveBeenCalledWith('30d')
  })
})
