import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { MemoryRouter } from 'react-router-dom'
import { vi, describe, it, expect, beforeEach } from 'vitest'
import SystemSettings from '../SystemSettings'
import * as settingsApi from '../../api/settings'
import * as featureFlagsApi from '../../api/featureFlags'
import client from '../../api/client'
import { LanguageProvider } from '../../context/LanguageContext'

// Mock i18next
vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string) => key,
    i18n: {
      changeLanguage: vi.fn(),
      language: 'en',
    },
  }),
}))

// Mock API modules
vi.mock('../../api/settings', () => ({
  getSettings: vi.fn(),
  updateSetting: vi.fn(),
}))

vi.mock('../../api/featureFlags', () => ({
  getFeatureFlags: vi.fn(),
  updateFeatureFlags: vi.fn(),
}))

vi.mock('../../api/client', () => ({
  default: {
    get: vi.fn(),
  },
}))

const createQueryClient = () =>
  new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false },
    },
  })

const renderWithProviders = (ui: React.ReactNode) => {
  const queryClient = createQueryClient()
  return render(
    <QueryClientProvider client={queryClient}>
      <LanguageProvider>
        <MemoryRouter>{ui}</MemoryRouter>
      </LanguageProvider>
    </QueryClientProvider>
  )
}

describe('SystemSettings', () => {
  beforeEach(() => {
    vi.clearAllMocks()

    // Default mock responses
    vi.mocked(settingsApi.getSettings).mockResolvedValue({
      'caddy.admin_api': 'http://localhost:2019',
      'caddy.ssl_provider': 'auto',
      'ui.domain_link_behavior': 'new_tab',
      'security.cerberus.enabled': 'false',
    })

    vi.mocked(featureFlagsApi.getFeatureFlags).mockResolvedValue({
      'feature.cerberus.enabled': false,
      'feature.crowdsec.console_enrollment': false,
      'feature.uptime.enabled': false,
    })

    vi.mocked(client.get).mockResolvedValue({
      data: {
        status: 'healthy',
        service: 'charon',
        version: '0.1.0',
        git_commit: 'abc123',
        build_time: '2025-01-01T00:00:00Z',
      },
    })
  })

  describe('SSL Provider Selection', () => {
    it('renders SSL Provider label', async () => {
      renderWithProviders(<SystemSettings />)

      await waitFor(() => {
        expect(screen.getByText('SSL Provider')).toBeTruthy()
      })
    })

    it('displays the correct help text for SSL provider', async () => {
      renderWithProviders(<SystemSettings />)

      await waitFor(() => {
        expect(screen.getByText(/Choose the Certificate Authority/i)).toBeTruthy()
      })
    })

    it('renders the SSL provider select trigger', async () => {
      renderWithProviders(<SystemSettings />)

      await waitFor(() => {
        expect(screen.getByText('SSL Provider')).toBeTruthy()
      })

      // Radix UI Select uses a button as the trigger
      const selectTrigger = screen.getByRole('combobox', { name: /ssl provider/i })
      expect(selectTrigger).toBeTruthy()
    })

    it('displays Auto as default selection', async () => {
      renderWithProviders(<SystemSettings />)

      await waitFor(() => {
        expect(screen.getByText('Auto (Recommended)')).toBeTruthy()
      })
    })

    it('saves SSL provider setting when save button is clicked', async () => {
      vi.mocked(settingsApi.updateSetting).mockResolvedValue(undefined)

      renderWithProviders(<SystemSettings />)

      await waitFor(() => {
        expect(screen.getByText('SSL Provider')).toBeTruthy()
      })

      const user = userEvent.setup()
      const saveButton = screen.getByRole('button', { name: /Save Settings/i })
      await user.click(saveButton)

      await waitFor(() => {
        expect(settingsApi.updateSetting).toHaveBeenCalledWith(
          'caddy.ssl_provider',
          expect.any(String),
          'caddy',
          'string'
        )
      })
    })
  })

  describe('General Settings', () => {
    it('renders the page title', async () => {
      renderWithProviders(<SystemSettings />)

      await waitFor(() => {
        expect(screen.getByText('System Settings')).toBeTruthy()
      })
    })

    it('loads and displays Caddy Admin API setting', async () => {
      vi.mocked(settingsApi.getSettings).mockResolvedValue({
        'caddy.admin_api': 'http://custom:2019',
      })

      renderWithProviders(<SystemSettings />)

      await waitFor(() => {
        const input = screen.getByPlaceholderText('http://localhost:2019') as HTMLInputElement
        expect(input.value).toBe('http://custom:2019')
      })
    })

    it('saves all settings when save button is clicked', async () => {
      vi.mocked(settingsApi.updateSetting).mockResolvedValue(undefined)

      renderWithProviders(<SystemSettings />)

      await waitFor(() => {
        expect(screen.getByText('Save Settings')).toBeTruthy()
      })

      const user = userEvent.setup()
      const saveButton = screen.getByRole('button', { name: /Save Settings/i })
      await user.click(saveButton)

      await waitFor(() => {
        expect(settingsApi.updateSetting).toHaveBeenCalledTimes(3)
        expect(settingsApi.updateSetting).toHaveBeenCalledWith(
          'caddy.admin_api',
          expect.any(String),
          'caddy',
          'string'
        )
        expect(settingsApi.updateSetting).toHaveBeenCalledWith(
          'caddy.ssl_provider',
          expect.any(String),
          'caddy',
          'string'
        )
        expect(settingsApi.updateSetting).toHaveBeenCalledWith(
          'ui.domain_link_behavior',
          expect.any(String),
          'ui',
          'string'
        )
      })
    })
  })

  describe('System Status', () => {
    it('displays system health information', async () => {
      vi.mocked(client.get).mockResolvedValue({
        data: {
          status: 'healthy',
          service: 'charon',
          version: '1.0.0',
          git_commit: 'abc123def',
          build_time: '2025-12-06T00:00:00Z',
        },
      })

      renderWithProviders(<SystemSettings />)

      await waitFor(() => {
        expect(screen.getByText('charon')).toBeTruthy()
        expect(screen.getByText('1.0.0')).toBeTruthy()
        expect(screen.getByText('abc123def')).toBeTruthy()
      })
    })

    it('displays System Status section', async () => {
      renderWithProviders(<SystemSettings />)

      await waitFor(() => {
        expect(screen.getByText('System Status')).toBeTruthy()
      })
    })
  })

  describe('Features', () => {
    it('renders the Features section', async () => {
      renderWithProviders(<SystemSettings />)

      await waitFor(() => {
        expect(screen.getByText('Features')).toBeTruthy()
      })
    })

    it('displays all feature flag toggles', async () => {
      vi.mocked(featureFlagsApi.getFeatureFlags).mockResolvedValue({
        'feature.cerberus.enabled': false,
        'feature.crowdsec.console_enrollment': false,
        'feature.uptime.enabled': false,
      })

      renderWithProviders(<SystemSettings />)

      await waitFor(() => {
        expect(screen.getByText('Cerberus Security Suite')).toBeTruthy()
        expect(screen.getByText('CrowdSec Console Enrollment')).toBeTruthy()
        expect(screen.getByText('Uptime Monitoring')).toBeTruthy()
      })
    })

    it('shows Cerberus toggle as checked when enabled', async () => {
      vi.mocked(featureFlagsApi.getFeatureFlags).mockResolvedValue({
        'feature.cerberus.enabled': true,
        'feature.crowdsec.console_enrollment': false,
        'feature.uptime.enabled': false,
      })

      renderWithProviders(<SystemSettings />)

      await waitFor(() => {
        expect(screen.getByText('Cerberus Security Suite')).toBeTruthy()
      })

      const switchInput = screen.getByRole('checkbox', { name: /cerberus security suite toggle/i })
      expect(switchInput).toBeChecked()
    })

    it('shows Uptime toggle as checked when enabled', async () => {
      vi.mocked(featureFlagsApi.getFeatureFlags).mockResolvedValue({
        'feature.uptime.enabled': true,
        'feature.cerberus.enabled': false,
        'feature.crowdsec.console_enrollment': false,
      })

      renderWithProviders(<SystemSettings />)

      await waitFor(() => {
        expect(screen.getByText('Uptime Monitoring')).toBeTruthy()
      })

      const switchInput = screen.getByRole('checkbox', { name: /uptime monitoring toggle/i })
      expect(switchInput).toBeChecked()
    })

    it('shows Cerberus toggle as unchecked when disabled', async () => {
      vi.mocked(featureFlagsApi.getFeatureFlags).mockResolvedValue({
        'feature.cerberus.enabled': false,
        'feature.crowdsec.console_enrollment': false,
        'feature.uptime.enabled': false,
      })

      renderWithProviders(<SystemSettings />)

      await waitFor(() => {
        expect(screen.getByText('Cerberus Security Suite')).toBeTruthy()
      })

      const switchInput = screen.getByRole('checkbox', { name: /cerberus security suite toggle/i })
      expect(switchInput).not.toBeChecked()
    })

    it('toggles Cerberus feature flag when switch is clicked', async () => {
      vi.mocked(featureFlagsApi.getFeatureFlags).mockResolvedValue({
        'feature.cerberus.enabled': false,
        'feature.crowdsec.console_enrollment': false,
        'feature.uptime.enabled': false,
      })
      vi.mocked(featureFlagsApi.updateFeatureFlags).mockResolvedValue(undefined)

      renderWithProviders(<SystemSettings />)

      await waitFor(() => {
        expect(screen.getByText('Cerberus Security Suite')).toBeTruthy()
      })

      const user = userEvent.setup()
      const switchInput = screen.getByRole('checkbox', { name: /cerberus security suite toggle/i })

      await user.click(switchInput)

      await waitFor(() => {
        expect(featureFlagsApi.updateFeatureFlags).toHaveBeenCalledWith({
          'feature.cerberus.enabled': true,
        })
      })
    })

    it('toggles CrowdSec Console Enrollment feature flag when switch is clicked', async () => {
      vi.mocked(featureFlagsApi.getFeatureFlags).mockResolvedValue({
        'feature.cerberus.enabled': false,
        'feature.crowdsec.console_enrollment': false,
        'feature.uptime.enabled': false,
      })
      vi.mocked(featureFlagsApi.updateFeatureFlags).mockResolvedValue(undefined)

      renderWithProviders(<SystemSettings />)

      await waitFor(() => {
        expect(screen.getByText('CrowdSec Console Enrollment')).toBeTruthy()
      })

      const user = userEvent.setup()
      const switchInput = screen.getByRole('checkbox', { name: /crowdsec console enrollment toggle/i })

      await user.click(switchInput)

      await waitFor(() => {
        expect(featureFlagsApi.updateFeatureFlags).toHaveBeenCalledWith({
          'feature.crowdsec.console_enrollment': true,
        })
      })
    })

    it('toggles Uptime feature flag when switch is clicked', async () => {
      vi.mocked(featureFlagsApi.getFeatureFlags).mockResolvedValue({
        'feature.uptime.enabled': true,
        'feature.cerberus.enabled': false,
        'feature.crowdsec.console_enrollment': false,
      })
      vi.mocked(featureFlagsApi.updateFeatureFlags).mockResolvedValue(undefined)

      renderWithProviders(<SystemSettings />)

      await waitFor(() => {
        expect(screen.getByText('Uptime Monitoring')).toBeTruthy()
      })

      const user = userEvent.setup()
      const switchInput = screen.getByRole('checkbox', { name: /uptime monitoring toggle/i })

      await user.click(switchInput)

      await waitFor(() => {
        expect(featureFlagsApi.updateFeatureFlags).toHaveBeenCalledWith({
          'feature.uptime.enabled': false,
        })
      })
    })

    it('shows loading skeleton when feature flags are not loaded', async () => {
      // Set settings to resolve but feature flags to never resolve (pending state)
      vi.mocked(settingsApi.getSettings).mockResolvedValue({
        'caddy.admin_api': 'http://localhost:2019',
        'caddy.ssl_provider': 'auto',
        'ui.domain_link_behavior': 'new_tab',
      })
      vi.mocked(featureFlagsApi.getFeatureFlags).mockReturnValue(new Promise(() => {}))

      renderWithProviders(<SystemSettings />)

      // When featureFlags is undefined but settings is loaded, it shows skeleton in the Features card
      await waitFor(() => {
        expect(screen.getByText('Features')).toBeTruthy()
      })

      // Verify skeleton elements are rendered (Skeleton component uses animate-pulse class)
      const skeletons = document.querySelectorAll('.animate-pulse')
      expect(skeletons.length).toBeGreaterThan(0)
    })

    it('shows loading overlay while toggling a feature flag', async () => {
      vi.mocked(featureFlagsApi.getFeatureFlags).mockResolvedValue({
        'feature.cerberus.enabled': false,
        'feature.crowdsec.console_enrollment': false,
        'feature.uptime.enabled': false,
      })
      vi.mocked(featureFlagsApi.updateFeatureFlags).mockImplementation(
        () => new Promise(() => {})
      )

      renderWithProviders(<SystemSettings />)

      await waitFor(() => {
        expect(screen.getByText('Cerberus Security Suite')).toBeTruthy()
      })

      const user = userEvent.setup()
      const switchInput = screen.getByRole('checkbox', { name: /cerberus security suite toggle/i })

      await user.click(switchInput)

      await waitFor(() => {
        expect(screen.getByText('Updating features...')).toBeInTheDocument()
      })
    })
  })
})
