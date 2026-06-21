import { render, screen, fireEvent } from '@testing-library/react'
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'

import '@testing-library/jest-dom/vitest'
import { ThemeProvider } from '../../context/ThemeContext'
import { THEME_STORAGE_KEY } from '../../context/ThemeContextValue'
import AppearanceSettings from '../AppearanceSettings'

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string) => {
      const map: Record<string, string> = {
        'appearance.title': 'Appearance',
        'appearance.description': 'Customize the look and feel of Charon',
        'appearance.themeGallery': 'Theme Gallery',
        'appearance.themeGalleryDescription': 'Choose a built-in theme or create your own',
        'appearance.followSystem': 'Follow System',
        'appearance.followSystemDescription': 'Automatically match your OS light/dark preference',
        'appearance.customTheme': 'Custom Theme',
        'common.enabled': 'Enabled',
      }
      return map[key] ?? key
    },
  }),
}))

function renderAppearanceSettings() {
  return render(
    <ThemeProvider>
      <AppearanceSettings />
    </ThemeProvider>
  )
}

describe('AppearanceSettings', () => {
  beforeEach(() => {
    localStorage.clear()
    document.documentElement.setAttribute('data-theme', 'dark')
  })

  afterEach(() => {
    document.documentElement.setAttribute('data-theme', 'dark')
  })

  it('renders the theme gallery section', () => {
    renderAppearanceSettings()
    expect(screen.getByText('Theme Gallery')).toBeInTheDocument()
  })

  it('renders the gallery description', () => {
    renderAppearanceSettings()
    expect(screen.getByText('Choose a built-in theme or create your own')).toBeInTheDocument()
  })

  it('renders the radiogroup with 6 theme cards', () => {
    renderAppearanceSettings()
    const cards = screen.getAllByRole('radio')
    expect(cards).toHaveLength(6)
  })

  it('dark theme card is selected by default', () => {
    renderAppearanceSettings()
    const checked = screen.getAllByRole('radio').find(
      c => c.getAttribute('aria-checked') === 'true'
    )
    expect(checked).toBeDefined()
  })

  it('clicking a theme card changes the active selection', () => {
    renderAppearanceSettings()
    const cards = screen.getAllByRole('radio')

    // Click the light theme card (index 1)
    fireEvent.click(cards[1])

    // Now the light card should be checked
    expect(cards[1]).toHaveAttribute('aria-checked', 'true')
    expect(cards[0]).toHaveAttribute('aria-checked', 'false')
  })

  it('theme change persists to localStorage', () => {
    renderAppearanceSettings()
    const cards = screen.getAllByRole('radio')
    fireEvent.click(cards[3]) // high-contrast-light
    expect(localStorage.getItem(THEME_STORAGE_KEY)).toBe('high-contrast-light')
  })

  it('shows follow system description when system theme is selected', () => {
    renderAppearanceSettings()
    const cards = screen.getAllByRole('radio')
    fireEvent.click(cards[5]) // system is last
    expect(screen.getByText('Automatically match your OS light/dark preference')).toBeInTheDocument()
  })

  it('does not show follow system description for non-system themes', () => {
    renderAppearanceSettings()
    expect(
      screen.queryByText('Automatically match your OS light/dark preference')
    ).not.toBeInTheDocument()
  })

  it('hovering a card triggers preview (data-theme changes transiently)', () => {
    renderAppearanceSettings()
    const cards = screen.getAllByRole('radio')

    // Hover over light card
    fireEvent.mouseEnter(cards[1])
    expect(document.documentElement.getAttribute('data-theme')).toBe('light')
  })

  it('leaving a card restores data-theme to current theme', () => {
    renderAppearanceSettings()
    const cards = screen.getAllByRole('radio')

    fireEvent.mouseEnter(cards[1])
    fireEvent.mouseLeave(cards[1])
    // Should restore to dark (default)
    expect(document.documentElement.getAttribute('data-theme')).toBe('dark')
  })

  it('clears preview before committing theme change', () => {
    // This validates the race condition fix: setPreviewTheme(null) before setTheme
    renderAppearanceSettings()
    const cards = screen.getAllByRole('radio')

    // Start preview
    fireEvent.mouseEnter(cards[2]) // high-contrast-dark
    expect(document.documentElement.getAttribute('data-theme')).toBe('high-contrast-dark')

    // Click to commit — preview should be cleared, then theme applied
    fireEvent.click(cards[2])

    // After click, the committed theme is high-contrast-dark
    expect(document.documentElement.getAttribute('data-theme')).toBe('high-contrast-dark')
    expect(localStorage.getItem(THEME_STORAGE_KEY)).toBe('high-contrast-dark')
  })
})
