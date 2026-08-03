import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'

import '@testing-library/jest-dom/vitest'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'

import * as settingsApi from '../../api/settings'
import { ThemeProvider } from '../../context/ThemeContext'
import { THEME_STORAGE_KEY } from '../../context/ThemeContextValue'
import { useAuth } from '../../hooks/useAuth'
import { useUserThemes } from '../../hooks/useUserThemes'
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

const mockUseUserThemes = vi.mocked(useUserThemes)
const mockUseAuth = vi.mocked(useAuth)

// Referenced inside vi.mock factories below — vi.hoisted() guarantees these
// are initialized before the (also hoisted) vi.mock calls run, regardless of
// their textual position relative to other top-level statements in this file.
const { mockRefetchUser, mockAckMutate, mockOptInMutate } = vi.hoisted(() => ({
  mockRefetchUser: vi.fn(),
  mockAckMutate: vi.fn(),
  mockOptInMutate: vi.fn(),
}))

// Mock useAuth for LogoCustomizer's admin check and the What's New toggle
vi.mock('../../hooks/useAuth', () => ({
  useAuth: vi.fn().mockReturnValue({
    user: { user_id: 1, role: 'admin', name: 'Admin', changelog_opt_out: false },
    login: vi.fn(),
    logout: vi.fn(),
    changePassword: vi.fn(),
    refetchUser: mockRefetchUser,
    isAuthenticated: true,
    isLoading: false,
  }),
}))

