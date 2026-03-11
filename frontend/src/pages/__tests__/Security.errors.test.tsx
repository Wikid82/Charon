/**
 * Security Error Handling Tests
 * Test IDs: EH-01 through EH-10
 *
 * Tests error messages on API failures, toast notifications on mutation errors,
 * and optimistic update rollback.
 */
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { act, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { BrowserRouter } from 'react-router-dom'
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'

import * as crowdsecApi from '../../api/crowdsec'
import * as securityApi from '../../api/security'
import * as settingsApi from '../../api/settings'
import { toast } from '../../utils/toast'
import Security from '../Security'

import type * as useSecurity from '../../hooks/useSecurity'

vi.mock('../../api/security')
vi.mock('../../api/crowdsec')
vi.mock('../../api/settings')
vi.mock('../../utils/toast', () => ({
  toast: {
    success: vi.fn(),
    error: vi.fn(),
    info: vi.fn(),
    warning: vi.fn(),
  },
}))
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
describe.skip('Security Error Handling Tests', () => {
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

  afterEach(() => {
    vi.clearAllMocks()
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

  describe('EH-01: Failed Security Status Fetch Shows Error', () => {
    it('should show "Failed to load security configuration" when API fails', async () => {
      vi.mocked(securityApi.getSecurityStatus).mockRejectedValue(new Error('Network error'))

      await renderSecurityPage()

      await waitFor(() => {
        expect(screen.getByText(/Failed to load security configuration/i)).toBeInTheDocument()
      })
    })
  })

  describe('EH-02: Toggle Mutation Failure Shows Toast', () => {
    it('should call toast.error() when toggle mutation fails', async () => {
      const user = userEvent.setup()
      vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(mockSecurityStatusAllEnabled)
      vi.mocked(settingsApi.updateSetting).mockRejectedValue(new Error('Permission denied'))

      await renderSecurityPage()

      await waitFor(() => screen.getByTestId('toggle-waf'))
      await user.click(screen.getByTestId('toggle-waf'))

      await waitFor(() => {
        expect(toast.error).toHaveBeenCalledWith(expect.stringContaining('Failed to update setting'))
      })
    })
  })

  describe('EH-03: CrowdSec Start Failure Shows Specific Toast', () => {
    it('should show "Failed to start CrowdSec: [message]" on start failure', async () => {
      const user = userEvent.setup()
      vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(mockSecurityStatusCrowdsecDisabled)
      vi.mocked(crowdsecApi.statusCrowdsec).mockResolvedValue({ running: false, pid: 0, lapi_ready: false })
      vi.mocked(settingsApi.updateSetting).mockResolvedValue()
      vi.mocked(crowdsecApi.startCrowdsec).mockRejectedValue(new Error('Service unavailable'))

      await renderSecurityPage()

      await waitFor(() => screen.getByTestId('toggle-crowdsec'))
      await user.click(screen.getByTestId('toggle-crowdsec'))

      await waitFor(() => {
        expect(toast.error).toHaveBeenCalledWith(expect.stringContaining('Failed to start CrowdSec'))
      })
      expect(toast.error).toHaveBeenCalledWith(expect.stringContaining('Service unavailable'))
    })
  })

  describe('EH-04: CrowdSec Stop Failure Shows Specific Toast', () => {
    it('should show "Failed to stop CrowdSec: [message]" on stop failure', async () => {
      const user = userEvent.setup()
      vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(mockSecurityStatusAllEnabled)
      vi.mocked(crowdsecApi.statusCrowdsec).mockResolvedValue({ running: true, pid: 1234, lapi_ready: true })
      vi.mocked(settingsApi.updateSetting).mockResolvedValue()
      vi.mocked(crowdsecApi.stopCrowdsec).mockRejectedValue(new Error('Process locked'))

      await renderSecurityPage()

      await waitFor(() => screen.getByTestId('toggle-crowdsec'))
      await user.click(screen.getByTestId('toggle-crowdsec'))

      await waitFor(() => {
        expect(toast.error).toHaveBeenCalledWith(expect.stringContaining('Failed to stop CrowdSec'))
      })
      expect(toast.error).toHaveBeenCalledWith(expect.stringContaining('Process locked'))
    })
  })

  describe('EH-05: WAF Toggle Failure Shows Error', () => {
    it('should show error toast when WAF toggle fails', async () => {
      const user = userEvent.setup()
      vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(mockSecurityStatusAllEnabled)
      vi.mocked(settingsApi.updateSetting).mockRejectedValue(new Error('WAF configuration error'))

      await renderSecurityPage()

      await waitFor(() => screen.getByTestId('toggle-waf'))
      await user.click(screen.getByTestId('toggle-waf'))

      await waitFor(() => {
        expect(toast.error).toHaveBeenCalledWith(expect.stringContaining('Failed to update setting'))
      })
    })
  })

  describe('EH-06: Rate Limiting Update Failure Shows Toast', () => {
    it('should show error toast when rate limiting toggle fails', async () => {
      const user = userEvent.setup()
      vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(mockSecurityStatusAllEnabled)
      vi.mocked(settingsApi.updateSetting).mockRejectedValue(new Error('Rate limit config error'))

      await renderSecurityPage()

      await waitFor(() => screen.getByTestId('toggle-rate-limit'))
      await user.click(screen.getByTestId('toggle-rate-limit'))

      await waitFor(() => {
        expect(toast.error).toHaveBeenCalledWith(expect.stringContaining('Failed to update setting'))
      })
    })
  })

  describe('EH-07: Network Error Shows Generic Message', () => {
    it('should handle network errors gracefully', async () => {
      const user = userEvent.setup()
      vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(mockSecurityStatusAllEnabled)
      vi.mocked(settingsApi.updateSetting).mockRejectedValue(new Error('Network request failed'))

      await renderSecurityPage()

      await waitFor(() => screen.getByTestId('toggle-acl'))
      await user.click(screen.getByTestId('toggle-acl'))

      await waitFor(() => {
        expect(toast.error).toHaveBeenCalledWith(expect.stringContaining('Network request failed'))
      })
    })

    it('should handle non-Error objects gracefully', async () => {
      const user = userEvent.setup()
      vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(mockSecurityStatusAllEnabled)
      vi.mocked(settingsApi.updateSetting).mockRejectedValue('Unknown error string')

      await renderSecurityPage()

      await waitFor(() => screen.getByTestId('toggle-acl'))
      await user.click(screen.getByTestId('toggle-acl'))

      await waitFor(() => {
        expect(toast.error).toHaveBeenCalled()
      })
    })
  })

  describe('EH-08: ACL Toggle Failure Shows Error', () => {
    it('should show error when ACL toggle fails', async () => {
      const user = userEvent.setup()
      vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(mockSecurityStatusAllEnabled)
      vi.mocked(settingsApi.updateSetting).mockRejectedValue(new Error('ACL update failed'))

      await renderSecurityPage()

      await waitFor(() => screen.getByTestId('toggle-acl'))
      await user.click(screen.getByTestId('toggle-acl'))

      await waitFor(() => {
        expect(toast.error).toHaveBeenCalledWith(expect.stringContaining('Failed to update setting'))
      })
      expect(toast.error).toHaveBeenCalledWith(expect.stringContaining('ACL update failed'))
    })
  })

  describe('EH-09: Multiple Consecutive Failures Show Multiple Toasts', () => {
    it('should show separate toast for each failed operation', async () => {
      const user = userEvent.setup()
      vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(mockSecurityStatusAllEnabled)
      vi.mocked(settingsApi.updateSetting).mockRejectedValue(new Error('Server error'))

      await renderSecurityPage()

      // First failure
      await waitFor(() => screen.getByTestId('toggle-waf'))
      await user.click(screen.getByTestId('toggle-waf'))

      await waitFor(() => {
        expect(toast.error).toHaveBeenCalledTimes(1)
      })

      // Second failure
      await user.click(screen.getByTestId('toggle-acl'))

      await waitFor(() => {
        expect(toast.error).toHaveBeenCalledTimes(2)
      })
    })
  })

  describe('EH-10: Optimistic Update Reverts on Error', () => {
    it('should revert toggle state when mutation fails', async () => {
      const user = userEvent.setup()
      vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(mockSecurityStatusAllEnabled)
      vi.mocked(settingsApi.updateSetting).mockRejectedValue(new Error('Update failed'))

      await renderSecurityPage()

      await waitFor(() => screen.getByTestId('toggle-waf'))

      // WAF is initially enabled
      const toggle = screen.getByTestId('toggle-waf')
      expect(toggle).toBeChecked()

      // Click to disable - optimistic update will uncheck it
      await user.click(toggle)

      // Wait for error and rollback
      await waitFor(() => {
        expect(toast.error).toHaveBeenCalled()
      })

      // After rollback, the toggle should be back to checked (enabled)
      await waitFor(() => {
        expect(screen.getByTestId('toggle-waf')).toBeChecked()
      })
    })

    it('should revert CrowdSec state on start failure', async () => {
      const user = userEvent.setup()
      vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(mockSecurityStatusCrowdsecDisabled)
      vi.mocked(crowdsecApi.statusCrowdsec).mockResolvedValue({ running: false, pid: 0, lapi_ready: false })
      vi.mocked(settingsApi.updateSetting).mockResolvedValue()
      vi.mocked(crowdsecApi.startCrowdsec).mockRejectedValue(new Error('Start failed'))

      await renderSecurityPage()

      await waitFor(() => screen.getByTestId('toggle-crowdsec'))

      // CrowdSec is initially disabled
      const toggle = screen.getByTestId('toggle-crowdsec')
      expect(toggle).not.toBeChecked()

      // Click to enable
      await user.click(toggle)

      // Wait for error
      await waitFor(() => {
        expect(toast.error).toHaveBeenCalledWith(expect.stringContaining('Failed to start CrowdSec'))
      })

      // After rollback, toggle should be back to unchecked (disabled)
      await waitFor(() => {
        expect(screen.getByTestId('toggle-crowdsec')).not.toBeChecked()
      })
    })

    it('should revert CrowdSec state on stop failure', async () => {
      const user = userEvent.setup()
      vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(mockSecurityStatusAllEnabled)
      vi.mocked(crowdsecApi.statusCrowdsec).mockResolvedValue({ running: true, pid: 1234, lapi_ready: true })
      vi.mocked(settingsApi.updateSetting).mockResolvedValue()
      vi.mocked(crowdsecApi.stopCrowdsec).mockRejectedValue(new Error('Stop failed'))

      await renderSecurityPage()

      await waitFor(() => screen.getByTestId('toggle-crowdsec'))

      // CrowdSec is initially enabled
      const toggle = screen.getByTestId('toggle-crowdsec')
      expect(toggle).toBeChecked()

      // Click to disable
      await user.click(toggle)

      // Wait for error
      await waitFor(() => {
        expect(toast.error).toHaveBeenCalledWith(expect.stringContaining('Failed to stop CrowdSec'))
      })

      // After rollback, toggle should be back to checked (enabled)
      await waitFor(() => {
        expect(screen.getByTestId('toggle-crowdsec')).toBeChecked()
      })
    })
  })
})
