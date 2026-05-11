import { render, screen } from '@testing-library/react'
import { describe, it, expect } from 'vitest'

import { TunnelStatusBadge } from '../TunnelStatusBadge'

describe('TunnelStatusBadge', () => {
  it('renders "connected" state with success variant label', () => {
    render(<TunnelStatusBadge state="connected" />)
    const badge = screen.getByRole('status')
    expect(badge).toBeInTheDocument()
    expect(badge).toHaveTextContent('Connected')
    expect(badge).toHaveAttribute('aria-label', expect.stringContaining('Connected'))
  })

  it('renders "connecting" state', () => {
    render(<TunnelStatusBadge state="connecting" />)
    const badge = screen.getByRole('status')
    expect(badge).toHaveTextContent('Connecting')
    expect(badge).toHaveAttribute('aria-label', expect.stringContaining('Connecting'))
  })

  it('renders "error" state', () => {
    render(<TunnelStatusBadge state="error" />)
    const badge = screen.getByRole('status')
    expect(badge).toHaveTextContent('Error')
    expect(badge).toHaveAttribute('aria-label', expect.stringContaining('Error'))
  })

  it('renders "stopped" state', () => {
    render(<TunnelStatusBadge state="stopped" />)
    const badge = screen.getByRole('status')
    expect(badge).toHaveTextContent('Stopped')
    expect(badge).toHaveAttribute('aria-label', expect.stringContaining('Stopped'))
  })

  it('hides label when showLabel=false, but still shows icon', () => {
    const { container } = render(<TunnelStatusBadge state="connected" showLabel={false} />)
    const badge = screen.getByRole('status')
    // The span text should not be present
    expect(badge.querySelector('span')).not.toBeInTheDocument()
    // Icon (svg) should still be present
    expect(container.querySelector('svg')).toBeInTheDocument()
    // aria-label still set for screen readers
    expect(badge).toHaveAttribute('aria-label', expect.stringContaining('Connected'))
  })

  it('applies custom className', () => {
    render(<TunnelStatusBadge state="connected" className="custom-class" />)
    const badge = screen.getByRole('status')
    expect(badge).toHaveClass('custom-class')
  })

  it('icon has aria-hidden on it', () => {
    const { container } = render(<TunnelStatusBadge state="connected" />)
    const svg = container.querySelector('svg')
    expect(svg).toHaveAttribute('aria-hidden', 'true')
  })
})
