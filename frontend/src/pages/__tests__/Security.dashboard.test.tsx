/**
 * Security Dashboard Card Status Verification Tests
 * Test IDs: SD-01 through SD-10
 *
 * Tests all 4 security cards display correct status, Cerberus disabled banner,
 * and toggle switches disabled when Cerberus is off.
 */
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { act, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { BrowserRouter } from 'react-router-dom'
import Security from '../Security'
import * as securityApi from '../../api/security'
import * as crowdsecApi from '../../api/crowdsec'
import * as settingsApi from '../../api/settings'

vi.mock('../../api/security')
vi.mock('../../api/crowdsec')
vi.mock('../../api/settings')
vi.mock('../../hooks/useSecurity', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../../hooks/useSecurity')>()
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

const mockSecurityStatusCerberusDisabled = {
  cerberus: { enabled: false },
  crowdsec: { mode: 'disabled' as const, api_url: '', enabled: false },
  waf: { mode: 'disabled' as const, enabled: false },
  rate_limit: { enabled: false },
  acl: { enabled: false },
}

const mockSecurityStatusMixed = {
  cerberus: { enabled: true },
  crowdsec: { mode: 'local' as const, api_url: 'http://localhost', enabled: true },
  waf: { mode: 'disabled' as const, enabled: false },
  rate_limit: { enabled: true },
  acl: { enabled: false },
}

describe('Security Dashboard - Card Status Tests', () => {
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

  describe('SD-01: Cerberus Disabled Banner', () => {
    it('should show "Cerberus Disabled" banner when cerberus.enabled=false', async () => {
      vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(mockSecurityStatusCerberusDisabled)
      await renderSecurityPage()

      await waitFor(() => {
        expect(screen.getByText(/Cerberus Disabled/i)).toBeInTheDocument()
      })
    })

    it('should show documentation link in disabled banner', async () => {
      vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(mockSecurityStatusCerberusDisabled)
      await renderSecurityPage()

      await waitFor(() => {
        // Multiple Documentation buttons exist (one in banner, one in header)
        const docButtons = screen.getAllByRole('button', { name: /Documentation/i })
        expect(docButtons.length).toBeGreaterThanOrEqual(1)
        // The primary one in the banner should have blue-600 (primary variant)
        expect(docButtons[0]).toBeInTheDocument()
      })
    })

    it('should not show banner when Cerberus is enabled', async () => {
      vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(mockSecurityStatusAllEnabled)
      await renderSecurityPage()

      await waitFor(() => {
        expect(screen.getByText(/Cerberus Dashboard/i)).toBeInTheDocument()
      })
      expect(screen.queryByText(/^Cerberus Disabled$/)).not.toBeInTheDocument()
    })
  })

  describe('SD-02: CrowdSec Card Active Status', () => {
    it('should show "Active" when crowdsec.enabled=true', async () => {
      vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(mockSecurityStatusAllEnabled)
      vi.mocked(crowdsecApi.statusCrowdsec).mockResolvedValue({ running: true, pid: 1234, lapi_ready: true })

      await renderSecurityPage()

      await waitFor(() => {
        const cards = screen.getAllByText('Active')
        expect(cards.length).toBeGreaterThan(0)
      })

      const toggle = screen.getByTestId('toggle-crowdsec')
      expect(toggle).toBeChecked()
    })

    it('should show running PID when CrowdSec is running', async () => {
      vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(mockSecurityStatusAllEnabled)
      vi.mocked(crowdsecApi.statusCrowdsec).mockResolvedValue({ running: true, pid: 1234, lapi_ready: true })

      await renderSecurityPage()

      await waitFor(() => {
        expect(screen.getByText(/Running \(pid 1234\)/i)).toBeInTheDocument()
      })
    })
  })

  describe('SD-03: CrowdSec Card Disabled Status', () => {
    it('should show "Disabled" when crowdsec.enabled=false', async () => {
      vi.mocked(securityApi.getSecurityStatus).mockResolvedValue({
        ...mockSecurityStatusAllEnabled,
        crowdsec: { mode: 'disabled', api_url: '', enabled: false },
      })

      await renderSecurityPage()

      await waitFor(() => {
        expect(screen.getByText(/Cerberus Dashboard/i)).toBeInTheDocument()
      })

      const toggle = screen.getByTestId('toggle-crowdsec')
      expect(toggle).not.toBeChecked()
    })
  })

  describe('SD-04: WAF (Coraza) Card Status', () => {
    it('should show "Active" when waf.enabled=true', async () => {
      vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(mockSecurityStatusAllEnabled)

      await renderSecurityPage()

      await waitFor(() => {
        expect(screen.getByTestId('toggle-waf')).toBeChecked()
      })
    })

    it('should show "Disabled" when waf.enabled=false', async () => {
      vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(mockSecurityStatusMixed)

      await renderSecurityPage()

      await waitFor(() => {
        expect(screen.getByTestId('toggle-waf')).not.toBeChecked()
      })
    })
  })

  describe('SD-05: Rate Limiting Card Status', () => {
    it('should show badge and text when rate_limit.enabled=true', async () => {
      vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(mockSecurityStatusAllEnabled)

      await renderSecurityPage()

      await waitFor(() => {
        expect(screen.getByTestId('toggle-rate-limit')).toBeChecked()
        expect(screen.getByText(/● Active/)).toBeInTheDocument()
      })
    })

    it('should show "Disabled" badge when rate_limit.enabled=false', async () => {
      vi.mocked(securityApi.getSecurityStatus).mockResolvedValue({
        ...mockSecurityStatusAllEnabled,
        rate_limit: { enabled: false },
      })

      await renderSecurityPage()

      await waitFor(() => {
        expect(screen.getByTestId('toggle-rate-limit')).not.toBeChecked()
        expect(screen.getByText(/○ Disabled/)).toBeInTheDocument()
      })
    })
  })

  describe('SD-06: ACL Card Status', () => {
    it('should show "Active" when acl.enabled=true', async () => {
      vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(mockSecurityStatusAllEnabled)

      await renderSecurityPage()

      await waitFor(() => {
        expect(screen.getByTestId('toggle-acl')).toBeChecked()
      })
    })

    it('should show "Disabled" when acl.enabled=false', async () => {
      vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(mockSecurityStatusMixed)

      await renderSecurityPage()

      await waitFor(() => {
        expect(screen.getByTestId('toggle-acl')).not.toBeChecked()
      })
    })
  })

  describe('SD-07: Layer Indicators', () => {
    it('should display all layer indicators in correct order', async () => {
      vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(mockSecurityStatusAllEnabled)

      await renderSecurityPage()

      await waitFor(() => {
        expect(screen.getByText(/Cerberus Dashboard/i)).toBeInTheDocument()
      })

      // Verify each layer indicator is present
      expect(screen.getByText(/Layer 1: IP Reputation/i)).toBeInTheDocument()
      expect(screen.getByText(/Layer 2: Access Control/i)).toBeInTheDocument()
      expect(screen.getByText(/Layer 3: Request Inspection/i)).toBeInTheDocument()
      expect(screen.getByText(/Layer 4: Volume Control/i)).toBeInTheDocument()
    })
  })

  describe('SD-08: Threat Protection Summaries', () => {
    it('should display threat protection descriptions for each card', async () => {
      vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(mockSecurityStatusAllEnabled)
      // CrowdSec must be running to show threat protection descriptions
      vi.mocked(crowdsecApi.statusCrowdsec).mockResolvedValue({ running: true, pid: 1234, lapi_ready: true })

      await renderSecurityPage()

      await waitFor(() => {
        expect(screen.getByText(/Cerberus Dashboard/i)).toBeInTheDocument()
      })

      // Verify threat protection descriptions
      expect(screen.getByText(/Known attackers, botnets/i)).toBeInTheDocument()
      expect(screen.getByText(/Unauthorized IPs, geo-based attacks/i)).toBeInTheDocument()
      expect(screen.getByText(/SQL injection, XSS, RCE/i)).toBeInTheDocument()
      expect(screen.getByText(/DDoS attacks, credential stuffing/i)).toBeInTheDocument()
    })
  })

  describe('SD-09: Card Order (Pipeline Sequence)', () => {
    it('should maintain card order: CrowdSec → ACL → WAF → Rate Limiting', async () => {
      vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(mockSecurityStatusAllEnabled)

      await renderSecurityPage()

      await waitFor(() => {
        expect(screen.getByText(/Cerberus Dashboard/i)).toBeInTheDocument()
      })

      // Get all card headings
      const cards = screen.getAllByRole('heading', { level: 3 })
      const cardNames = cards.map((card: HTMLElement) => card.textContent)

      // Verify pipeline order: CrowdSec (Layer 1) → ACL (Layer 2) → Coraza (Layer 3) → Rate Limiting (Layer 4) + Security Access Logs
      expect(cardNames).toEqual(['CrowdSec', 'Access Control', 'Coraza', 'Rate Limiting', 'Security Access Logs'])
    })

    it('should maintain card order even after toggle', async () => {
      const user = userEvent.setup()
      vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(mockSecurityStatusAllEnabled)
      vi.mocked(settingsApi.updateSetting).mockResolvedValue()

      await renderSecurityPage()

      await waitFor(() => {
        expect(screen.getByTestId('toggle-waf')).toBeInTheDocument()
      })

      // Toggle WAF off
      await user.click(screen.getByTestId('toggle-waf'))

      // Cards should still be in order
      const cards = screen.getAllByRole('heading', { level: 3 })
      const cardNames = cards.map((card: HTMLElement) => card.textContent)
      expect(cardNames).toEqual(['CrowdSec', 'Access Control', 'Coraza', 'Rate Limiting', 'Security Access Logs'])
    })
  })

  describe('SD-10: Toggle Switches Disabled When Cerberus Off', () => {
    it('should disable all service toggles when Cerberus is disabled', async () => {
      vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(mockSecurityStatusCerberusDisabled)

      await renderSecurityPage()

      await waitFor(() => {
        expect(screen.getByText(/Cerberus Disabled/i)).toBeInTheDocument()
      })

      // All toggles should be disabled
      expect(screen.getByTestId('toggle-crowdsec')).toBeDisabled()
      expect(screen.getByTestId('toggle-waf')).toBeDisabled()
      expect(screen.getByTestId('toggle-acl')).toBeDisabled()
      expect(screen.getByTestId('toggle-rate-limit')).toBeDisabled()
    })

    it('should enable toggles when Cerberus is enabled', async () => {
      vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(mockSecurityStatusAllEnabled)

      await renderSecurityPage()

      await waitFor(() => {
        expect(screen.getByText(/Cerberus Dashboard/i)).toBeInTheDocument()
      })

      // All toggles should be enabled
      expect(screen.getByTestId('toggle-crowdsec')).not.toBeDisabled()
      expect(screen.getByTestId('toggle-waf')).not.toBeDisabled()
      expect(screen.getByTestId('toggle-acl')).not.toBeDisabled()
      expect(screen.getByTestId('toggle-rate-limit')).not.toBeDisabled()
    })
  })
})
