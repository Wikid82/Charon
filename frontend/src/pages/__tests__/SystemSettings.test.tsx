import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { MemoryRouter } from 'react-router-dom'
import { vi, describe, it, expect, beforeEach } from 'vitest'
import SystemSettings from '../SystemSettings'
import * as settingsApi from '../../api/settings'
import * as featureFlagsApi from '../../api/featureFlags'
import client from '../../api/client'

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
      <MemoryRouter>{ui}</MemoryRouter>
    </QueryClientProvider>
  )
}

// Helper to get SSL Provider select element
const getSSLProviderSelect = (): HTMLSelectElement => {
  const selects = document.querySelectorAll('select')
  const sslSelect = Array.from(selects).find(s =>
    s.querySelector('option[value="auto"]')
  ) as HTMLSelectElement
  return sslSelect
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
    it('defaults to "auto" when no setting is present', async () => {
      vi.mocked(settingsApi.getSettings).mockResolvedValue({
        'caddy.admin_api': 'http://localhost:2019',
      })

      renderWithProviders(<SystemSettings />)

      await waitFor(() => {
        expect(screen.getByText('SSL Provider')).toBeTruthy()
      })

      const sslSelect = getSSLProviderSelect()
      expect(sslSelect).toBeTruthy()
      expect(sslSelect.value).toBe('auto')
    })

    it('renders all SSL provider options correctly', async () => {
      renderWithProviders(<SystemSettings />)

      await waitFor(() => {
        expect(screen.getByText('SSL Provider')).toBeTruthy()
      })

      const select = getSSLProviderSelect()
      const options = Array.from(select.options).map(opt => ({
        value: opt.value,
        text: opt.textContent,
      }))

      expect(options).toEqual([
        { value: 'auto', text: 'Auto (Recommended)' },
        { value: 'letsencrypt-prod', text: "Let's Encrypt (Prod)" },
        { value: 'letsencrypt-staging', text: "Let's Encrypt (Staging)" },
        { value: 'zerossl', text: 'ZeroSSL' },
      ])
    })

    it('displays the correct help text for SSL provider', async () => {
      renderWithProviders(<SystemSettings />)

      await waitFor(() => {
        expect(screen.getByText("Choose the Certificate Authority. 'Auto' uses Let's Encrypt with ZeroSSL fallback. Staging is for testing.")).toBeTruthy()
      })
    })

    it('loads "auto" value from API correctly', async () => {
      vi.mocked(settingsApi.getSettings).mockResolvedValue({
        'caddy.admin_api': 'http://localhost:2019',
        'caddy.ssl_provider': 'auto',
      })

      renderWithProviders(<SystemSettings />)

      await waitFor(() => {
        expect(screen.getByText('SSL Provider')).toBeTruthy()
      })

      const select = getSSLProviderSelect()
      expect(select.value).toBe('auto')
    })

    it('loads "letsencrypt-staging" value from API correctly', async () => {
      vi.mocked(settingsApi.getSettings).mockResolvedValue({
        'caddy.admin_api': 'http://localhost:2019',
        'caddy.ssl_provider': 'letsencrypt-staging',
      })

      renderWithProviders(<SystemSettings />)

      await waitFor(() => {
        const select = getSSLProviderSelect()
        expect(select.value).toBe('letsencrypt-staging')
      })
    })

    it('loads "letsencrypt-prod" value from API correctly', async () => {
      vi.mocked(settingsApi.getSettings).mockResolvedValue({
        'caddy.admin_api': 'http://localhost:2019',
        'caddy.ssl_provider': 'letsencrypt-prod',
      })

      renderWithProviders(<SystemSettings />)

      await waitFor(() => {
        const select = getSSLProviderSelect()
        expect(select.value).toBe('letsencrypt-prod')
      })
    })

    it('loads "zerossl" value from API correctly', async () => {
      vi.mocked(settingsApi.getSettings).mockResolvedValue({
        'caddy.admin_api': 'http://localhost:2019',
        'caddy.ssl_provider': 'zerossl',
      })

      renderWithProviders(<SystemSettings />)

      await waitFor(() => {
        const select = getSSLProviderSelect()
        expect(select.value).toBe('zerossl')
      })
    })

    it('defaults to "auto" when API returns invalid value', async () => {
      vi.mocked(settingsApi.getSettings).mockResolvedValue({
        'caddy.admin_api': 'http://localhost:2019',
        'caddy.ssl_provider': 'invalid-provider',
      })

      renderWithProviders(<SystemSettings />)

      await waitFor(() => {
        expect(screen.getByText('SSL Provider')).toBeTruthy()
      })

      const select = getSSLProviderSelect()
      expect(select.value).toBe('auto')
    })

    it('defaults to "auto" when API returns empty string', async () => {
      vi.mocked(settingsApi.getSettings).mockResolvedValue({
        'caddy.admin_api': 'http://localhost:2019',
        'caddy.ssl_provider': '',
      })

      renderWithProviders(<SystemSettings />)

      await waitFor(() => {
        expect(screen.getByText('SSL Provider')).toBeTruthy()
      })

      const select = getSSLProviderSelect()
      expect(select.value).toBe('auto')
    })

    it('allows changing SSL provider selection', async () => {
      renderWithProviders(<SystemSettings />)

      await waitFor(() => {
        expect(screen.getByText('SSL Provider')).toBeTruthy()
      })

      const user = userEvent.setup()
      const select = getSSLProviderSelect()

      // Change to Let's Encrypt Staging
      await user.selectOptions(select, 'letsencrypt-staging')
      expect(select.value).toBe('letsencrypt-staging')

      // Change to ZeroSSL
      await user.selectOptions(select, 'zerossl')
      expect(select.value).toBe('zerossl')

      // Change to Let's Encrypt Prod
      await user.selectOptions(select, 'letsencrypt-prod')
      expect(select.value).toBe('letsencrypt-prod')

      // Change back to Auto
      await user.selectOptions(select, 'auto')
      expect(select.value).toBe('auto')
    })

    it('saves SSL provider setting when save button is clicked', async () => {
      vi.mocked(settingsApi.updateSetting).mockResolvedValue(undefined)

      renderWithProviders(<SystemSettings />)

      await waitFor(() => {
        expect(screen.getByText('SSL Provider')).toBeTruthy()
      })

      const user = userEvent.setup()
      const select = getSSLProviderSelect()

      // Change to Let's Encrypt Staging
      await user.selectOptions(select, 'letsencrypt-staging')

      // Click save
      const saveButton = screen.getByRole('button', { name: /Save Settings/i })
      await user.click(saveButton)

      await waitFor(() => {
        expect(settingsApi.updateSetting).toHaveBeenCalledWith(
          'caddy.ssl_provider',
          'letsencrypt-staging',
          'caddy',
          'string'
        )
      })
    })

    it('handles backward compatibility with legacy "letsencrypt" value', async () => {
      // Old deployments might have "letsencrypt" instead of "letsencrypt-prod"
      vi.mocked(settingsApi.getSettings).mockResolvedValue({
        'caddy.admin_api': 'http://localhost:2019',
        'caddy.ssl_provider': 'letsencrypt',
      })

      renderWithProviders(<SystemSettings />)

      await waitFor(() => {
        expect(screen.getByText('SSL Provider')).toBeTruthy()
      })

      const select = getSSLProviderSelect()
      // Should default to 'auto' for invalid values
      expect(select.value).toBe('auto')
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

    it('shows loading state for system status', async () => {
      vi.mocked(client.get).mockReturnValue(new Promise(() => {}))

      renderWithProviders(<SystemSettings />)

      await waitFor(() => {
        expect(screen.getByText('System Status')).toBeTruthy()
      })

      // Check for loading spinner
      const spinners = document.querySelectorAll('.animate-spin')
      expect(spinners.length).toBeGreaterThan(0)
    })
  })

  describe('Features', () => {
    it('renders the Features section', async () => {
      renderWithProviders(<SystemSettings />)

      await waitFor(() => {
        expect(screen.getByText('Features')).toBeTruthy()
      })
    })

    it('displays Cerberus Security Suite toggle', async () => {
      vi.mocked(featureFlagsApi.getFeatureFlags).mockResolvedValue({
        'feature.cerberus.enabled': true,
        'feature.uptime.enabled': false,
      })

      renderWithProviders(<SystemSettings />)

      await waitFor(() => {
        expect(screen.getByText('Cerberus Security Suite')).toBeTruthy()
      })

      const cerberusLabel = screen.getByText('Cerberus Security Suite')
      const tooltipParent = cerberusLabel.closest('[title]') as HTMLElement
      expect(tooltipParent?.getAttribute('title')).toContain('Advanced security features')
    })

    it('displays Uptime Monitoring toggle', async () => {
      vi.mocked(featureFlagsApi.getFeatureFlags).mockResolvedValue({
        'feature.uptime.enabled': true,
        'feature.cerberus.enabled': false,
      })

      renderWithProviders(<SystemSettings />)

      await waitFor(() => {
        expect(screen.getByText('Uptime Monitoring')).toBeTruthy()
      })

      const uptimeLabel = screen.getByText('Uptime Monitoring')
      const tooltipParent = uptimeLabel.closest('[title]') as HTMLElement
      expect(tooltipParent?.getAttribute('title')).toContain('Monitor the availability')
    })

    it('shows Cerberus toggle as checked when enabled', async () => {
      vi.mocked(featureFlagsApi.getFeatureFlags).mockResolvedValue({
        'feature.cerberus.enabled': true,
        'feature.uptime.enabled': false,
      })

      renderWithProviders(<SystemSettings />)

      await waitFor(() => {
        expect(screen.getByText('Cerberus Security Suite')).toBeTruthy()
      })

      // Find the switch by looking for the parent div and then the input
      const cerberusText = screen.getByText('Cerberus Security Suite')
      const parentDiv = cerberusText.closest('.flex')
      const switchInput = parentDiv?.querySelector('input[type="checkbox"]') as HTMLInputElement
      expect(switchInput?.checked).toBe(true)
    })

    it('shows Uptime toggle as checked when enabled', async () => {
      vi.mocked(featureFlagsApi.getFeatureFlags).mockResolvedValue({
        'feature.uptime.enabled': true,
        'feature.cerberus.enabled': false,
      })

      renderWithProviders(<SystemSettings />)

      await waitFor(() => {
        expect(screen.getByText('Uptime Monitoring')).toBeTruthy()
      })

      const uptimeText = screen.getByText('Uptime Monitoring')
      const parentDiv = uptimeText.closest('.flex')
      const switchInput = parentDiv?.querySelector('input[type="checkbox"]') as HTMLInputElement
      expect(switchInput?.checked).toBe(true)
    })

    it('shows Cerberus toggle as unchecked when disabled', async () => {
      vi.mocked(featureFlagsApi.getFeatureFlags).mockResolvedValue({
        'feature.cerberus.enabled': false,
        'feature.uptime.enabled': false,
      })

      renderWithProviders(<SystemSettings />)

      await waitFor(() => {
        expect(screen.getByText('Cerberus Security Suite')).toBeTruthy()
      })

      const cerberusText = screen.getByText('Cerberus Security Suite')
      const parentDiv = cerberusText.closest('.flex')
      const switchInput = parentDiv?.querySelector('input[type="checkbox"]') as HTMLInputElement
      expect(switchInput?.checked).toBe(false)
    })

    it('toggles Cerberus feature flag when switch is clicked', async () => {
      vi.mocked(featureFlagsApi.getFeatureFlags).mockResolvedValue({
        'feature.cerberus.enabled': false,
        'feature.uptime.enabled': false,
      })
      vi.mocked(featureFlagsApi.updateFeatureFlags).mockResolvedValue(undefined)

      renderWithProviders(<SystemSettings />)

      await waitFor(() => {
        expect(screen.getByText('Cerberus Security Suite')).toBeTruthy()
      })

      const user = userEvent.setup()
      const cerberusText = screen.getByText('Cerberus Security Suite')
      const parentDiv = cerberusText.closest('.flex')
      const switchInput = parentDiv?.querySelector('input[type="checkbox"]') as HTMLInputElement

      await user.click(switchInput)

      await waitFor(() => {
        expect(featureFlagsApi.updateFeatureFlags).toHaveBeenCalledWith({
          'feature.cerberus.enabled': true,
        })
      })
    })

    it('toggles Uptime feature flag when switch is clicked', async () => {
      vi.mocked(featureFlagsApi.getFeatureFlags).mockResolvedValue({
        'feature.uptime.enabled': true,
        'feature.cerberus.enabled': false,
      })
      vi.mocked(featureFlagsApi.updateFeatureFlags).mockResolvedValue(undefined)

      renderWithProviders(<SystemSettings />)

      await waitFor(() => {
        expect(screen.getByText('Uptime Monitoring')).toBeTruthy()
      })

      const user = userEvent.setup()
      const uptimeText = screen.getByText('Uptime Monitoring')
      const parentDiv = uptimeText.closest('.flex')
      const switchInput = parentDiv?.querySelector('input[type="checkbox"]') as HTMLInputElement

      await user.click(switchInput)

      await waitFor(() => {
        expect(featureFlagsApi.updateFeatureFlags).toHaveBeenCalledWith({
          'feature.uptime.enabled': false,
        })
      })
    })

    it('shows loading message when feature flags are not loaded', async () => {
      vi.mocked(featureFlagsApi.getFeatureFlags).mockReturnValue(new Promise(() => {}))

      renderWithProviders(<SystemSettings />)

      await waitFor(() => {
        expect(screen.getByText('Features')).toBeTruthy()
      })

      expect(screen.getByText('Loading features...')).toBeTruthy()
    })

    it('shows loading overlay while toggling a feature flag', async () => {
      vi.mocked(featureFlagsApi.getFeatureFlags).mockResolvedValue({
        'feature.cerberus.enabled': false,
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
      const cerberusText = screen.getByText('Cerberus Security Suite')
      const parentDiv = cerberusText.closest('.flex')
      const switchInput = parentDiv?.querySelector('input[type="checkbox"]') as HTMLInputElement

      await user.click(switchInput)

      await waitFor(() => {
        expect(screen.getByText('Updating features...')).toBeInTheDocument()
      })
    })
  })
})
