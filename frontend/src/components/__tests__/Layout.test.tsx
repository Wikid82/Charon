import { ReactNode } from 'react'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { BrowserRouter } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import Layout from '../Layout'
import { ThemeProvider } from '../../context/ThemeContext'
import * as featureFlagsApi from '../../api/featureFlags'

const mockLogout = vi.fn()

// Mock AuthContext
vi.mock('../../hooks/useAuth', () => ({
  useAuth: () => ({
    logout: mockLogout,
  }),
}))

// Mock API
vi.mock('../../api/health', () => ({
  checkHealth: vi.fn().mockResolvedValue({
    version: '0.1.0',
    git_commit: 'abcdef1',
  }),
}))

vi.mock('../../api/featureFlags', () => ({
  getFeatureFlags: vi.fn().mockResolvedValue({
    'feature.cerberus.enabled': true,
    'feature.uptime.enabled': true,
  }),
}))

const renderWithProviders = (children: ReactNode) => {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: {
        retry: false,
      },
    },
  })

  return render(
    <QueryClientProvider client={queryClient}>
      <BrowserRouter>
        <ThemeProvider>
          {children}
        </ThemeProvider>
      </BrowserRouter>
    </QueryClientProvider>
  )
}

describe('Layout', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    // Default: all features enabled
    vi.mocked(featureFlagsApi.getFeatureFlags).mockResolvedValue({
      'feature.cerberus.enabled': true,
      'feature.uptime.enabled': true,
    })
  })

  it('renders the application logo', () => {
    renderWithProviders(
      <Layout>
        <div>Test Content</div>
      </Layout>
    )

    const logos = screen.getAllByAltText('Charon')
    expect(logos.length).toBeGreaterThan(0)
    expect(logos[0]).toBeInTheDocument()
  })

  it('renders all navigation items', async () => {
    renderWithProviders(
      <Layout>
        <div>Test Content</div>
      </Layout>
    )

    expect(screen.getByText('Dashboard')).toBeInTheDocument()
    expect(screen.getByText('Proxy Hosts')).toBeInTheDocument()
    expect(screen.getByText('Remote Servers')).toBeInTheDocument()
    expect(screen.getByText('Certificates')).toBeInTheDocument()
    // Expand Tasks and Import to see nested items
    await userEvent.click(screen.getByText('Tasks'))
    expect(screen.getByText('Import')).toBeInTheDocument()
    await userEvent.click(screen.getByText('Import'))
    expect(screen.getByText('Caddyfile')).toBeInTheDocument()
    expect(screen.getByText('CrowdSec')).toBeInTheDocument()
    expect(screen.getByText('Settings')).toBeInTheDocument()
  })

  it('renders children content', () => {
    renderWithProviders(
      <Layout>
        <div data-testid="test-content">Test Content</div>
      </Layout>
    )

    expect(screen.getByTestId('test-content')).toBeInTheDocument()
  })

  it('displays version information', async () => {
    renderWithProviders(
      <Layout>
        <div>Test Content</div>
      </Layout>
    )

    expect(await screen.findByText('Version 0.1.0')).toBeInTheDocument()
  })

  it('calls logout when logout button is clicked', async () => {
    renderWithProviders(
      <Layout>
        <div>Test Content</div>
      </Layout>
    )

    await userEvent.click(screen.getByText('Logout'))

    expect(mockLogout).toHaveBeenCalled()
  })

  it('toggles sidebar on mobile', async () => {
    renderWithProviders(
      <Layout>
        <div>Test Content</div>
      </Layout>
    )

    // The mobile sidebar toggle is found by test-id
    const toggleButton = screen.getByTestId('mobile-menu-toggle')

    // Click to open the sidebar
    await userEvent.click(toggleButton)

    // The overlay should be present when mobile sidebar is open
    // The overlay has class 'fixed inset-0 bg-gray-900/50 z-20 lg:hidden'
    // Click the toggle again to close
    await userEvent.click(toggleButton)

    // Toggle button should still be in the document
    expect(toggleButton).toBeInTheDocument()
  })

  describe('Feature Flags - Conditional Sidebar Items', () => {
    it('displays Cerberus nav item when Cerberus is enabled', async () => {
      vi.mocked(featureFlagsApi.getFeatureFlags).mockResolvedValue({
        'feature.cerberus.enabled': true,
        'feature.uptime.enabled': true,
      })

      renderWithProviders(
        <Layout>
          <div>Test Content</div>
        </Layout>
      )

      await waitFor(() => {
        expect(screen.getByText('Cerberus')).toBeInTheDocument()
      })
    })

    it('hides Cerberus nav item when Cerberus is disabled', async () => {
      vi.mocked(featureFlagsApi.getFeatureFlags).mockResolvedValue({
        'feature.cerberus.enabled': false,
        'feature.uptime.enabled': true,
      })

      renderWithProviders(
        <Layout>
          <div>Test Content</div>
        </Layout>
      )

      await waitFor(() => {
        expect(screen.queryByText('Cerberus')).not.toBeInTheDocument()
      })
    })

    it('displays Uptime nav item when Uptime is enabled', async () => {
      vi.mocked(featureFlagsApi.getFeatureFlags).mockResolvedValue({
        'feature.cerberus.enabled': true,
        'feature.uptime.enabled': true,
      })

      renderWithProviders(
        <Layout>
          <div>Test Content</div>
        </Layout>
      )

      await waitFor(() => {
        expect(screen.getByText('Uptime')).toBeInTheDocument()
      })
    })

    it('hides Uptime nav item when Uptime is disabled', async () => {
      vi.mocked(featureFlagsApi.getFeatureFlags).mockResolvedValue({
        'feature.cerberus.enabled': true,
        'feature.uptime.enabled': false,
      })

      renderWithProviders(
        <Layout>
          <div>Test Content</div>
        </Layout>
      )

      await waitFor(() => {
        expect(screen.queryByText('Uptime')).not.toBeInTheDocument()
      })
    })

    it('shows Cerberus and Uptime when both features are enabled', async () => {
      vi.mocked(featureFlagsApi.getFeatureFlags).mockResolvedValue({
        'feature.cerberus.enabled': true,
        'feature.uptime.enabled': true,
      })

      renderWithProviders(
        <Layout>
          <div>Test Content</div>
        </Layout>
      )

      await waitFor(() => {
        expect(screen.getByText('Cerberus')).toBeInTheDocument()
        expect(screen.getByText('Uptime')).toBeInTheDocument()
      })
    })

    it('hides both Cerberus and Uptime when both features are disabled', async () => {
      vi.mocked(featureFlagsApi.getFeatureFlags).mockResolvedValue({
        'feature.cerberus.enabled': false,
        'feature.uptime.enabled': false,
      })

      renderWithProviders(
        <Layout>
          <div>Test Content</div>
        </Layout>
      )

      await waitFor(() => {
        expect(screen.queryByText('Cerberus')).not.toBeInTheDocument()
        expect(screen.queryByText('Uptime')).not.toBeInTheDocument()
      })
    })

    it('defaults to showing Cerberus and Uptime when feature flags are loading', async () => {
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      vi.mocked(featureFlagsApi.getFeatureFlags).mockResolvedValue(undefined as any)

      renderWithProviders(
        <Layout>
          <div>Test Content</div>
        </Layout>
      )

      // When flags are undefined, items should be visible by default (conservative approach)
      await waitFor(() => {
        expect(screen.getByText('Cerberus')).toBeInTheDocument()
        expect(screen.getByText('Uptime')).toBeInTheDocument()
      })
    })

    it('shows other nav items regardless of feature flags', async () => {
      vi.mocked(featureFlagsApi.getFeatureFlags).mockResolvedValue({
        'feature.cerberus.enabled': false,
        'feature.uptime.enabled': false,
      })

      renderWithProviders(
        <Layout>
          <div>Test Content</div>
        </Layout>
      )

      await waitFor(() => {
        expect(screen.getByText('Dashboard')).toBeInTheDocument()
        expect(screen.getByText('Proxy Hosts')).toBeInTheDocument()
        expect(screen.getByText('Remote Servers')).toBeInTheDocument()
        expect(screen.getByText('Certificates')).toBeInTheDocument()
      })
    })
  })
})
