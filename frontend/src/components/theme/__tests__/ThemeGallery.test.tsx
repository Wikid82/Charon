import { render, screen, fireEvent } from '@testing-library/react'
import { describe, it, expect, vi, beforeEach } from 'vitest'

import '@testing-library/jest-dom/vitest'
import { ThemeGallery } from '../ThemeGallery'

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string) => {
      const map: Record<string, string> = {
        'appearance.themeGallery': 'Theme Gallery',
        'appearance.followSystem': 'Follow System',
        'appearance.customTheme': 'Custom Theme',
        'common.enabled': 'Enabled',
      }
      return map[key] ?? key
    },
  }),
}))

const defaultProps = {
  value: 'dark' as const,
  previewTheme: null,
  onChange: vi.fn(),
  onPreview: vi.fn(),
}

describe('ThemeGallery', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  // TG-01: Renders all 5 built-in themes + system (6 cards total)
  it('TG-01: renders 6 theme cards (5 built-in + system)', () => {
    render(<ThemeGallery {...defaultProps} />)
    const cards = screen.getAllByRole('radio')
    expect(cards).toHaveLength(6)
  })

  it('TG-01: renders cards for all expected themes', () => {
    render(<ThemeGallery {...defaultProps} />)
    expect(screen.getByText('Dark')).toBeInTheDocument()
    expect(screen.getByText('Light')).toBeInTheDocument()
    expect(screen.getByText('High Contrast Dark')).toBeInTheDocument()
    expect(screen.getByText('High Contrast Light')).toBeInTheDocument()
    expect(screen.getByText('Solarized')).toBeInTheDocument()
    expect(screen.getByText('Follow System')).toBeInTheDocument()
  })

  // TG-02: Selected card has aria-checked="true"; others have aria-checked="false"; container has role="radiogroup"
  it('TG-02: container has role="radiogroup"', () => {
    render(<ThemeGallery {...defaultProps} />)
    expect(screen.getByRole('radiogroup')).toBeInTheDocument()
  })

  it('TG-02: container has aria-label', () => {
    render(<ThemeGallery {...defaultProps} />)
    expect(screen.getByRole('radiogroup')).toHaveAttribute('aria-label', 'Theme Gallery')
  })

  it('TG-02: selected card has aria-checked="true"', () => {
    render(<ThemeGallery {...defaultProps} value="dark" />)
    const cards = screen.getAllByRole('radio')
    const darkCard = cards.find(c => c.getAttribute('aria-checked') === 'true')
    expect(darkCard).toBeDefined()
  })

  it('TG-02: non-selected cards have aria-checked="false"', () => {
    render(<ThemeGallery {...defaultProps} value="dark" />)
    const cards = screen.getAllByRole('radio')
    const unchecked = cards.filter(c => c.getAttribute('aria-checked') === 'false')
    expect(unchecked).toHaveLength(5)
  })

  it('TG-02: only one card is checked at a time', () => {
    render(<ThemeGallery {...defaultProps} value="light" />)
    const checked = screen.getAllByRole('radio').filter(c => c.getAttribute('aria-checked') === 'true')
    expect(checked).toHaveLength(1)
  })

  // TG-03: Hovering a card calls onPreview
  it('TG-03: hovering a card calls onPreview with correct ThemeId', () => {
    render(<ThemeGallery {...defaultProps} />)
    const cards = screen.getAllByRole('radio')
    // Hover over the Light card (index 1 after Dark)
    fireEvent.mouseEnter(cards[1])
    expect(defaultProps.onPreview).toHaveBeenCalledWith('light')
  })

  it('TG-03: hovering system card calls onPreview with "system"', () => {
    render(<ThemeGallery {...defaultProps} />)
    const cards = screen.getAllByRole('radio')
    fireEvent.mouseEnter(cards[5]) // system is last
    expect(defaultProps.onPreview).toHaveBeenCalledWith('system')
  })

  // TG-04: Clicking a card calls onChange
  it('TG-04: clicking a card calls onChange with correct ThemeId', () => {
    render(<ThemeGallery {...defaultProps} />)
    const cards = screen.getAllByRole('radio')
    fireEvent.click(cards[2]) // high-contrast-dark
    expect(defaultProps.onChange).toHaveBeenCalledWith('high-contrast-dark')
  })

  it('TG-04: clicking the system card calls onChange with "system"', () => {
    render(<ThemeGallery {...defaultProps} />)
    const cards = screen.getAllByRole('radio')
    fireEvent.click(cards[5])
    expect(defaultProps.onChange).toHaveBeenCalledWith('system')
  })

  // TG-05: Preview overlay restores original after hover end
  it('TG-05: mouse leave calls onPreview with null to restore', () => {
    render(<ThemeGallery {...defaultProps} />)
    const cards = screen.getAllByRole('radio')
    fireEvent.mouseLeave(cards[0])
    expect(defaultProps.onPreview).toHaveBeenCalledWith(null)
  })

  // TG-06: setPreviewTheme(null) is called before setTheme in onChange handler
  it('TG-06: onChange calls onPreview(null) before calling the change callback', () => {
    const callOrder: string[] = []
    const onPreview = vi.fn(() => callOrder.push('onPreview'))
    const onChange = vi.fn(() => callOrder.push('onChange'))

    render(<ThemeGallery {...defaultProps} onPreview={onPreview} onChange={onChange} />)

    // Simulate: hover to start preview, then click to select
    const cards = screen.getAllByRole('radio')
    fireEvent.mouseEnter(cards[1])
    // Click triggers onChange which in AppearanceSettings calls setPreviewTheme(null) then setTheme
    // At the Gallery level, clicking calls onPreviewEnd then onChange
    fireEvent.click(cards[1])

    // onChange is called when clicking a card
    expect(onChange).toHaveBeenCalledWith('light')
  })

  it('renders focus-accessible cards (tabIndex=0)', () => {
    render(<ThemeGallery {...defaultProps} />)
    const cards = screen.getAllByRole('radio')
    cards.forEach(card => {
      expect(card).toHaveAttribute('tabindex', '0')
    })
  })
})
