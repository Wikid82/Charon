/**
 * Security Loading Overlay Tests
 * Test IDs: LS-01 through LS-10
 *
 * Tests ConfigReloadOverlay appears during operations, specific loading messages,
 * and overlay blocks interactions.
 */
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

// Test Data Fixtures
const mockSecurityStatusAllEnabled = {
  cerberus: { enabled: true },
  crowdsec: { mode: 'local' as const, api_url: 'http://localhost', enabled: true },
  waf: { mode: 'enabled' as const, enabled: true },
  rate_limit: { enabled: true },
  acl: { enabled: true },
}

const mockSecurityStatusCrowdsecDisabled = {
  cerberus: { enabled: true },
  crowdsec: { mode: 'local' as const, api_url: 'http://localhost', enabled: false },
  waf: { mode: 'enabled' as const, enabled: true },
  rate_limit: { enabled: true },
  acl: { enabled: true },
}

// BLOCKER 3: Temporarily skipped due to undici InvalidArgumentError in WebSocket mocks
describe.skip('Security Loading Overlay Tests', () => {
  let queryClient: QueryClient

  beforeEach(() => {
    queryClient = new QueryClient({
      defaultOptions: {
        queries: { retry: false },
        mutations: { retry: false },
      },
    })
    vi.clearAllMocks()
    vi.mocked(crowdsecApi.statusCrowdsec).mockResolvedValue({ running: false, pid: 0, lapi_ready: false })
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

  describe('LS-01: Initial Page Load Shows Loading Text', () => {
    it('should show Skeleton components during initial load', async () => {
      vi.mocked(securityApi.getSecurityStatus).mockReturnValue(new Promise(() => {}))

      await renderSecurityPage()

      // Loading state now uses Skeleton components instead of text
      const skeletons = document.querySelectorAll('.animate-pulse')
      expect(skeletons.length).toBeGreaterThan(0)
    })
  })

  describe('LS-02: Toggling Service Shows CerberusLoader Overlay', () => {
    it('should show ConfigReloadOverlay with type="cerberus" when toggling', async () => {
      const user = userEvent.setup()
      vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(mockSecurityStatusAllEnabled)
      // Never-resolving promise to keep loading state
      vi.mocked(settingsApi.updateSetting).mockImplementation(() => new Promise(() => {}))

      await renderSecurityPage()

      await waitFor(() => screen.getByTestId('toggle-waf'))
      const toggle = screen.getByTestId('toggle-waf')
      await user.click(toggle)

      await waitFor(() => {
        expect(screen.getByText(/Three heads turn/i)).toBeInTheDocument()
        expect(screen.getByText(/Cerberus configuration updating/i)).toBeInTheDocument()
      })
    })
  })

  describe('LS-03: Starting CrowdSec Shows "Summoning the guardian..."', () => {
    it('should show specific message for CrowdSec start operation', async () => {
      const user = userEvent.setup()
      vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(mockSecurityStatusCrowdsecDisabled)
      vi.mocked(crowdsecApi.statusCrowdsec).mockResolvedValue({ running: false, pid: 0, lapi_ready: false })
      vi.mocked(settingsApi.updateSetting).mockResolvedValue()
      // Never-resolving promise to keep loading state
      vi.mocked(crowdsecApi.startCrowdsec).mockImplementation(() => new Promise(() => {}))

      await renderSecurityPage()

      await waitFor(() => screen.getByTestId('toggle-crowdsec'))
      const toggle = screen.getByTestId('toggle-crowdsec')
      await user.click(toggle)

      await waitFor(() => {
        expect(screen.getByText(/Summoning the guardian/i)).toBeInTheDocument()
        expect(screen.getByText(/CrowdSec is starting/i)).toBeInTheDocument()
      })
    })
  })

  describe('LS-04: Stopping CrowdSec Shows "Guardian rests..."', () => {
    it('should show specific message for CrowdSec stop operation', async () => {
      const user = userEvent.setup()
      vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(mockSecurityStatusAllEnabled)
      vi.mocked(crowdsecApi.statusCrowdsec).mockResolvedValue({ running: true, pid: 1234, lapi_ready: true })
      vi.mocked(settingsApi.updateSetting).mockResolvedValue()
      // Never-resolving promise to keep loading state
      vi.mocked(crowdsecApi.stopCrowdsec).mockImplementation(() => new Promise(() => {}))

      await renderSecurityPage()

      await waitFor(() => screen.getByTestId('toggle-crowdsec'))
      const toggle = screen.getByTestId('toggle-crowdsec')
      await user.click(toggle)

      await waitFor(() => {
        expect(screen.getByText(/Guardian rests/i)).toBeInTheDocument()
        expect(screen.getByText(/CrowdSec is stopping/i)).toBeInTheDocument()
      })
    })
  })

  describe('LS-05: WAF Config Operations Show Overlay', () => {
    it('should show overlay when toggling WAF', async () => {
      const user = userEvent.setup()
      vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(mockSecurityStatusAllEnabled)
      vi.mocked(settingsApi.updateSetting).mockImplementation(() => new Promise(() => {}))

      await renderSecurityPage()

      await waitFor(() => screen.getByTestId('toggle-waf'))
      await user.click(screen.getByTestId('toggle-waf'))

      await waitFor(() => {
        expect(screen.getByText(/Three heads turn/i)).toBeInTheDocument()
      })
    })
  })

  describe('LS-06: Rate Limiting Toggle Shows Overlay', () => {
    it('should show overlay when toggling rate limiting', async () => {
      const user = userEvent.setup()
      vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(mockSecurityStatusAllEnabled)
      vi.mocked(settingsApi.updateSetting).mockImplementation(() => new Promise(() => {}))

      await renderSecurityPage()

      await waitFor(() => screen.getByTestId('toggle-rate-limit'))
      await user.click(screen.getByTestId('toggle-rate-limit'))

      await waitFor(() => {
        expect(screen.getByText(/Three heads turn/i)).toBeInTheDocument()
        expect(screen.getByText(/Cerberus configuration updating/i)).toBeInTheDocument()
      })
    })
  })

  describe('LS-07: ACL Toggle Shows Overlay', () => {
    it('should show overlay when toggling ACL', async () => {
      const user = userEvent.setup()
      vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(mockSecurityStatusAllEnabled)
      vi.mocked(settingsApi.updateSetting).mockImplementation(() => new Promise(() => {}))

      await renderSecurityPage()

      await waitFor(() => screen.getByTestId('toggle-acl'))
      await user.click(screen.getByTestId('toggle-acl'))

      await waitFor(() => {
        expect(screen.getByText(/Three heads turn/i)).toBeInTheDocument()
      })
    })
  })

  describe('LS-08: Overlay Contains CerberusLoader Component', () => {
    it('should render CerberusLoader animation within overlay', async () => {
      const user = userEvent.setup()
      vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(mockSecurityStatusAllEnabled)
      vi.mocked(settingsApi.updateSetting).mockImplementation(() => new Promise(() => {}))

      await renderSecurityPage()

      await waitFor(() => screen.getByTestId('toggle-waf'))
      await user.click(screen.getByTestId('toggle-waf'))

      await waitFor(() => {
        // The CerberusLoader has role="status" with aria-label="Security Loading"
        expect(screen.getByRole('status', { name: /Security Loading/i })).toBeInTheDocument()
      })
    })
  })

  describe('LS-09: Overlay Blocks Interactions', () => {
    it('should show overlay during toggle operation', async () => {
      const user = userEvent.setup()
      vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(mockSecurityStatusAllEnabled)
      vi.mocked(settingsApi.updateSetting).mockImplementation(() => new Promise(() => {}))

      await renderSecurityPage()

      await waitFor(() => screen.getByTestId('toggle-waf'))
      await user.click(screen.getByTestId('toggle-waf'))

      await waitFor(() => {
        // Verify the fixed overlay is present (it has class "fixed inset-0")
        const overlay = document.querySelector('.fixed.inset-0')
        expect(overlay).toBeInTheDocument()
      })
    })

    it('should have z-50 overlay that covers content', async () => {
      const user = userEvent.setup()
      vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(mockSecurityStatusAllEnabled)
      vi.mocked(settingsApi.updateSetting).mockImplementation(() => new Promise(() => {}))

      await renderSecurityPage()

      await waitFor(() => screen.getByTestId('toggle-waf'))
      await user.click(screen.getByTestId('toggle-waf'))

      await waitFor(() => {
        const overlay = document.querySelector('.z-50')
        expect(overlay).toBeInTheDocument()
      })
    })
  })

  describe('LS-10: Overlay Disappears on Mutation Success', () => {
    it('should remove overlay after toggle completes successfully', async () => {
      const user = userEvent.setup()
      vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(mockSecurityStatusAllEnabled)

      // First call - resolves quickly to simulate successful toggle
      vi.mocked(settingsApi.updateSetting).mockResolvedValue()

      await renderSecurityPage()

      await waitFor(() => screen.getByTestId('toggle-waf'))

      // The overlay might flash briefly and disappear, so we verify no overlay after completion
      await user.click(screen.getByTestId('toggle-waf'))

      // Wait for mutation to complete and overlay to disappear
      await waitFor(() => {
        const overlay = document.querySelector('.fixed.inset-0.bg-slate-900\\/70')
        // After successful mutation, overlay should be gone
        expect(overlay).not.toBeInTheDocument()
      }, { timeout: 3000 })
    })

    it('should not show overlay when mutation completes instantly', async () => {
      vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(mockSecurityStatusAllEnabled)
      vi.mocked(settingsApi.updateSetting).mockResolvedValue()

      await renderSecurityPage()

      await waitFor(() => {
        expect(screen.getByText(/Cerberus Dashboard/i)).toBeInTheDocument()
      })

      // After successful load, no overlay should be present
      const overlay = document.querySelector('.fixed.inset-0.bg-slate-900\\/70')
      expect(overlay).not.toBeInTheDocument()
    })
  })
})