// Mock the changelog hooks used by the What's New toggle and revisit modal
vi.mock('../../hooks/useChangelog', () => ({
  useAckChangelog: vi.fn().mockReturnValue({ mutate: mockAckMutate }),
  useOptInChangelog: vi.fn().mockReturnValue({ mutate: mockOptInMutate }),
  useChangelogStatus: vi.fn().mockReturnValue({ data: { show_changelog: false, versions: [] }, isError: false }),
  useChangelogAll: vi.fn().mockReturnValue({ data: undefined, isError: false }),
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
        'appearance.whatsNew': "What's New Notifications",
        'appearance.whatsNewDescription': 'Get notified about new features and fixes.',
        'appearance.showWhatsNewToggle': 'Show update notifications',
        'appearance.whatsNewRevisit': "What's New",
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

  // Mutation integration tests — verify that UI interactions trigger the correct API calls
  describe('Logo mutation triggers', () => {
    it('clicking logo URL tab then saving calls saveUrlMutation (updateSetting ×2)', async () => {
      renderAppearanceSettings()

      // Switch to URL tab — key is "appearance.logoUrlTab" (not in local mock → returns key)
      const urlTab = screen.getByText('appearance.logoUrlTab')
      await userEvent.click(urlTab)

      // Type a valid https:// URL in the logo URL input
      const urlInput = screen.getByPlaceholderText('appearance.logoUrlPlaceholder')
      await userEvent.type(urlInput, 'https://example.com/logo.png')

      await userEvent.click(screen.getByText('appearance.logoSaveButton'))

      await waitFor(() =>
        expect(vi.mocked(settingsApi.updateSetting)).toHaveBeenCalledWith(
          'ui.logo_url', 'https://example.com/logo.png', 'ui', 'string'
        )
      )
      expect(vi.mocked(settingsApi.updateSetting)).toHaveBeenCalledWith(
        'ui.logo_type', 'url', 'ui', 'string'
      )
    })

    it('clicking logo Reset calls deleteLogo', async () => {
      renderAppearanceSettings()

      await userEvent.click(screen.getByText('appearance.logoResetButton'))

      await waitFor(() => expect(vi.mocked(settingsApi.deleteLogo)).toHaveBeenCalled())
    })
  })

  describe('Banner mutation triggers', () => {
    it('clicking banner URL tab then saving calls saveBannerUrlMutation (updateSetting ×2)', async () => {
      renderAppearanceSettings()

      const urlTab = screen.getByText('appearance.bannerUrlTab')
      await userEvent.click(urlTab)

      const urlInput = screen.getByPlaceholderText('appearance.bannerUrlPlaceholder')
      await userEvent.type(urlInput, 'https://example.com/banner.jpg')

      await userEvent.click(screen.getByText('appearance.bannerSaveButton'))

      await waitFor(() =>
        expect(vi.mocked(settingsApi.updateSetting)).toHaveBeenCalledWith(
          'ui.banner_url', 'https://example.com/banner.jpg', 'ui', 'string'
        )
      )
      expect(vi.mocked(settingsApi.updateSetting)).toHaveBeenCalledWith(
        'ui.banner_type', 'url', 'ui', 'string'
      )
    })

    it('clicking banner Reset calls deleteBanner', async () => {
      renderAppearanceSettings()

      await userEvent.click(screen.getByText('appearance.bannerResetButton'))

      await waitFor(() => expect(vi.mocked(settingsApi.deleteBanner)).toHaveBeenCalled())
    })
  })

  describe('Custom color change', () => {
    it('changing a color input in custom mode calls setCustomTheme', () => {
      localStorage.setItem('charon-theme', 'custom')
      renderAppearanceSettings()

      // eslint-disable-next-line testing-library/no-node-access
      const colorInputs = document.querySelectorAll('input[type="color"]')
      expect(colorInputs.length).toBeGreaterThan(0)

      // Fire change on first color input → triggers handleCustomColorsChange
      fireEvent.change(colorInputs[0], { target: { value: '#ff0000' } })

      // The data-theme should still be 'custom' after color change
      expect(document.documentElement.getAttribute('data-theme')).toBe('custom')
    })
  })

  describe('UserThemeManager onActivate', () => {
    it('activating a user theme calls setUserTheme and clears preview', async () => {
      const sampleUserTheme = {
        id: 'abc',
        name: 'My Theme',
        colors: {
          bgBase: '15 23 42', bgSubtle: '30 41 59', bgMuted: '51 65 85', bgElevated: '30 41 59',
          borderDefault: '51 65 85', borderStrong: '71 85 105', textPrimary: '248 250 252',
          textSecondary: '203 213 225', textMuted: '148 163 184', brandPrimary: '59 130 246',
          colorScheme: 'dark' as const,
        },
        created_at: '2026-06-01T00:00:00Z',
        updated_at: '2026-06-01T00:00:00Z',
      }
      mockUseUserThemes.mockReturnValue({
        userThemes: [sampleUserTheme],
        isLoading: false,
        error: null,
        createTheme: vi.fn(),
        updateTheme: vi.fn(),
        deleteTheme: vi.fn(),
        isCreating: false,
        isUpdating: false,
        isDeleting: false,
      })

      renderAppearanceSettings()

      // The aria-label uses the translation KEY (not English text) because the local
      // mock doesn't include appearance.activateTheme — so the label is "appearance.activateTheme My Theme"
      const activateBtn = screen.getByRole('button', {
        name: (name) =>
          name.includes('My Theme') &&
          !name.toLowerCase().includes('edit') &&
          !name.toLowerCase().includes('delete'),
      })
      await userEvent.click(activateBtn)

      // After activating, theme should be user:abc and data-theme should be 'custom'
      expect(document.documentElement.getAttribute('data-theme')).toBe('custom')
      expect(localStorage.getItem('charon-theme')).toBe('user:abc')
    })
  })

  describe("What's New Notifications section", () => {
    beforeEach(() => {
      mockAckMutate.mockClear()
      mockOptInMutate.mockClear()
      mockRefetchUser.mockClear()
      mockUseAuth.mockReturnValue({
        user: { user_id: 1, role: 'admin', name: 'Admin', changelog_opt_out: false },
        login: vi.fn(),
        logout: vi.fn(),
        changePassword: vi.fn(),
        refetchUser: mockRefetchUser,
        isAuthenticated: true,
        isLoading: false,
      })
    })

    it('renders the section title and description', () => {
      renderAppearanceSettings()
      expect(screen.getByText("What's New Notifications")).toBeInTheDocument()
      expect(screen.getByText('Get notified about new features and fixes.')).toBeInTheDocument()
    })

    it('toggle is checked when the user has not opted out', () => {
      renderAppearanceSettings()
      const toggle = screen.getByRole('checkbox', { name: 'Show update notifications' })
      expect(toggle).toHaveAttribute('data-state', 'checked')
    })

    it('toggle is unchecked when the user has opted out', () => {
      mockUseAuth.mockReturnValue({
        user: { user_id: 1, role: 'admin', name: 'Admin', changelog_opt_out: true },
        login: vi.fn(),
        logout: vi.fn(),
        changePassword: vi.fn(),
        refetchUser: mockRefetchUser,
        isAuthenticated: true,
        isLoading: false,
      })
      renderAppearanceSettings()
      const toggle = screen.getByRole('checkbox', { name: 'Show update notifications' })
      expect(toggle).toHaveAttribute('data-state', 'unchecked')
    })

    it('turning the toggle off calls ack with dismiss_temporary + opt_out true, then refetches the user', async () => {
      const user = userEvent.setup()
      renderAppearanceSettings()

      await user.click(screen.getByRole('checkbox', { name: 'Show update notifications' }))

      expect(mockAckMutate).toHaveBeenCalledWith(
        { action: 'dismiss_temporary', opt_out: true },
        expect.objectContaining({ onSuccess: expect.any(Function) })
      )
      expect(mockOptInMutate).not.toHaveBeenCalled()

      // Simulate the mutation's onSuccess firing to verify the refetch wiring
      const options = mockAckMutate.mock.calls[0][1]
      options.onSuccess()
      expect(mockRefetchUser).toHaveBeenCalledTimes(1)
    })

    it('turning the toggle on calls opt-in, then refetches the user', async () => {
      mockUseAuth.mockReturnValue({
        user: { user_id: 1, role: 'admin', name: 'Admin', changelog_opt_out: true },
        login: vi.fn(),
        logout: vi.fn(),
        changePassword: vi.fn(),
        refetchUser: mockRefetchUser,
        isAuthenticated: true,
        isLoading: false,
      })
      const user = userEvent.setup()
      renderAppearanceSettings()

      await user.click(screen.getByRole('checkbox', { name: 'Show update notifications' }))

      expect(mockOptInMutate).toHaveBeenCalledWith(
        undefined,
        expect.objectContaining({ onSuccess: expect.any(Function) })
      )
      expect(mockAckMutate).not.toHaveBeenCalled()

      const options = mockOptInMutate.mock.calls[0][1]
      options.onSuccess()
      expect(mockRefetchUser).toHaveBeenCalledTimes(1)
    })

    it('clicking the revisit link opens the browse-mode What\'s New modal', async () => {
      const user = userEvent.setup()
      renderAppearanceSettings()

      expect(screen.queryByText('whatsNew.title')).not.toBeInTheDocument()

      await user.click(screen.getByRole('button', { name: "What's New" }))

      // WhatsNewModal is not mocked (only its hooks are), so its real
      // DialogTitle renders using the un-mocked i18next key for this test file.
      expect(screen.getByText('whatsNew.title')).toBeInTheDocument()
    })

    it('closing the browse-mode modal (Close button) hides it again', async () => {
      const user = userEvent.setup()
      renderAppearanceSettings()

      await user.click(screen.getByRole('button', { name: "What's New" }))
      expect(screen.getByText('whatsNew.title')).toBeInTheDocument()

      await user.click(screen.getByRole('button', { name: 'whatsNew.closeButton' }))

      expect(screen.queryByText('whatsNew.title')).not.toBeInTheDocument()
      expect(mockAckMutate).not.toHaveBeenCalled()
    })
  })
})
