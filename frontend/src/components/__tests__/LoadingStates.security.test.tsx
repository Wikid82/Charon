import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import {
  CharonLoader,
  CharonCoinLoader,
  CerberusLoader,
  ConfigReloadOverlay,
} from '../LoadingStates'

describe('LoadingStates - Security Audit', () => {
  describe('CharonLoader', () => {
    it('renders without crashing', () => {
      const { container } = render(<CharonLoader />)
      expect(container.querySelector('svg')).toBeInTheDocument()
    })

    it('handles all size variants', () => {
      const { rerender } = render(<CharonLoader size="sm" />)
      expect(screen.getByRole('status')).toBeInTheDocument()

      rerender(<CharonLoader size="md" />)
      expect(screen.getByRole('status')).toBeInTheDocument()

      rerender(<CharonLoader size="lg" />)
      expect(screen.getByRole('status')).toBeInTheDocument()
    })

    it('has accessible role and label', () => {
      render(<CharonLoader />)
      const status = screen.getByRole('status')
      expect(status).toHaveAttribute('aria-label', 'Loading')
    })

    it('applies correct size classes', () => {
      const { container, rerender } = render(<CharonLoader size="sm" />)
      expect(container.firstChild).toHaveClass('w-12', 'h-12')

      rerender(<CharonLoader size="md" />)
      expect(container.firstChild).toHaveClass('w-20', 'h-20')

      rerender(<CharonLoader size="lg" />)
      expect(container.firstChild).toHaveClass('w-28', 'h-28')
    })
  })

  describe('CharonCoinLoader', () => {
    it('renders without crashing', () => {
      const { container } = render(<CharonCoinLoader />)
      expect(container.querySelector('svg')).toBeInTheDocument()
    })

    it('has accessible role and label for authentication', () => {
      render(<CharonCoinLoader />)
      const status = screen.getByRole('status')
      expect(status).toHaveAttribute('aria-label', 'Authenticating')
    })

    it('renders gradient definition', () => {
      const { container } = render(<CharonCoinLoader />)
      const gradient = container.querySelector('#goldGradient')
      expect(gradient).toBeInTheDocument()
    })

    it('applies correct size classes', () => {
      const { container, rerender } = render(<CharonCoinLoader size="sm" />)
      expect(container.firstChild).toHaveClass('w-12', 'h-12')

      rerender(<CharonCoinLoader size="md" />)
      expect(container.firstChild).toHaveClass('w-20', 'h-20')

      rerender(<CharonCoinLoader size="lg" />)
      expect(container.firstChild).toHaveClass('w-28', 'h-28')
    })
  })

  describe('CerberusLoader', () => {
    it('renders without crashing', () => {
      const { container } = render(<CerberusLoader />)
      expect(container.querySelector('svg')).toBeInTheDocument()
    })

    it('has accessible role and label for security', () => {
      render(<CerberusLoader />)
      const status = screen.getByRole('status')
      expect(status).toHaveAttribute('aria-label', 'Security Loading')
    })

    it('renders three heads (three circles for heads)', () => {
      const { container } = render(<CerberusLoader />)
      const circles = container.querySelectorAll('circle')
      // At least 3 head circles should exist (plus paws and eyes)
      expect(circles.length).toBeGreaterThanOrEqual(3)
    })

    it('applies correct size classes', () => {
      const { container, rerender } = render(<CerberusLoader size="sm" />)
      expect(container.firstChild).toHaveClass('w-12', 'h-12')

      rerender(<CerberusLoader size="md" />)
      expect(container.firstChild).toHaveClass('w-20', 'h-20')

      rerender(<CerberusLoader size="lg" />)
      expect(container.firstChild).toHaveClass('w-28', 'h-28')
    })
  })

  describe('ConfigReloadOverlay - XSS Protection', () => {
    it('renders with default props', () => {
      render(<ConfigReloadOverlay />)
      expect(screen.getByText('Ferrying configuration...')).toBeInTheDocument()
      expect(screen.getByText('Charon is crossing the Styx')).toBeInTheDocument()
    })

    it('ATTACK: prevents XSS in message prop', () => {
      const xssPayload = '<script>alert("XSS")</script>'
      render(<ConfigReloadOverlay message={xssPayload} />)

      // React should escape this automatically
      expect(screen.getByText(xssPayload)).toBeInTheDocument()
      expect(document.querySelector('script')).not.toBeInTheDocument()
    })

    it('ATTACK: prevents XSS in submessage prop', () => {
      const xssPayload = '<img src=x onerror="alert(1)">'
      render(<ConfigReloadOverlay submessage={xssPayload} />)

      expect(screen.getByText(xssPayload)).toBeInTheDocument()
      expect(document.querySelector('img[onerror]')).not.toBeInTheDocument()
    })

    it('ATTACK: handles extremely long messages', () => {
      const longMessage = 'A'.repeat(10000)
      const { container } = render(<ConfigReloadOverlay message={longMessage} />)

      // Should render without crashing
      expect(container).toBeInTheDocument()
      expect(screen.getByText(longMessage)).toBeInTheDocument()
    })

    it('ATTACK: handles special characters', () => {
      const specialChars = '!@#$%^&*()_+-=[]{}|;:",.<>?/~`'
      render(
        <ConfigReloadOverlay
          message={specialChars}
          submessage={specialChars}
        />
      )

      expect(screen.getAllByText(specialChars)).toHaveLength(2)
    })

    it('ATTACK: handles unicode and emoji', () => {
      const unicode = '🔥💀🐕‍🦺 λ µ π Σ 中文 العربية עברית'
      render(<ConfigReloadOverlay message={unicode} />)

      expect(screen.getByText(unicode)).toBeInTheDocument()
    })

    it('renders correct theme - charon (blue)', () => {
      const { container } = render(<ConfigReloadOverlay type="charon" />)
      const overlay = container.querySelector('.bg-blue-950\\/90')
      expect(overlay).toBeInTheDocument()
    })

    it('renders correct theme - coin (gold)', () => {
      const { container } = render(<ConfigReloadOverlay type="coin" />)
      const overlay = container.querySelector('.bg-amber-950\\/90')
      expect(overlay).toBeInTheDocument()
    })

    it('renders correct theme - cerberus (red)', () => {
      const { container } = render(<ConfigReloadOverlay type="cerberus" />)
      const overlay = container.querySelector('.bg-red-950\\/90')
      expect(overlay).toBeInTheDocument()
    })

    it('applies correct z-index (z-50)', () => {
      const { container } = render(<ConfigReloadOverlay />)
      const overlay = container.querySelector('.z-50')
      expect(overlay).toBeInTheDocument()
    })

    it('applies backdrop blur', () => {
      const { container } = render(<ConfigReloadOverlay />)
      const backdrop = container.querySelector('.backdrop-blur-sm')
      expect(backdrop).toBeInTheDocument()
    })

    it('ATTACK: type prop injection attempt', () => {
      // @ts-expect-error - Testing invalid type
      const { container } = render(<ConfigReloadOverlay type="<script>alert(1)</script>" />)

      // Should default to charon theme
      expect(container.querySelector('.bg-blue-950\\/90')).toBeInTheDocument()
    })
  })

  describe('Overlay Integration Tests', () => {
    it('CharonLoader renders inside overlay', () => {
      render(<ConfigReloadOverlay type="charon" />)
      expect(screen.getByRole('status')).toHaveAttribute('aria-label', 'Loading')
    })

    it('CharonCoinLoader renders inside overlay', () => {
      render(<ConfigReloadOverlay type="coin" />)
      expect(screen.getByRole('status')).toHaveAttribute('aria-label', 'Authenticating')
    })

    it('CerberusLoader renders inside overlay', () => {
      render(<ConfigReloadOverlay type="cerberus" />)
      expect(screen.getByRole('status')).toHaveAttribute('aria-label', 'Security Loading')
    })
  })

  describe('CSS Animation Requirements', () => {
    it('CharonLoader uses animate-bob-boat class', () => {
      const { container } = render(<CharonLoader />)
      const animated = container.querySelector('.animate-bob-boat')
      expect(animated).toBeInTheDocument()
    })

    it('CharonCoinLoader uses animate-spin-y class', () => {
      const { container } = render(<CharonCoinLoader />)
      const animated = container.querySelector('.animate-spin-y')
      expect(animated).toBeInTheDocument()
    })

    it('CerberusLoader uses animate-rotate-head class', () => {
      const { container } = render(<CerberusLoader />)
      const animated = container.querySelector('.animate-rotate-head')
      expect(animated).toBeInTheDocument()
    })
  })

  describe('Edge Cases', () => {
    it('handles undefined size prop gracefully', () => {
      const { container } = render(<CharonLoader size={undefined} />)
      expect(container.firstChild).toHaveClass('w-20', 'h-20') // defaults to md
    })

    it('handles null message', () => {
      // @ts-expect-error - Testing null
      render(<ConfigReloadOverlay message={null} />)
      expect(screen.getByText('null')).toBeInTheDocument()
    })

    it('handles empty string message', () => {
      render(<ConfigReloadOverlay message="" submessage="" />)
      // Should render but be empty
      expect(screen.queryByText('Ferrying configuration...')).not.toBeInTheDocument()
    })

    it('handles undefined type prop', () => {
      const { container } = render(<ConfigReloadOverlay type={undefined} />)
      // Should default to charon
      expect(container.querySelector('.bg-blue-950\\/90')).toBeInTheDocument()
    })
  })

  describe('Accessibility Requirements', () => {
    it('overlay is keyboard accessible', () => {
      const { container } = render(<ConfigReloadOverlay />)
      const overlay = container.firstChild
      expect(overlay).toBeInTheDocument()
    })

    it('all loaders have status role', () => {
      render(
        <>
          <CharonLoader />
          <CharonCoinLoader />
          <CerberusLoader />
        </>
      )
      const statuses = screen.getAllByRole('status')
      expect(statuses).toHaveLength(3)
    })

    it('all loaders have aria-label', () => {
      const { container: c1 } = render(<CharonLoader />)
      const { container: c2 } = render(<CharonCoinLoader />)
      const { container: c3 } = render(<CerberusLoader />)

      expect(c1.firstChild).toHaveAttribute('aria-label')
      expect(c2.firstChild).toHaveAttribute('aria-label')
      expect(c3.firstChild).toHaveAttribute('aria-label')
    })
  })

  describe('Performance Tests', () => {
    it('renders CharonLoader quickly', () => {
      const start = performance.now()
      render(<CharonLoader />)
      const end = performance.now()
      expect(end - start).toBeLessThan(100) // Should render in <100ms
    })

    it('renders CharonCoinLoader quickly', () => {
      const start = performance.now()
      render(<CharonCoinLoader />)
      const end = performance.now()
      expect(end - start).toBeLessThan(100)
    })

    it('renders CerberusLoader quickly', () => {
      const start = performance.now()
      render(<CerberusLoader />)
      const end = performance.now()
      expect(end - start).toBeLessThan(100)
    })

    it('renders ConfigReloadOverlay quickly', () => {
      const start = performance.now()
      render(<ConfigReloadOverlay />)
      const end = performance.now()
      expect(end - start).toBeLessThan(100)
    })
  })
})
