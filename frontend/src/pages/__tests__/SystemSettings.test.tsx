import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router-dom'
import { vi, describe, it, expect, beforeEach } from 'vitest'

import client from '../../api/client'
import * as featureFlagsApi from '../../api/featureFlags'
import * as settingsApi from '../../api/settings'
import { LanguageProvider } from '../../context/LanguageContext'
import SystemSettings from '../SystemSettings'

// Note: react-i18next mock is provided globally by src/test/setup.ts

// Mock API modules
vi.mock('../../api/settings', () => ({
  getSettings: vi.fn(),
  updateSetting: vi.fn(),
  validatePublicURL: vi.fn(),
  testPublicURL: vi.fn(),
}))

vi.mock('../../api/featureFlags', () => ({
  getFeatureFlags: vi.fn(),
  updateFeatureFlags: vi.fn(),
}))

vi.mock('../../api/client', () => ({
  default: {
    get: vi.fn(),
    post: vi.fn(),
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
      'caddy.keepalive_idle': '',
      'caddy.keepalive_count': '',
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
      vi.mocked(settingsApi.updateSetting).mockResolvedValue()

      renderWithProviders(<SystemSettings />)

      await waitFor(() => {
        expect(screen.getByText('SSL Provider')).toBeTruthy()
      })

      const user = userEvent.setup()
      const saveButtons = screen.getAllByRole('button', { name: /Save Settings/i })
      await user.click(saveButtons[0])

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

    it('loads keepalive settings when present', async () => {
      vi.mocked(settingsApi.getSettings).mockResolvedValue({
        'caddy.admin_api': 'http://localhost:2019',
        'caddy.ssl_provider': 'auto',
        'caddy.keepalive_idle': '2m',
        'caddy.keepalive_count': '5',
        'ui.domain_link_behavior': 'new_tab',
      })

      renderWithProviders(<SystemSettings />)

      await waitFor(() => {
        const keepaliveIdleInput = screen.getByLabelText('Keepalive Idle (Optional)') as HTMLInputElement
        const keepaliveCountInput = screen.getByLabelText('Keepalive Count (Optional)') as HTMLInputElement
        expect(keepaliveIdleInput.value).toBe('2m')
        expect(keepaliveCountInput.value).toBe('5')
      })
    })

    it('renders keepalive controls in General settings', async () => {
      renderWithProviders(<SystemSettings />)

      await waitFor(() => {
        expect(screen.getByLabelText('Keepalive Idle (Optional)')).toBeInTheDocument()
        expect(screen.getByLabelText('Keepalive Count (Optional)')).toBeInTheDocument()
      })
    })

    it('saves all settings when save button is clicked', async () => {
      vi.mocked(settingsApi.updateSetting).mockResolvedValue()

      renderWithProviders(<SystemSettings />)

      await waitFor(() => {
        expect(screen.getAllByText('Save Settings')).toHaveLength(2)
      })

      const user = userEvent.setup()
      const saveButtons = screen.getAllByRole('button', { name: /Save Settings/i })
      await user.click(saveButtons[0])

      await waitFor(() => {
        expect(settingsApi.updateSetting).toHaveBeenCalledTimes(6)
      })
      expect(settingsApi.updateSetting).toHaveBeenCalledWith(
          'ui.domain_link_behavior',
          expect.any(String),
          'ui',
          'string'
        )
      expect(settingsApi.updateSetting).toHaveBeenCalledWith(
          'caddy.keepalive_count',
          '',
          'caddy',
          'string'
        )
      expect(settingsApi.updateSetting).toHaveBeenCalledWith(
          'caddy.keepalive_idle',
          '',
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
          'caddy.admin_api',
          expect.any(String),
          'caddy',
          'string'
        )
    })

    it('saves keepalive settings when valid values are provided', async () => {
      vi.mocked(settingsApi.updateSetting).mockResolvedValue()

      renderWithProviders(<SystemSettings />)

      await waitFor(() => {
        expect(screen.getByLabelText('Keepalive Idle (Optional)')).toBeInTheDocument()
      })

      const user = userEvent.setup()
      const keepaliveIdleInput = screen.getByLabelText('Keepalive Idle (Optional)')
      const keepaliveCountInput = screen.getByLabelText('Keepalive Count (Optional)')
      await user.clear(keepaliveIdleInput)
      await user.type(keepaliveIdleInput, '30s')
      await user.clear(keepaliveCountInput)
      await user.type(keepaliveCountInput, '3')

      const saveButtons = screen.getAllByRole('button', { name: /Save Settings/i })
      await user.click(saveButtons[0])

      await waitFor(() => {
        expect(settingsApi.updateSetting).toHaveBeenCalledWith(
          'caddy.keepalive_idle',
          '30s',
          'caddy',
          'string'
        )
      })
      expect(settingsApi.updateSetting).toHaveBeenCalledWith(
          'caddy.keepalive_count',
          '3',
          'caddy',
          'string'
        )
    })

    it('disables save when keepalive values are invalid', async () => {
      renderWithProviders(<SystemSettings />)

      await waitFor(() => {
        expect(screen.getByLabelText('Keepalive Idle (Optional)')).toBeInTheDocument()
      })

      const user = userEvent.setup()
      const keepaliveIdleInput = screen.getByLabelText('Keepalive Idle (Optional)')
      await user.clear(keepaliveIdleInput)
      await user.type(keepaliveIdleInput, 'invalid-duration')

      await waitFor(() => {
        expect(screen.getByText('Enter a valid duration (for example: 30s, 2m, 1h).')).toBeInTheDocument()
      })

      const saveButtons = screen.getAllByRole('button', { name: /Save Settings/i })
      expect(saveButtons[0]).toBeDisabled()
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

  describe('Application URL Card', () => {
    it('renders public URL input field', async () => {
      renderWithProviders(<SystemSettings />)

      await waitFor(() => {
        expect(screen.getByPlaceholderText('https://charon.example.com')).toBeTruthy()
      })
    })

    it('shows green border and checkmark when URL is valid', async () => {
      vi.mocked(client.get).mockResolvedValue({
        data: { status: 'healthy', service: 'charon', version: '1.0.0' },
      })

      renderWithProviders(<SystemSettings />)

      await waitFor(() => {
        expect(screen.getByPlaceholderText('https://charon.example.com')).toBeTruthy()
      })

      const user = userEvent.setup()
      const input = screen.getByPlaceholderText('https://charon.example.com')

      // Mock validation response for valid URL
      vi.mocked(client.post).mockResolvedValue({
        data: { valid: true, normalized: 'https://example.com' },
      })

      await user.clear(input)
      await user.type(input, 'https://example.com')

      // Wait for debounced validation
      await waitFor(() => {
        expect(client.post).toHaveBeenCalledWith('/settings/validate-url', {
          url: 'https://example.com',
        })
      }, { timeout: 1000 })

      await waitFor(() => {
        const checkIcon = document.querySelector('.text-green-500')
        expect(checkIcon).toBeTruthy()
      })

      await waitFor(() => {
        expect(input.className).toContain('border-green-500')
      })
    })

    it('shows red border and X icon when URL is invalid', async () => {
      vi.mocked(client.get).mockResolvedValue({
        data: { status: 'healthy', service: 'charon', version: '1.0.0' },
      })

      renderWithProviders(<SystemSettings />)

      await waitFor(() => {
        expect(screen.getByPlaceholderText('https://charon.example.com')).toBeTruthy()
      })

      const user = userEvent.setup()
      const input = screen.getByPlaceholderText('https://charon.example.com')

      // Mock validation response for invalid URL
      vi.mocked(client.post).mockResolvedValue({
        data: { valid: false, error: 'Invalid URL format' },
      })

      await user.clear(input)
      await user.type(input, 'invalid-url')

      await waitFor(() => {
        expect(client.post).toHaveBeenCalledWith('/settings/validate-url', {
          url: 'invalid-url',
        })
      }, { timeout: 1000 })

      await waitFor(() => {
        const xIcon = document.querySelector('.text-red-500')
        expect(xIcon).toBeTruthy()
      })

      await waitFor(() => {
        expect(input.className).toContain('border-red-500')
      })
    })

    it('shows invalid URL error message when validation fails', async () => {
      vi.mocked(client.get).mockResolvedValue({
        data: { status: 'healthy', service: 'charon', version: '1.0.0' },
      })

      renderWithProviders(<SystemSettings />)

      await waitFor(() => {
        expect(screen.getByPlaceholderText('https://charon.example.com')).toBeTruthy()
      })

      const user = userEvent.setup()
      const input = screen.getByPlaceholderText('https://charon.example.com')

      vi.mocked(client.post).mockResolvedValue({
        data: { valid: false, error: 'Invalid URL format' },
      })

      await user.clear(input)
      await user.type(input, 'bad-url')

      // Wait for debounce and validation
      await new Promise(resolve => setTimeout(resolve, 400))

      await waitFor(() => {
        // Check for red border class indicating invalid state
        const inputElement = screen.getByPlaceholderText('https://charon.example.com')
        expect(inputElement.className).toContain('border-red')
      }, { timeout: 1000 })
    })

    it('clears validation state when URL is cleared', async () => {
      vi.mocked(settingsApi.getSettings).mockResolvedValue({
        'app.public_url': 'https://example.com',
      })
      vi.mocked(client.get).mockResolvedValue({
        data: { status: 'healthy', service: 'charon', version: '1.0.0' },
      })

      renderWithProviders(<SystemSettings />)

      await waitFor(() => {
        const input = screen.getByPlaceholderText('https://charon.example.com') as HTMLInputElement
        expect(input.value).toBe('https://example.com')
      })

      const user = userEvent.setup()
      const input = screen.getByPlaceholderText('https://charon.example.com')

      await user.clear(input)

      await waitFor(() => {
        expect(input.className).not.toContain('border-green-500')
        expect(input.className).not.toContain('border-red-500')
      })
    })

    it('renders test button and verifies functionality', async () => {
      vi.mocked(settingsApi.getSettings).mockResolvedValue({
        'app.public_url': 'https://example.com',
      })
      vi.mocked(settingsApi.testPublicURL).mockResolvedValue({
        reachable: true,
        latency: 42,
      })

      renderWithProviders(<SystemSettings />)

      await waitFor(() => {
        expect(screen.getByPlaceholderText('https://charon.example.com')).toBeTruthy()
      })

      // Find test button by looking for buttons with External Link icon
      const buttons = screen.getAllByRole('button')
      const testButton = buttons.find((btn) => btn.querySelector('.lucide-external-link'))
      expect(testButton).toBeTruthy()
      expect(testButton).not.toBeDisabled()

      const user = userEvent.setup()
      await user.click(testButton!)

      await waitFor(() => {
        expect(settingsApi.testPublicURL).toHaveBeenCalledWith('https://example.com')
      })
    })

    it('disables test button when URL is empty', async () => {
      vi.mocked(settingsApi.getSettings).mockResolvedValue({
        'app.public_url': '',
      })

      renderWithProviders(<SystemSettings />)

      await waitFor(() => {
        const input = screen.getByPlaceholderText('https://charon.example.com') as HTMLInputElement
        expect(input.value).toBe('')
      })

      const buttons = screen.getAllByRole('button')
      const testButton = buttons.find((btn) => btn.querySelector('.lucide-external-link'))
      expect(testButton).toBeDisabled()
    })

    it('handles validation API error gracefully', async () => {
      renderWithProviders(<SystemSettings />)

      await waitFor(() => {
        expect(screen.getByPlaceholderText('https://charon.example.com')).toBeTruthy()
      })

      const user = userEvent.setup()
      const input = screen.getByPlaceholderText('https://charon.example.com')

      vi.mocked(client.post).mockRejectedValue(new Error('Network error'))

      await user.clear(input)
      await user.type(input, 'https://example.com')

      await waitFor(() => {
        const xIcon = document.querySelector('.text-red-500')
        expect(xIcon).toBeTruthy()
      }, { timeout: 1000 })
    })
  })
})
