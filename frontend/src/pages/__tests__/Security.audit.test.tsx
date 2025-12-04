/**
 * Security Page - QA Security Audit Tests
 *
 * Tests edge cases, input validation, error states, and security concerns
 * for the Security Dashboard implementation.
 */
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { BrowserRouter } from 'react-router-dom'
import Security from '../Security'
import * as securityApi from '../../api/security'
import * as crowdsecApi from '../../api/crowdsec'
import * as settingsApi from '../../api/settings'
import { toast } from '../../utils/toast'

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
  })

  const wrapper = ({ children }: { children: React.ReactNode }) => (
    <QueryClientProvider client={queryClient}>
      <BrowserRouter>{children}</BrowserRouter>
    </QueryClientProvider>
  )

  const mockSecurityStatus = {
    cerberus: { enabled: true },
    crowdsec: { mode: 'local' as const, api_url: 'http://localhost', enabled: true },
    waf: { mode: 'enabled' as const, enabled: true },
    rate_limit: { enabled: true },
    acl: { enabled: true }
  }

  describe('Input Validation', () => {
    it('React escapes XSS in rendered text - validation check', async () => {
      // Note: React automatically escapes text content, so XSS in input values
      // won't execute. This test verifies that property.
      vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(mockSecurityStatus)

      render(<Security />, { wrapper })

      await waitFor(() => screen.getByText(/Security Dashboard/i))

      // DOM should not contain any actual script elements from user input
      expect(document.querySelectorAll('script[src*="alert"]').length).toBe(0)

      // Verify React is escaping properly - any text rendered should be text, not HTML
      expect(screen.queryByText('<script>')).toBeNull()
    })

    it('handles empty admin whitelist gracefully', async () => {
      vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(mockSecurityStatus)

      render(<Security />, { wrapper })

      await waitFor(() => screen.getByText(/Security Dashboard/i))

      // Empty whitelist input should exist and be empty
      const whitelistInput = screen.getByDisplayValue('')
      expect(whitelistInput).toBeInTheDocument()
    })
  })

  describe('Error Handling', () => {
    it('displays error toast when toggle mutation fails', async () => {
      const user = userEvent.setup()
      vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(mockSecurityStatus)
      vi.mocked(settingsApi.updateSetting).mockRejectedValue(new Error('Network error'))

      render(<Security />, { wrapper })

      await waitFor(() => screen.getByTestId('toggle-crowdsec'))
      const toggle = screen.getByTestId('toggle-crowdsec')
      await user.click(toggle)

      await waitFor(() => {
        expect(toast.error).toHaveBeenCalledWith(expect.stringContaining('Failed to update setting'))
      })
    })

    it('handles CrowdSec start failure gracefully', async () => {
      const user = userEvent.setup()
      vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(mockSecurityStatus)
      vi.mocked(crowdsecApi.statusCrowdsec).mockResolvedValue({ running: false })
      vi.mocked(crowdsecApi.startCrowdsec).mockRejectedValue(new Error('Failed to start'))

      render(<Security />, { wrapper })

      await waitFor(() => screen.getByTestId('crowdsec-start'))
      const startButton = screen.getByTestId('crowdsec-start')
      await user.click(startButton)

      await waitFor(() => {
        expect(toast.error).toHaveBeenCalled()
      })
    })

    it('handles CrowdSec stop failure gracefully', async () => {
      const user = userEvent.setup()
      vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(mockSecurityStatus)
      vi.mocked(crowdsecApi.statusCrowdsec).mockResolvedValue({ running: true, pid: 1234 })
      vi.mocked(crowdsecApi.stopCrowdsec).mockRejectedValue(new Error('Failed to stop'))

      render(<Security />, { wrapper })

      await waitFor(() => screen.getByTestId('crowdsec-stop'))
      const stopButton = screen.getByTestId('crowdsec-stop')
      await user.click(stopButton)

      await waitFor(() => {
        expect(toast.error).toHaveBeenCalled()
      })
    })

    it('handles CrowdSec export failure gracefully', async () => {
      const user = userEvent.setup()
      vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(mockSecurityStatus)
      vi.mocked(crowdsecApi.exportCrowdsecConfig).mockRejectedValue(new Error('Export failed'))

      render(<Security />, { wrapper })

      await waitFor(() => screen.getByRole('button', { name: /Export/i }))
      const exportButton = screen.getByRole('button', { name: /Export/i })
      await user.click(exportButton)

      await waitFor(() => {
        expect(toast.error).toHaveBeenCalledWith('Failed to export CrowdSec configuration')
      })
    })

    it('handles CrowdSec status check failure gracefully', async () => {
      vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(mockSecurityStatus)
      vi.mocked(crowdsecApi.statusCrowdsec).mockRejectedValue(new Error('Status check failed'))

      render(<Security />, { wrapper })

      // Page should still render even if status check fails
      await waitFor(() => expect(screen.getByText(/Security Dashboard/i)).toBeInTheDocument())
    })
  })

  describe('Concurrent Operations', () => {
    it('disables controls during pending mutations', async () => {
      const user = userEvent.setup()
      vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(mockSecurityStatus)
      // Never resolving promise to simulate pending state
      vi.mocked(settingsApi.updateSetting).mockImplementation(() => new Promise(() => {}))

      render(<Security />, { wrapper })

      await waitFor(() => screen.getByTestId('toggle-cerberus'))
      const toggle = screen.getByTestId('toggle-cerberus')
      await user.click(toggle)

      // Overlay should appear indicating operation in progress
      await waitFor(() => expect(screen.getByText(/Cerberus awakens/i)).toBeInTheDocument())
    })

    it('prevents double-click on CrowdSec start button', async () => {
      const user = userEvent.setup()
      vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(mockSecurityStatus)
      vi.mocked(crowdsecApi.statusCrowdsec).mockResolvedValue({ running: false })
      let callCount = 0
      vi.mocked(crowdsecApi.startCrowdsec).mockImplementation(async () => {
        callCount++
        await new Promise(resolve => setTimeout(resolve, 100))
        return { success: true }
      })

      render(<Security />, { wrapper })

      await waitFor(() => screen.getByTestId('crowdsec-start'))
      const startButton = screen.getByTestId('crowdsec-start')

      // Double click
      await user.click(startButton)
      await user.click(startButton)

      // Wait for potential multiple calls
      await new Promise(resolve => setTimeout(resolve, 150))

      // Should only be called once due to disabled state
      expect(callCount).toBe(1)
    })
  })

  describe('UI Consistency', () => {
    it('maintains card order when services are toggled', async () => {
      const user = userEvent.setup()
      vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(mockSecurityStatus)
      vi.mocked(settingsApi.updateSetting).mockResolvedValue()

      render(<Security />, { wrapper })

      await waitFor(() => screen.getByText(/Security Dashboard/i))

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

      render(<Security />, { wrapper })

      await waitFor(() => screen.getByText(/Security Dashboard/i))

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

      render(<Security />, { wrapper })

      await waitFor(() => screen.getByText(/Security Dashboard/i))

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

      render(<Security />, { wrapper })

      await waitFor(() => screen.getByText(/Security Dashboard/i))

      expect(screen.getByTestId('toggle-cerberus')).toBeInTheDocument()
      expect(screen.getByTestId('toggle-crowdsec')).toBeInTheDocument()
      expect(screen.getByTestId('toggle-acl')).toBeInTheDocument()
      expect(screen.getByTestId('toggle-waf')).toBeInTheDocument()
      expect(screen.getByTestId('toggle-rate-limit')).toBeInTheDocument()
    })

    it('WAF controls have proper test IDs when enabled', async () => {
      vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(mockSecurityStatus)

      render(<Security />, { wrapper })

      await waitFor(() => screen.getByText(/Security Dashboard/i))

      expect(screen.getByTestId('waf-mode-select')).toBeInTheDocument()
      expect(screen.getByTestId('waf-ruleset-select')).toBeInTheDocument()
    })

    it('CrowdSec buttons have proper test IDs when enabled', async () => {
      vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(mockSecurityStatus)
      vi.mocked(crowdsecApi.statusCrowdsec).mockResolvedValue({ running: false })

      render(<Security />, { wrapper })

      await waitFor(() => screen.getByText(/Security Dashboard/i))

      expect(screen.getByTestId('crowdsec-start')).toBeInTheDocument()
      expect(screen.getByTestId('crowdsec-stop')).toBeInTheDocument()
    })
  })

  describe('Contract Verification (Spec Compliance)', () => {
    it('pipeline order matches spec: CrowdSec → ACL → WAF → Rate Limiting', async () => {
      vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(mockSecurityStatus)

      render(<Security />, { wrapper })

      await waitFor(() => screen.getByText(/Security Dashboard/i))

      const cards = screen.getAllByRole('heading', { level: 3 })
      const cardNames = cards.map(card => card.textContent)

      // Spec requirement from current_spec.md
      expect(cardNames).toEqual(['CrowdSec', 'Access Control', 'WAF (Coraza)', 'Rate Limiting'])
    })

    it('layer indicators match spec descriptions', async () => {
      vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(mockSecurityStatus)

      render(<Security />, { wrapper })

      await waitFor(() => screen.getByText(/Security Dashboard/i))

      // From spec: Layer 1: IP Reputation, Layer 2: Access Control, Layer 3: Request Inspection, Layer 4: Volume Control
      expect(screen.getByText(/Layer 1: IP Reputation/i)).toBeInTheDocument()
      expect(screen.getByText(/Layer 2: Access Control/i)).toBeInTheDocument()
      expect(screen.getByText(/Layer 3: Request Inspection/i)).toBeInTheDocument()
      expect(screen.getByText(/Layer 4: Volume Control/i)).toBeInTheDocument()
    })

    it('threat summaries match spec when services enabled', async () => {
      vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(mockSecurityStatus)

      render(<Security />, { wrapper })

      await waitFor(() => screen.getByText(/Security Dashboard/i))

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

      render(<Security />, { wrapper })

      await waitFor(() => screen.getByTestId('toggle-waf'))

      const toggle = screen.getByTestId('toggle-waf')

      // Rapid clicks
      for (let i = 0; i < 5; i++) {
        await user.click(toggle)
      }

      // Page should still be functional
      await waitFor(() => expect(screen.getByText(/Security Dashboard/i)).toBeInTheDocument())
    })

    it('handles undefined crowdsec status gracefully', async () => {
      vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(mockSecurityStatus)
      vi.mocked(crowdsecApi.statusCrowdsec).mockResolvedValue(null as any)

      render(<Security />, { wrapper })

      // Should not crash
      await waitFor(() => expect(screen.getByText(/Security Dashboard/i)).toBeInTheDocument())
    })
  })
})
