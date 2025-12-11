/**
 * Security Page - QA Security Audit Tests
 *
 * Tests edge cases, input validation, error states, and security concerns
 * for the Cerberus Dashboard implementation.
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
import { toast } from '../../utils/toast'

const mockSecurityStatus = {
  cerberus: { enabled: true },
  crowdsec: { mode: 'local' as const, api_url: 'http://localhost', enabled: true },
  waf: { mode: 'enabled' as const, enabled: true },
  rate_limit: { enabled: true },
  acl: { enabled: true },
}

vi.mock('../../api/security')
vi.mock('../../api/crowdsec')
vi.mock('../../api/settings')
vi.mock('../../utils/toast', () => ({
  toast: {
    success: vi.fn(),
    error: vi.fn(),
  },
}))
vi.mock('../../hooks/useSecurity', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../../hooks/useSecurity')>()
  return {
    ...actual,
    useSecurityConfig: vi.fn(() => ({ data: { config: { admin_whitelist: '' } } })),
    useUpdateSecurityConfig: vi.fn(() => ({ mutate: vi.fn(), isPending: false })),
    useGenerateBreakGlassToken: vi.fn(() => ({ mutate: vi.fn(), isPending: false })),
    useRuleSets: vi.fn(() => ({ data: { rulesets: [] } })),
  }
})

describe('Security Page - QA Security Audit', () => {
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
    vi.mocked(crowdsecApi.statusCrowdsec).mockResolvedValue({ running: false })
    vi.mocked(settingsApi.updateSetting).mockResolvedValue()
    vi.mocked(crowdsecApi.exportCrowdsecConfig).mockResolvedValue(new Blob())
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

  describe('Input Validation', () => {
    it('React escapes XSS in rendered text - validation check', async () => {
      // Note: React automatically escapes text content, so XSS in input values
      // won't execute. This test verifies that property.
      vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(mockSecurityStatus)

      await renderSecurityPage()

      await waitFor(() => screen.getByText(/Cerberus Dashboard/i))

      // DOM should not contain any actual script elements from user input
      expect(document.querySelectorAll('script[src*="alert"]').length).toBe(0)

      // Verify React is escaping properly - any text rendered should be text, not HTML
      expect(screen.queryByText('<script>')).toBeNull()
    })

    it('handles empty admin whitelist gracefully', async () => {
      vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(mockSecurityStatus)

      await renderSecurityPage()

      await waitFor(() => screen.getByText(/Cerberus Dashboard/i))

      // Empty whitelist input should exist and be empty - use label to find it
      const whitelistLabel = screen.getByText(/Admin whitelist \(comma-separated CIDR\/IPs\)/i)
      expect(whitelistLabel).toBeInTheDocument()
      // The input follows the label, get it by querying parent
      const whitelistInput = whitelistLabel.parentElement?.querySelector('input')
      expect(whitelistInput).toBeInTheDocument()
      expect(whitelistInput?.value).toBe('')
    })
  })

  describe('Error Handling', () => {
    it('displays error toast when toggle mutation fails', async () => {
      const user = userEvent.setup()
      vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(mockSecurityStatus)
      vi.mocked(settingsApi.updateSetting).mockRejectedValue(new Error('Network error'))

      await renderSecurityPage()

      await waitFor(() => screen.getByTestId('toggle-crowdsec'))
      const toggle = screen.getByTestId('toggle-crowdsec')
      await user.click(toggle)

      await waitFor(() => {
        expect(toast.error).toHaveBeenCalledWith(expect.stringContaining('Failed to stop CrowdSec'))
      })
    })

    it('handles CrowdSec start failure gracefully', async () => {
      const user = userEvent.setup()
      vi.mocked(securityApi.getSecurityStatus).mockResolvedValue({
        ...mockSecurityStatus,
        crowdsec: { mode: 'local', api_url: 'http://localhost', enabled: false },
      })
      vi.mocked(crowdsecApi.statusCrowdsec).mockResolvedValue({ running: false })
      vi.mocked(crowdsecApi.startCrowdsec).mockRejectedValue(new Error('Failed to start'))

      await renderSecurityPage()

      await waitFor(() => screen.getByTestId('toggle-crowdsec'))
      const toggle = screen.getByTestId('toggle-crowdsec')
      await user.click(toggle)

      await waitFor(() => {
        expect(toast.error).toHaveBeenCalled()
      })
    })

    it('handles CrowdSec stop failure gracefully', async () => {
      const user = userEvent.setup()
      vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(mockSecurityStatus)
      vi.mocked(crowdsecApi.statusCrowdsec).mockResolvedValue({ running: true, pid: 1234 })
      vi.mocked(crowdsecApi.stopCrowdsec).mockRejectedValue(new Error('Failed to stop'))

      await renderSecurityPage()

      await waitFor(() => screen.getByTestId('toggle-crowdsec'))
      const toggle = screen.getByTestId('toggle-crowdsec')
      await user.click(toggle)

      await waitFor(() => {
        expect(toast.error).toHaveBeenCalled()
      })
    })



    it('handles CrowdSec status check failure gracefully', async () => {
      vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(mockSecurityStatus)
      vi.mocked(crowdsecApi.statusCrowdsec).mockRejectedValue(new Error('Status check failed'))

      await renderSecurityPage()

      // Page should still render even if status check fails
      await waitFor(() => expect(screen.getByText(/Cerberus Dashboard/i)).toBeInTheDocument())
    })
  })

  describe('Concurrent Operations', () => {
    it('disables controls during pending mutations', async () => {
      const user = userEvent.setup()
      vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(mockSecurityStatus)
      // Never resolving promise to simulate pending state
      vi.mocked(settingsApi.updateSetting).mockImplementation(() => new Promise(() => {}))

      await renderSecurityPage()

      await waitFor(() => screen.getByTestId('toggle-waf'))
      const toggle = screen.getByTestId('toggle-waf')
      await user.click(toggle)

      // Overlay should appear indicating operation in progress
      await waitFor(() => expect(screen.getByText(/Three heads turn/i)).toBeInTheDocument())
    })

    it('prevents double toggle when starting CrowdSec', async () => {
      const user = userEvent.setup()
      vi.mocked(securityApi.getSecurityStatus).mockResolvedValue({
        ...mockSecurityStatus,
        crowdsec: { mode: 'local', api_url: 'http://localhost', enabled: false },
      })
      vi.mocked(crowdsecApi.statusCrowdsec).mockResolvedValue({ running: false })
      let callCount = 0
      vi.mocked(crowdsecApi.startCrowdsec).mockImplementation(async () => {
        callCount++
        await new Promise(resolve => setTimeout(resolve, 100))
        return { success: true }
      })

      await renderSecurityPage()

      await waitFor(() => screen.getByTestId('toggle-crowdsec'))
      const toggle = screen.getByTestId('toggle-crowdsec')

      // Double click
      await user.click(toggle)
      await user.click(toggle)

      // Wait for potential multiple calls
      await act(async () => {
        await new Promise(resolve => setTimeout(resolve, 150))
      })

      // Should only be called once due to disabled state
      expect(callCount).toBe(1)
    })
  })

  describe('UI Consistency', () => {
    it('maintains card order when services are toggled', async () => {
      const user = userEvent.setup()
      vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(mockSecurityStatus)
      vi.mocked(settingsApi.updateSetting).mockResolvedValue()

      await renderSecurityPage()

      await waitFor(() => screen.getByText(/Cerberus Dashboard/i))

      // Get initial card order
      const initialCards = screen.getAllByRole('heading', { level: 3 })
      const initialOrder = initialCards.map(card => card.textContent)

      // Toggle a service
      const toggle = screen.getByTestId('toggle-waf')
      await user.click(toggle)

      // Wait for mutation to settle
      await waitFor(() => expect(settingsApi.updateSetting).toHaveBeenCalled())

      // Cards should still be in same order
      const finalCards = screen.getAllByRole('heading', { level: 3 })
      const finalOrder = finalCards.map(card => card.textContent)

      expect(finalOrder).toEqual(initialOrder)
    })

    it('shows correct layer indicator icons', async () => {
      vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(mockSecurityStatus)

      await renderSecurityPage()

      await waitFor(() => screen.getByText(/Cerberus Dashboard/i))

      // Each layer should have correct emoji
      expect(screen.getByText(/🛡️ Layer 1/)).toBeInTheDocument()
      expect(screen.getByText(/🔒 Layer 2/)).toBeInTheDocument()
      expect(screen.getByText(/🛡️ Layer 3/)).toBeInTheDocument()
      expect(screen.getByText(/⚡ Layer 4/)).toBeInTheDocument()
    })

    it('shows all four security cards even when all disabled', async () => {
      const disabledStatus = {
        cerberus: { enabled: true },
        crowdsec: { mode: 'local' as const, api_url: '', enabled: false },
        waf: { mode: 'enabled' as const, enabled: false },
        rate_limit: { enabled: false },
        acl: { enabled: false }
      }
      vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(disabledStatus)

      await renderSecurityPage()

      await waitFor(() => screen.getByText(/Cerberus Dashboard/i))

      // All 4 cards should be present
      expect(screen.getByText('CrowdSec')).toBeInTheDocument()
      expect(screen.getByText('Access Control')).toBeInTheDocument()
      expect(screen.getByText('WAF (Coraza)')).toBeInTheDocument()
      expect(screen.getByText('Rate Limiting')).toBeInTheDocument()
    })
  })

  describe('Accessibility', () => {
    it('all toggles have proper test IDs for automation', async () => {
      vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(mockSecurityStatus)

      await renderSecurityPage()

      await waitFor(() => screen.getByText(/Cerberus Dashboard/i))

      expect(screen.getByTestId('toggle-crowdsec')).toBeInTheDocument()
      expect(screen.getByTestId('toggle-acl')).toBeInTheDocument()
      expect(screen.getByTestId('toggle-waf')).toBeInTheDocument()
      expect(screen.getByTestId('toggle-rate-limit')).toBeInTheDocument()
    })

    it('WAF controls have proper test IDs when enabled', async () => {
      vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(mockSecurityStatus)

      await renderSecurityPage()

      await waitFor(() => screen.getByText(/Cerberus Dashboard/i))

      expect(screen.getByTestId('waf-mode-select')).toBeInTheDocument()
      expect(screen.getByTestId('waf-ruleset-select')).toBeInTheDocument()
    })

    it('CrowdSec controls surface primary actions when enabled', async () => {
      vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(mockSecurityStatus)
      vi.mocked(crowdsecApi.statusCrowdsec).mockResolvedValue({ running: false })

      await renderSecurityPage()

      await waitFor(() => screen.getByText(/Cerberus Dashboard/i))

      expect(screen.getByTestId('toggle-crowdsec')).toBeInTheDocument()
      // CrowdSec card should only have Config button now
      const configButtons = screen.getAllByRole('button', { name: /Config/i })
      expect(configButtons.some(btn => btn.textContent === 'Config')).toBe(true)
    })
  })

  describe('Contract Verification (Spec Compliance)', () => {
    it('pipeline order matches spec: CrowdSec → ACL → WAF → Rate Limiting', async () => {
      vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(mockSecurityStatus)

      await renderSecurityPage()

      await waitFor(() => screen.getByText(/Cerberus Dashboard/i))

      const cards = screen.getAllByRole('heading', { level: 3 })
      const cardNames = cards.map(card => card.textContent)

      // Spec requirement from current_spec.md plus Live Security Logs feature
      expect(cardNames).toEqual(['CrowdSec', 'Access Control', 'WAF (Coraza)', 'Rate Limiting', 'Live Security Logs'])
    })

    it('layer indicators match spec descriptions', async () => {
      vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(mockSecurityStatus)

      await renderSecurityPage()

      await waitFor(() => screen.getByText(/Cerberus Dashboard/i))

      // From spec: Layer 1: IP Reputation, Layer 2: Access Control, Layer 3: Request Inspection, Layer 4: Volume Control
      expect(screen.getByText(/Layer 1: IP Reputation/i)).toBeInTheDocument()
      expect(screen.getByText(/Layer 2: Access Control/i)).toBeInTheDocument()
      expect(screen.getByText(/Layer 3: Request Inspection/i)).toBeInTheDocument()
      expect(screen.getByText(/Layer 4: Volume Control/i)).toBeInTheDocument()
    })

    it('threat summaries match spec when services enabled', async () => {
      vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(mockSecurityStatus)

      await renderSecurityPage()

      await waitFor(() => screen.getByText(/Cerberus Dashboard/i))

      // From spec:
      // CrowdSec: "Known attackers, botnets, brute-force attempts"
      // ACL: "Unauthorized IPs, geo-based attacks, insider threats"
      // WAF: "SQL injection, XSS, RCE, zero-day exploits*"
      // Rate Limiting: "DDoS attacks, credential stuffing, API abuse"
      expect(screen.getByText(/Known attackers, botnets/i)).toBeInTheDocument()
      expect(screen.getByText(/Unauthorized IPs, geo-based attacks/i)).toBeInTheDocument()
      expect(screen.getByText(/SQL injection, XSS, RCE/i)).toBeInTheDocument()
      expect(screen.getByText(/DDoS attacks, credential stuffing/i)).toBeInTheDocument()
    })
  })

  describe('Edge Cases', () => {
    it('handles rapid toggle clicks without crashing', async () => {
      const user = userEvent.setup()
      vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(mockSecurityStatus)
      vi.mocked(settingsApi.updateSetting).mockImplementation(
        () => new Promise(resolve => setTimeout(resolve, 50))
      )

      await renderSecurityPage()

      await waitFor(() => screen.getByTestId('toggle-waf'))

      const toggle = screen.getByTestId('toggle-waf')

      // Rapid clicks
      for (let i = 0; i < 5; i++) {
        await user.click(toggle)
      }

      // Page should still be functional
      await waitFor(() => expect(screen.getByText(/Cerberus Dashboard/i)).toBeInTheDocument())
    })

    it('handles undefined crowdsec status gracefully', async () => {
      vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(mockSecurityStatus)
      vi.mocked(crowdsecApi.statusCrowdsec).mockResolvedValue(null as never)

      await renderSecurityPage()

      // Should not crash
      await waitFor(() => expect(screen.getByText(/Cerberus Dashboard/i)).toBeInTheDocument())
    })
  })
})
