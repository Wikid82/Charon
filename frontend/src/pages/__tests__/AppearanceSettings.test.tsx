import { render, screen, fireEvent } from '@testing-library/react'
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'

import '@testing-library/jest-dom/vitest'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { ThemeProvider } from '../../context/ThemeContext'
import { THEME_STORAGE_KEY } from '../../context/ThemeContextValue'
import AppearanceSettings from '../AppearanceSettings'

// ThemeProvider calls useUserThemes internally; stub it so tests don't need a real API server
vi.mock('../../hooks/useUserThemes', () => ({
  useUserThemes: vi.fn().mockReturnValue({
    userThemes: [],
    isLoading: false,
    error: null,
    createTheme: vi.fn(),
    updateTheme: vi.fn(),
    deleteTheme: vi.fn(),
    isCreating: false,
    isUpdating: false,
    isDeleting: false,
  }),
}))

// Mock the settings API so useQuery does not fail without a real server
vi.mock('../../api/settings', () => ({
  getSettings: vi.fn().mockResolvedValue({}),
  updateSetting: vi.fn().mockResolvedValue(undefined),
  uploadLogo: vi.fn().mockResolvedValue({ url: '/uploads/logo.png' }),
  deleteLogo: vi.fn().mockResolvedValue(undefined),
  uploadBanner: vi.fn().mockResolvedValue({ url: '/uploads/banner.png' }),
  deleteBanner: vi.fn().mockResolvedValue(undefined),
  validatePublicURL: vi.fn(),
  testPublicURL: vi.fn(),
}))

// Mock useAuth for LogoCustomizer's admin check
vi.mock('../../hooks/useAuth', () => ({
  useAuth: vi.fn().mockReturnValue({
    user: { user_id: 1, role: 'admin', name: 'Admin' },
    login: vi.fn(),
    logout: vi.fn(),
    changePassword: vi.fn(),
    isAuthenticated: true,
    isLoading: false,
  }),
}))

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
        'appearance.customThemeDescription': 'Fine-tune colors to create your own theme',
        'appearance.colorPickerBgBase': 'Background',
        'appearance.colorPickerBgSubtle': 'Subtle Background',
        'appearance.colorPickerBgMuted': 'Muted Background',
        'appearance.colorPickerBgElevated': 'Elevated Surface',
        'appearance.colorPickerBorderDefault': 'Border',
        'appearance.colorPickerBorderStrong': 'Strong Border',
        'appearance.colorPickerTextPrimary': 'Primary Text',
        'appearance.colorPickerTextSecondary': 'Secondary Text',
        'appearance.colorPickerTextMuted': 'Muted Text',
        'appearance.colorPickerBrandPrimary': 'Accent Color',
        'appearance.colorPickerColorScheme': 'Base Scheme',
        'appearance.importExport': 'Import / Export',
        'appearance.importExportDescription': 'Share themes between Charon instances',
        'appearance.exportButton': 'Export Theme',
        'appearance.importButton': 'Import Theme',
        'appearance.importError': 'Invalid theme file',
        'common.enabled': 'Enabled',
      }
      return map[key] ?? key
    },
  }),
}))

vi.mock('../../utils/toast', () => ({
  toast: {
    error: vi.fn(),
    success: vi.fn(),
    info: vi.fn(),
    warning: vi.fn(),
  },
}))

function renderAppearanceSettings() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  })
  return render(
    <QueryClientProvider client={queryClient}>
      <ThemeProvider>
        <AppearanceSettings />
      </ThemeProvider>
    </QueryClientProvider>
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

  // Custom picker visibility tests
  describe('Custom Theme section visibility', () => {
    it('does not show custom picker section when theme is "dark" (not custom)', () => {
      renderAppearanceSettings()
      expect(screen.queryByText('Custom Theme')).not.toBeInTheDocument()
    })

    it('does not show custom picker section when theme is "light"', () => {
      renderAppearanceSettings()
      const cards = screen.getAllByRole('radio')
      fireEvent.click(cards[1]) // light
      expect(screen.queryByText('Custom Theme')).not.toBeInTheDocument()
    })

    it('does not show custom picker section when theme is "system"', () => {
      renderAppearanceSettings()
      const cards = screen.getAllByRole('radio')
      fireEvent.click(cards[5]) // system
      expect(screen.queryByText('Custom Theme')).not.toBeInTheDocument()
    })

    it('shows custom picker section when localStorage has custom theme', () => {
      localStorage.setItem(THEME_STORAGE_KEY, 'custom')
      renderAppearanceSettings()
      expect(screen.getByText('Custom Theme')).toBeInTheDocument()
    })

    it('shows color inputs when custom theme is selected', () => {
      localStorage.setItem(THEME_STORAGE_KEY, 'custom')
      renderAppearanceSettings()
      // type="color" inputs are not accessible by role — use DOM query
      // eslint-disable-next-line testing-library/no-node-access
      const colorInputs = document.querySelectorAll('input[type="color"]')
      expect(colorInputs).toHaveLength(10)
    })

    it('shows color scheme select when custom theme is selected', () => {
      localStorage.setItem(THEME_STORAGE_KEY, 'custom')
      renderAppearanceSettings()
      expect(screen.getByLabelText('Base Scheme')).toBeInTheDocument()
    })
  })

  // Import/Export section visibility tests
  describe('Import/Export section', () => {
    it('always shows the import/export section', () => {
      renderAppearanceSettings()
      expect(screen.getByText('Import / Export')).toBeInTheDocument()
    })

    it('always shows export button', () => {
      renderAppearanceSettings()
      expect(screen.getByRole('button', { name: /export theme/i })).toBeInTheDocument()
    })

    it('always shows import button', () => {
      renderAppearanceSettings()
      expect(screen.getByRole('button', { name: /import theme/i })).toBeInTheDocument()
    })

    it('shows import/export section when theme is light', () => {
      renderAppearanceSettings()
      const cards = screen.getAllByRole('radio')
      fireEvent.click(cards[1]) // light
      expect(screen.getByText('Import / Export')).toBeInTheDocument()
    })

    it('shows import/export section when theme is custom', () => {
      localStorage.setItem(THEME_STORAGE_KEY, 'custom')
      renderAppearanceSettings()
      expect(screen.getByText('Import / Export')).toBeInTheDocument()
    })

    it('shows import/export section description', () => {
      renderAppearanceSettings()
      expect(screen.getByText('Share themes between Charon instances')).toBeInTheDocument()
    })
  })
})
