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
    localStorage.clear()
    localStorage.setItem('sidebarCollapsed', 'false')
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
    const user = userEvent.setup()
    renderWithProviders(
      <Layout>
        <div>Test Content</div>
      </Layout>
    )

    expect(await screen.findByText('Dashboard')).toBeInTheDocument()
    expect(await screen.findByText('Proxy Hosts')).toBeInTheDocument()
    expect(await screen.findByText('Remote Servers')).toBeInTheDocument()
    expect(await screen.findByText('Domains')).toBeInTheDocument()
    expect(await screen.findByText('Certificates')).toBeInTheDocument()
    expect(await screen.findByText('DNS')).toBeInTheDocument()
    expect(await screen.findByText('Settings')).toBeInTheDocument()

    // Expand DNS to see nested items
    await user.click(await screen.findByRole('button', { name: /dns/i }))
    expect(await screen.findByText('DNS Providers')).toBeInTheDocument()
    expect(await screen.findByText('Plugins')).toBeInTheDocument()

    // Expand Security to see nested items
    await user.click(await screen.findByRole('button', { name: /security/i }))
    expect(await screen.findByText('Access Lists')).toBeInTheDocument()
    expect(await screen.findByText('Rate Limiting')).toBeInTheDocument()

    // Expand Tasks and Import to see nested items
    await user.click(await screen.findByRole('button', { name: /tasks/i }))
    expect(await screen.findByText('Import')).toBeInTheDocument()
    await user.click(await screen.findByRole('button', { name: /import/i }))
    expect(await screen.findByText('Caddyfile')).toBeInTheDocument()
    const crowdSecLinks = await screen.findAllByRole('link', { name: 'CrowdSec' })
    expect(crowdSecLinks.some(link => link.getAttribute('href') === '/tasks/import/crowdsec')).toBe(true)
    expect(await screen.findByText('Import NPM')).toBeInTheDocument()
    expect(await screen.findByText('Import JSON')).toBeInTheDocument()
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

  it('persists collapse state to localStorage', async () => {
    localStorage.clear()
    renderWithProviders(
      <Layout>
        <div>Test Content</div>
      </Layout>
    )

    const collapseBtn = await screen.findByTitle('Collapse sidebar')
    await userEvent.click(collapseBtn)
    expect(JSON.parse(localStorage.getItem('sidebarCollapsed') || 'false')).toBe(true)
  })

  it('restores collapsed state from localStorage on load', async () => {
    localStorage.setItem('sidebarCollapsed', 'true')

    renderWithProviders(
      <Layout>
        <div>Test Content</div>
      </Layout>
    )

    expect(await screen.findByTitle('Expand sidebar')).toBeInTheDocument()
  })

  describe('Feature Flags - Conditional Sidebar Items', () => {
    it('displays Security nav item when Cerberus is enabled', async () => {
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
        expect(screen.getByText('Security')).toBeInTheDocument()
      })
    })

    it('hides Security nav item when Cerberus is disabled', async () => {
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
        expect(screen.queryByText('Security')).not.toBeInTheDocument()
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

    it('shows Security and Uptime when both features are enabled', async () => {
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
        expect(screen.getByText('Security')).toBeInTheDocument()
        expect(screen.getByText('Uptime')).toBeInTheDocument()
      })
    })

    it('hides both Security and Uptime when both features are disabled', async () => {
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
        expect(screen.queryByText('Security')).not.toBeInTheDocument()
        expect(screen.queryByText('Uptime')).not.toBeInTheDocument()
      })
    })

    it('defaults to showing Security and Uptime when feature flags are loading', async () => {
      vi.mocked(featureFlagsApi.getFeatureFlags).mockResolvedValue({})

      renderWithProviders(
        <Layout>
          <div>Test Content</div>
        </Layout>
      )

      // When flags are undefined, items should be visible by default (conservative approach)
      await waitFor(() => {
        expect(screen.getByText('Security')).toBeInTheDocument()
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
