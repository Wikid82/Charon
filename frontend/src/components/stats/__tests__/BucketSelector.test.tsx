import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, it, expect, vi } from 'vitest'

import { BucketSelector } from '../BucketSelector'

describe('BucketSelector', () => {
  it('renders all three bucket options', () => {
    render(<BucketSelector value="1h" onChange={vi.fn()} />)

    expect(screen.getByText('1h')).toBeInTheDocument()
    expect(screen.getByText('6h')).toBeInTheDocument()
    expect(screen.getByText('1d')).toBeInTheDocument()
  })

  it('marks the active bucket as checked', () => {
    render(<BucketSelector value="6h" onChange={vi.fn()} />)

    expect(screen.getByText('6h')).toHaveAttribute('aria-checked', 'true')
    expect(screen.getByText('1h')).toHaveAttribute('aria-checked', 'false')
    expect(screen.getByText('1d')).toHaveAttribute('aria-checked', 'false')
  })

  it('calls onChange with "1h" when 1h button is clicked', async () => {
    const user = userEvent.setup()
    const onChange = vi.fn()
    render(<BucketSelector value="6h" onChange={onChange} />)

    await user.click(screen.getByText('1h'))

    expect(onChange).toHaveBeenCalledExactlyOnceWith('1h')
  })

  it('calls onChange with "6h" when 6h button is clicked', async () => {
    const user = userEvent.setup()
    const onChange = vi.fn()
    render(<BucketSelector value="1h" onChange={onChange} />)

    await user.click(screen.getByText('6h'))

    expect(onChange).toHaveBeenCalledWith('6h')
  })

  it('calls onChange with "1d" when 1d button is clicked', async () => {
    const user = userEvent.setup()
    const onChange = vi.fn()
    render(<BucketSelector value="1h" onChange={onChange} />)

    await user.click(screen.getByText('1d'))

    expect(onChange).toHaveBeenCalledWith('1d')
  })

  it('renders a group with accessible label', () => {
    render(<BucketSelector value="1h" onChange={vi.fn()} />)

    expect(screen.getByRole('group', { name: 'Select bucket granularity' })).toBeInTheDocument()
  })

  it('all buttons are keyboard accessible as radio roles', () => {
    render(<BucketSelector value="1h" onChange={vi.fn()} />)

    const buttons = screen.getAllByRole('radio')
    expect(buttons).toHaveLength(3)
    for (const btn of buttons) {
      expect(btn.tagName).toBe('BUTTON')
    }
  })
})
