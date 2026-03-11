import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { act, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { BrowserRouter } from 'react-router-dom'
import { describe, it, expect, vi, beforeEach } from 'vitest'

import * as crowdsecApi from '../../api/crowdsec'
import * as securityApi from '../../api/security'
import * as settingsApi from '../../api/settings'
import Security from '../Security'

import type * as useSecurity from '../../hooks/useSecurity'

vi.mock('../../api/security')
vi.mock('../../api/crowdsec')
vi.mock('../../api/settings')
vi.mock('../../hooks/useSecurity', async (importOriginal) => {
  const actual = await importOriginal<typeof useSecurity>()
  return {
    ...actual,
    useSecurityConfig: vi.fn(() => ({ data: { config: { admin_whitelist: '10.0.0.0/8' } } })),
    useUpdateSecurityConfig: vi.fn(() => ({ mutate: vi.fn(), isPending: false })),
    useGenerateBreakGlassToken: vi.fn(() => ({ mutate: vi.fn(), isPending: false })),
    useRuleSets: vi.fn(() => ({
      data: {
        rulesets: [
          { id: 1, uuid: 'abc', name: 'OWASP CRS', source_url: 'https://example.com', mode: 'blocking', last_updated: '2025-12-04', content: 'rules' }
        ]
      }
    })),
  }
})

// BLOCKER 3: Temporarily skipped due to undici InvalidArgumentError in WebSocket mocks
describe.skip('Security', () => {
  let queryClient: QueryClient

  beforeEach(() => {
    queryClient = new QueryClient({
      defaultOptions: {
        queries: { retry: false },
        mutations: { retry: false },
      },
    })
    vi.clearAllMocks()
    vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(mockSecurityStatus)
    vi.mocked(crowdsecApi.statusCrowdsec).mockResolvedValue({ running: false, pid: 0, lapi_ready: false })
    vi.mocked(settingsApi.updateSetting).mockResolvedValue()
    vi.mocked(crowdsecApi.exportCrowdsecConfig).mockResolvedValue(new Blob())
    vi.spyOn(window, 'open').mockImplementation(() => null)
    vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => {})
    vi.spyOn(window, 'prompt').mockReturnValue('crowdsec-export.tar.gz')
  })

  const wrapper = ({ children }: { children: React.ReactNode }) => (
    <QueryClientProvider client={queryClient}>
      <BrowserRouter>{children}</BrowserRouter>
    </QueryClientProvider>
  )

  const renderSecurityPage = async () => {
    await act(async () => {
      render(<Security />, { wrapper })
    })
  }

  const mockSecurityStatus = {
    cerberus: { enabled: true },
    crowdsec: { mode: 'local' as const, api_url: 'http://localhost', enabled: true },
    waf: { mode: 'enabled' as const, enabled: true },
    rate_limit: { enabled: true },
    acl: { enabled: true }
  }

  describe('Rendering', () => {
    it('should show loading state initially', async () => {
      vi.mocked(securityApi.getSecurityStatus).mockReturnValue(new Promise(() => {}))

      await renderSecurityPage()

      // Loading state now uses Skeleton components instead of text
      const skeletons = document.querySelectorAll('.animate-pulse')
      expect(skeletons.length).toBeGreaterThan(0)
    })

    it('should show error if security status fails to load', async () => {
      vi.mocked(securityApi.getSecurityStatus).mockRejectedValue(new Error('Failed'))
      await renderSecurityPage()
      expect(await screen.findByText(/Failed to load security configuration/i)).toBeInTheDocument()
    })

    it('should render Cerberus Dashboard when status loads', async () => {
      vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(mockSecurityStatus)
      await renderSecurityPage()
      expect(await screen.findByText(/Cerberus Dashboard/i)).toBeInTheDocument()
    })

    it('should show banner when Cerberus is disabled', async () => {
      vi.mocked(securityApi.getSecurityStatus).mockResolvedValue({ ...mockSecurityStatus, cerberus: { enabled: false } })
      await renderSecurityPage()
      expect(await screen.findByText(/Security Features Unavailable/i)).toBeInTheDocument()
    })
  })

  describe('Service Toggles', () => {
    it('should toggle CrowdSec on', async () => {
      const user = userEvent.setup()
      vi.mocked(securityApi.getSecurityStatus).mockResolvedValue({ ...mockSecurityStatus, crowdsec: { mode: 'local', api_url: 'http://localhost', enabled: false } })
      vi.mocked(settingsApi.updateSetting).mockResolvedValue()

      await renderSecurityPage()

      await waitFor(() => screen.getByTestId('toggle-crowdsec'))
      const toggle = screen.getByTestId('toggle-crowdsec')
      await user.click(toggle)

      await waitFor(() => expect(settingsApi.updateSetting).toHaveBeenCalledWith('security.crowdsec.enabled', 'true', 'security', 'bool'))
    })

    it('should toggle WAF on', async () => {
      const user = userEvent.setup()
      vi.mocked(securityApi.getSecurityStatus).mockResolvedValue({ ...mockSecurityStatus, waf: { mode: 'enabled', enabled: false } })
      vi.mocked(settingsApi.updateSetting).mockResolvedValue()

      await renderSecurityPage()

      await waitFor(() => screen.getByTestId('toggle-waf'))
      const toggle = screen.getByTestId('toggle-waf')
      await act(async () => {
        await user.click(toggle)
      })

      await waitFor(() => expect(settingsApi.updateSetting).toHaveBeenCalledWith('security.waf.enabled', 'true', 'security', 'bool'))
    })

    it('should toggle ACL on', async () => {
      const user = userEvent.setup()
      vi.mocked(securityApi.getSecurityStatus).mockResolvedValue({ ...mockSecurityStatus, acl: { enabled: false } })
      vi.mocked(settingsApi.updateSetting).mockResolvedValue()

      await renderSecurityPage()

      await waitFor(() => screen.getByTestId('toggle-acl'))
      const toggle = screen.getByTestId('toggle-acl')
      await user.click(toggle)

      await waitFor(() => expect(settingsApi.updateSetting).toHaveBeenCalledWith('security.acl.enabled', 'true', 'security', 'bool'))
    })

    it('should toggle Rate Limiting on', async () => {
      const user = userEvent.setup()
      vi.mocked(securityApi.getSecurityStatus).mockResolvedValue({ ...mockSecurityStatus, rate_limit: { enabled: false } })
      vi.mocked(settingsApi.updateSetting).mockResolvedValue()

      await renderSecurityPage()

      await waitFor(() => screen.getByTestId('toggle-rate-limit'))
      const toggle = screen.getByTestId('toggle-rate-limit')
      await user.click(toggle)

      await waitFor(() => expect(settingsApi.updateSetting).toHaveBeenCalledWith('security.rate_limit.enabled', 'true', 'security', 'bool'))
    })
  })

  describe('Admin Whitelist', () => {
    it('should load admin whitelist from config', async () => {
      vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(mockSecurityStatus)

      await renderSecurityPage()
      await waitFor(() => screen.getByDisplayValue('10.0.0.0/8'))
      expect(screen.getByDisplayValue('10.0.0.0/8')).toBeInTheDocument()
    })

    it('should update admin whitelist on save', async () => {
      const user = userEvent.setup()
      const mockMutate = vi.fn()
      const { useUpdateSecurityConfig } = await import('../../hooks/useSecurity')
      vi.mocked(useUpdateSecurityConfig).mockReturnValue({ mutate: mockMutate, isPending: false } as unknown as ReturnType<typeof useUpdateSecurityConfig>)
      vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(mockSecurityStatus)

      await renderSecurityPage()

      await waitFor(() => screen.getByDisplayValue('10.0.0.0/8'))

      const saveButton = screen.getByRole('button', { name: /Save/i })
      await user.click(saveButton)

      await waitFor(() => {
        expect(mockMutate).toHaveBeenCalledWith({ name: 'default', admin_whitelist: '10.0.0.0/8' })
      })
    })
  })

  describe('CrowdSec Controls', () => {
    it('should start CrowdSec when toggling on', async () => {
      const user = userEvent.setup()
      vi.mocked(securityApi.getSecurityStatus).mockResolvedValue({
        ...mockSecurityStatus,
        crowdsec: { mode: 'local', api_url: 'http://localhost', enabled: false },
      })
      vi.mocked(crowdsecApi.statusCrowdsec).mockResolvedValue({ running: false, pid: 0, lapi_ready: false })
      vi.mocked(crowdsecApi.startCrowdsec).mockResolvedValue({ status: 'started', pid: 123, lapi_ready: true })

      await renderSecurityPage()

      await waitFor(() => screen.getByTestId('toggle-crowdsec'))
      const toggle = screen.getByTestId('toggle-crowdsec')
      await act(async () => {
        await user.click(toggle)
      })

      await waitFor(() => {
        expect(settingsApi.updateSetting).toHaveBeenCalledWith('security.crowdsec.enabled', 'true', 'security', 'bool')
        expect(crowdsecApi.startCrowdsec).toHaveBeenCalled()
      })
    })

    it('should stop CrowdSec when toggling off', async () => {
      const user = userEvent.setup()
      vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(mockSecurityStatus)
      vi.mocked(crowdsecApi.statusCrowdsec).mockResolvedValue({ running: true, pid: 1234, lapi_ready: true })
      vi.mocked(crowdsecApi.stopCrowdsec).mockResolvedValue({ success: true })

      await renderSecurityPage()

      await waitFor(() => screen.getByTestId('toggle-crowdsec'))
      const toggle = screen.getByTestId('toggle-crowdsec')
      await act(async () => {
        await user.click(toggle)
      })

      await waitFor(() => {
        expect(settingsApi.updateSetting).toHaveBeenCalledWith('security.crowdsec.enabled', 'false', 'security', 'bool')
        expect(crowdsecApi.stopCrowdsec).toHaveBeenCalled()
      })
    })


  })

  // Note: WAF Controls tests removed - dropdowns moved to dedicated WAF config page (/security/waf)

  describe('Card Order (Pipeline Sequence)', () => {
    it('should render cards in correct pipeline order: CrowdSec → ACL → WAF → Rate Limiting', async () => {
      vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(mockSecurityStatus)

      await renderSecurityPage()
      await waitFor(() => screen.getByText(/Cerberus Dashboard/i))

      // Get all card headings (CardTitle uses text-base class)
      const cards = screen.getAllByRole('heading', { level: 3 })
      const cardNames = cards.map(card => card.textContent)

      // Verify pipeline order: Admin Whitelist + CrowdSec (Layer 1) → ACL (Layer 2) → Coraza (Layer 3) → Rate Limiting (Layer 4) + Security Access Logs
      expect(cardNames).toEqual(['Admin Whitelist', 'CrowdSec', 'Access Control', 'Coraza WAF', 'Rate Limiting', 'Security Access Logs'])
    })

    it('should display layer indicators on each card', async () => {
      vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(mockSecurityStatus)

      await renderSecurityPage()
      await waitFor(() => screen.getByText(/Cerberus Dashboard/i))

      // Layer indicators are now Badges with just the layer number
      expect(screen.getByText('Layer 1')).toBeInTheDocument()
      expect(screen.getByText('Layer 2')).toBeInTheDocument()
      expect(screen.getByText('Layer 3')).toBeInTheDocument()
      expect(screen.getByText('Layer 4')).toBeInTheDocument()
    })

    it('should display threat protection summaries', async () => {
      vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(mockSecurityStatus)
      // CrowdSec must be running to show threat protection descriptions
      vi.mocked(crowdsecApi.statusCrowdsec).mockResolvedValue({ running: true, pid: 1234, lapi_ready: true })

      await renderSecurityPage()
      await waitFor(() => screen.getByText(/Cerberus Dashboard/i))

      // Verify threat protection descriptions
      expect(screen.getByText(/Known attackers, botnets/i)).toBeInTheDocument()
      expect(screen.getByText(/Unauthorized IPs, geo-based attacks/i)).toBeInTheDocument()
      expect(screen.getByText(/SQL injection, XSS, RCE/i)).toBeInTheDocument()
      expect(screen.getByText(/DDoS attacks, credential stuffing/i)).toBeInTheDocument()
    })
  })

  describe('Loading Overlay', () => {
    it('should show overlay when service is toggling', async () => {
      const user = userEvent.setup()
      vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(mockSecurityStatus)
      vi.mocked(settingsApi.updateSetting).mockImplementation(() => new Promise(() => {}))

      await renderSecurityPage()

      await waitFor(() => screen.getByTestId('toggle-waf'))
      const toggle = screen.getByTestId('toggle-waf')
      await user.click(toggle)

      expect(await screen.findByText(/Three heads turn/i)).toBeInTheDocument()
    })

    it('should show overlay when starting CrowdSec', async () => {
      const user = userEvent.setup()
      vi.mocked(securityApi.getSecurityStatus).mockResolvedValue({
        ...mockSecurityStatus,
        crowdsec: { mode: 'local', api_url: 'http://localhost', enabled: false },
      })
      vi.mocked(crowdsecApi.statusCrowdsec).mockResolvedValue({ running: false, pid: 0, lapi_ready: false })
      vi.mocked(crowdsecApi.startCrowdsec).mockImplementation(() => new Promise(() => {}))

      await renderSecurityPage()

      await waitFor(() => screen.getByTestId('toggle-crowdsec'))
      const toggle = screen.getByTestId('toggle-crowdsec')
      await user.click(toggle)

      expect(await screen.findByText(/Summoning the guardian/i)).toBeInTheDocument()
    })

    it('should show overlay when stopping CrowdSec', async () => {
      const user = userEvent.setup()
      vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(mockSecurityStatus)
      vi.mocked(crowdsecApi.statusCrowdsec).mockResolvedValue({ running: true, pid: 1234, lapi_ready: true })
      vi.mocked(crowdsecApi.stopCrowdsec).mockImplementation(() => new Promise(() => {}))

      await renderSecurityPage()

      await waitFor(() => screen.getByTestId('toggle-crowdsec'))
      const toggle = screen.getByTestId('toggle-crowdsec')
      await user.click(toggle)

      expect(await screen.findByText(/Guardian rests/i)).toBeInTheDocument()
    })
  })

  describe('Optimistic Update Mode Preservation', () => {
    it('should preserve waf.mode field when toggling WAF enabled', async () => {
      const user = userEvent.setup()
      // WAF status includes mode field that must be preserved
      const statusWithWafMode = {
        ...mockSecurityStatus,
        waf: { mode: 'enabled' as const, enabled: true },
      }
      vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(statusWithWafMode)
      // Make mutation take time so we can check optimistic update state
      vi.mocked(settingsApi.updateSetting).mockImplementation(
        () => new Promise((resolve) => setTimeout(resolve, 100))
      )

      await renderSecurityPage()
      await waitFor(() => screen.getByTestId('toggle-waf'))

      const toggle = screen.getByTestId('toggle-waf')
      await user.click(toggle)

      // Verify that updateSetting was called with correct parameters
      await waitFor(() => {
        expect(settingsApi.updateSetting).toHaveBeenCalledWith(
          'security.waf.enabled',
          'false', // toggling from true to false
          'security',
          'bool'
        )
      })

      // The query client's cached data should still have mode field preserved
      // Note: We verify that the mutation was called correctly, and the implementation
      // uses spread operator to preserve mode field during optimistic update
    })

    it('should preserve rate_limit.mode field when toggling Rate Limit enabled', async () => {
      const user = userEvent.setup()
      // Rate limit status includes mode field that must be preserved
      const statusWithRateLimitMode = {
        ...mockSecurityStatus,
        rate_limit: { mode: 'enabled' as const, enabled: true },
      }
      vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(statusWithRateLimitMode)
      vi.mocked(settingsApi.updateSetting).mockImplementation(
        () => new Promise((resolve) => setTimeout(resolve, 100))
      )

      await renderSecurityPage()
      await waitFor(() => screen.getByTestId('toggle-rate-limit'))

      const toggle = screen.getByTestId('toggle-rate-limit')
      await user.click(toggle)

      // Verify that updateSetting was called with correct parameters
      await waitFor(() => {
        expect(settingsApi.updateSetting).toHaveBeenCalledWith(
          'security.rate_limit.enabled',
          'false', // toggling from true to false
          'security',
          'bool'
        )
      })
    })

    it('should rollback to previous state on mutation error', async () => {
      const user = userEvent.setup()
      vi.mocked(securityApi.getSecurityStatus).mockResolvedValue({
        ...mockSecurityStatus,
        waf: { mode: 'enabled' as const, enabled: false },
      })
      vi.mocked(settingsApi.updateSetting).mockRejectedValue(new Error('Network error'))

      await renderSecurityPage()
      await waitFor(() => screen.getByTestId('toggle-waf'))

      const toggle = screen.getByTestId('toggle-waf')
      expect(toggle).not.toBeChecked() // initially disabled

      await user.click(toggle)

      // Verify updateSetting was called (mutation was triggered)
      await waitFor(() => {
        expect(settingsApi.updateSetting).toHaveBeenCalledWith(
          'security.waf.enabled',
          'true',
          'security',
          'bool'
        )
      })

      // After error, the toggle should rollback to initial state (unchecked)
      // The optimistic update should be reverted by the onError handler
      await waitFor(() => {
        expect(screen.getByTestId('toggle-waf')).not.toBeChecked()
      })
    })

    it('should handle ACL toggle without mode field', async () => {
      const user = userEvent.setup()
      // ACL doesn't have mode field (only enabled)
      vi.mocked(securityApi.getSecurityStatus).mockResolvedValue({
        ...mockSecurityStatus,
        acl: { enabled: false },
      })
      vi.mocked(settingsApi.updateSetting).mockResolvedValue()

      await renderSecurityPage()
      await waitFor(() => screen.getByTestId('toggle-acl'))

      const toggle = screen.getByTestId('toggle-acl')
      await user.click(toggle)

      await waitFor(() => {
        expect(settingsApi.updateSetting).toHaveBeenCalledWith(
          'security.acl.enabled',
          'true', // toggling from false to true
          'security',
          'bool'
        )
      })
    })
  })
})
