import { render, screen } from '@testing-library/react'
import { describe, it, expect } from 'vitest'
import { CharonLoader, CharonCoinLoader, CerberusLoader, ConfigReloadOverlay } from '../LoadingStates'

describe('CharonLoader', () => {
  it('renders boat animation with accessibility label', () => {
    render(<CharonLoader />)
    expect(screen.getByRole('status')).toHaveAttribute('aria-label', 'Loading')
  })

  it('renders with different sizes', () => {
    const { rerender } = render(<CharonLoader size="sm" />)
    expect(screen.getByRole('status')).toBeInTheDocument()

    rerender(<CharonLoader size="lg" />)
    expect(screen.getByRole('status')).toBeInTheDocument()
  })
})

describe('CharonCoinLoader', () => {
  it('renders coin animation with accessibility label', () => {
    render(<CharonCoinLoader />)
    expect(screen.getByRole('status')).toHaveAttribute('aria-label', 'Authenticating')
  })

  it('renders with different sizes', () => {
    const { rerender } = render(<CharonCoinLoader size="sm" />)
    expect(screen.getByRole('status')).toBeInTheDocument()

    rerender(<CharonCoinLoader size="lg" />)
    expect(screen.getByRole('status')).toBeInTheDocument()
  })
})

describe('CerberusLoader', () => {
  it('renders guardian animation with accessibility label', () => {
    render(<CerberusLoader />)
    expect(screen.getByRole('status')).toHaveAttribute('aria-label', 'Security Loading')
  })

  it('renders with different sizes', () => {
    const { rerender } = render(<CerberusLoader size="sm" />)
    expect(screen.getByRole('status')).toBeInTheDocument()

    rerender(<CerberusLoader size="lg" />)
    expect(screen.getByRole('status')).toBeInTheDocument()
  })
})

describe('ConfigReloadOverlay', () => {
  it('renders with Charon theme (default)', () => {
    render(<ConfigReloadOverlay />)
    expect(screen.getByText('Ferrying configuration...')).toBeInTheDocument()
    expect(screen.getByText('Charon is crossing the Styx')).toBeInTheDocument()
  })

  it('renders with Coin theme', () => {
    render(
      <ConfigReloadOverlay
        message="Paying the ferryman..."
        submessage="Your obol grants passage"
        type="coin"
      />
    )
    expect(screen.getByText('Paying the ferryman...')).toBeInTheDocument()
    expect(screen.getByText('Your obol grants passage')).toBeInTheDocument()
  })

  it('renders with Cerberus theme', () => {
    render(
      <ConfigReloadOverlay
        message="Cerberus awakens..."
        submessage="Guardian of the gates stands watch"
        type="cerberus"
      />
    )
    expect(screen.getByText('Cerberus awakens...')).toBeInTheDocument()
    expect(screen.getByText('Guardian of the gates stands watch')).toBeInTheDocument()
  })

  it('renders with custom messages', () => {
    render(
      <ConfigReloadOverlay
        message="Custom message"
        submessage="Custom submessage"
        type="charon"
      />
    )
    expect(screen.getByText('Custom message')).toBeInTheDocument()
    expect(screen.getByText('Custom submessage')).toBeInTheDocument()
  })

  it('applies correct theme colors', () => {
    const { container, rerender } = render(<ConfigReloadOverlay type="charon" />)
    let overlay = container.querySelector('.bg-blue-950\\/90')
    expect(overlay).toBeInTheDocument()

    rerender(<ConfigReloadOverlay type="coin" />)
    overlay = container.querySelector('.bg-amber-950\\/90')
    expect(overlay).toBeInTheDocument()

    rerender(<ConfigReloadOverlay type="cerberus" />)
    overlay = container.querySelector('.bg-red-950\\/90')
    expect(overlay).toBeInTheDocument()
  })

  it('renders as full-screen overlay with high z-index', () => {
    const { container } = render(<ConfigReloadOverlay />)
    const overlay = container.querySelector('.fixed.inset-0.z-50')
    expect(overlay).toBeInTheDocument()
  })
})
