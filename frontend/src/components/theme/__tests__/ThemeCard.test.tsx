import { render, screen, fireEvent } from '@testing-library/react'
import { describe, it, expect, vi, beforeEach } from 'vitest'

import '@testing-library/jest-dom/vitest'
import { ThemeCard } from '../ThemeCard'

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string) => {
      const map: Record<string, string> = {
        'appearance.themeGallery': 'Theme Gallery',
        'common.enabled': 'Enabled',
      }
      return map[key] ?? key
    },
  }),
}))

const defaultProps = {
  themeId: 'dark' as const,
  label: 'Dark',
  selected: false,
  onSelect: vi.fn(),
  onPreview: vi.fn(),
  onPreviewEnd: vi.fn(),
}

describe('ThemeCard', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('renders the label', () => {
    render(<ThemeCard {...defaultProps} />)
    expect(screen.getByText('Dark')).toBeInTheDocument()
  })

  it('has role="radio"', () => {
    render(<ThemeCard {...defaultProps} />)
    expect(screen.getByRole('radio')).toBeInTheDocument()
  })

  it('has aria-checked="false" when not selected', () => {
    render(<ThemeCard {...defaultProps} selected={false} />)
    expect(screen.getByRole('radio')).toHaveAttribute('aria-checked', 'false')
  })

  it('has aria-checked="true" when selected', () => {
    render(<ThemeCard {...defaultProps} selected={true} />)
    expect(screen.getByRole('radio')).toHaveAttribute('aria-checked', 'true')
  })

  it('calls onSelect on click', () => {
    render(<ThemeCard {...defaultProps} />)
    fireEvent.click(screen.getByRole('radio'))
    expect(defaultProps.onSelect).toHaveBeenCalledTimes(1)
  })

  it('calls onPreview on mouseenter', () => {
    render(<ThemeCard {...defaultProps} />)
    fireEvent.mouseEnter(screen.getByRole('radio'))
    expect(defaultProps.onPreview).toHaveBeenCalledTimes(1)
  })

  it('calls onPreviewEnd on mouseleave', () => {
    render(<ThemeCard {...defaultProps} />)
    fireEvent.mouseLeave(screen.getByRole('radio'))
    expect(defaultProps.onPreviewEnd).toHaveBeenCalledTimes(1)
  })

  it('calls onPreview on focus', () => {
    render(<ThemeCard {...defaultProps} />)
    fireEvent.focus(screen.getByRole('radio'))
    expect(defaultProps.onPreview).toHaveBeenCalledTimes(1)
  })

  it('calls onPreviewEnd on blur', () => {
    render(<ThemeCard {...defaultProps} />)
    fireEvent.blur(screen.getByRole('radio'))
    expect(defaultProps.onPreviewEnd).toHaveBeenCalledTimes(1)
  })

  it('calls onSelect on Enter key press', () => {
    render(<ThemeCard {...defaultProps} />)
    fireEvent.keyDown(screen.getByRole('radio'), { key: 'Enter' })
    expect(defaultProps.onSelect).toHaveBeenCalledTimes(1)
  })

  it('calls onSelect on Space key press', () => {
    render(<ThemeCard {...defaultProps} />)
    fireEvent.keyDown(screen.getByRole('radio'), { key: ' ' })
    expect(defaultProps.onSelect).toHaveBeenCalledTimes(1)
  })

  it('does not call onSelect on other key press', () => {
    render(<ThemeCard {...defaultProps} />)
    fireEvent.keyDown(screen.getByRole('radio'), { key: 'Tab' })
    expect(defaultProps.onSelect).not.toHaveBeenCalled()
  })

  it('shows a checkmark when selected', () => {
    render(<ThemeCard {...defaultProps} selected={true} />)
    // The Check icon is aria-hidden, verify the selected styling class is present
    const card = screen.getByRole('radio')
    expect(card).toHaveClass('border-brand-500')
  })

  it('does not show selected indicator border when not selected', () => {
    render(<ThemeCard {...defaultProps} selected={false} />)
    const card = screen.getByRole('radio')
    expect(card).not.toHaveClass('border-brand-500')
  })

  it('is focusable with tabIndex=0', () => {
    render(<ThemeCard {...defaultProps} />)
    expect(screen.getByRole('radio')).toHaveAttribute('tabindex', '0')
  })

  it('renders color swatches for all themes without errors', () => {
    const themeIds = ['dark', 'light', 'high-contrast-dark', 'high-contrast-light', 'solarized', 'system'] as const
    for (const themeId of themeIds) {
      const { unmount } = render(<ThemeCard {...defaultProps} themeId={themeId} label={themeId} />)
      expect(screen.getByRole('radio')).toBeInTheDocument()
      unmount()
    }
  })
})
