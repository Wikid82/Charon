/**
 * Security Page Functional Tests - LiveLogViewer Mocked
 *
 * These tests mock the LiveLogViewer component to avoid WebSocket issues
 * and focus on testing Security.tsx core functionality.
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
vi.mock('../../hooks/useNotifications', () => ({
  useSecurityNotificationSettings: vi.fn(() => ({
    data: {
      enabled: false,
      min_log_level: 'warn',
      notify_waf_blocks: true,
      notify_acl_denials: true,
      notify_rate_limit_hits: true,
      webhook_url: '',
      email_recipients: '',
    },
    isLoading: false,
  })),
  useUpdateSecurityNotificationSettings: vi.fn(() => ({
    mutate: vi.fn(),
    isPending: false,
  })),
}))

const securityTranslations: Record<string, string> = {
  'security.title': 'Security',
  'security.description': 'Configure security layers for your reverse proxy',
  'security.cerberusDashboard': 'Cerberus Dashboard',
  'security.cerberusActive': 'Active',
  'security.cerberusDisabled': 'Disabled',
  'security.cerberusReadyMessage': 'Cerberus is ready to protect your services',
  'security.cerberusDisabledMessage': 'Enable Cerberus in System Settings to activate security features',
  'security.featuresUnavailable': 'Security Features Unavailable',
  'security.featuresUnavailableMessage': 'Enable Cerberus in System Settings to use security features',
  'security.learnMore': 'Learn More',
  'security.adminWhitelist': 'Admin Whitelist',
  'security.adminWhitelistDescription': 'CIDRs that bypass security checks for admin access',
  'security.commaSeparatedCIDR': 'Comma-separated CIDRs (e.g., 192.168.1.0/24)',
  'security.generateToken': 'Generate Token',
  'security.generateTokenTooltip': 'Generate a one-time break-glass token for emergency access',
  'security.layer1': 'Layer 1',
  'security.layer2': 'Layer 2',
  'security.layer3': 'Layer 3',
  'security.layer4': 'Layer 4',
  'security.ids': 'IDS',
  'security.acl': 'ACL',
  'security.waf': 'WAF',
  'security.rate': 'Rate',
  'security.crowdsec': 'CrowdSec',
  'security.crowdsecDescription': 'IP Reputation',
  'security.crowdsecProtects': 'Blocks known attackers, botnets, and malicious IPs',
  'security.crowdsecDisabledDescription': 'Enable to block known malicious IPs',
  'security.accessControl': 'Access Control',
  'security.aclDescription': 'IP Allowlists/Blocklists',
  'security.aclProtects': 'Unauthorized IPs, geo-based attacks',
  'security.corazaWaf': 'Coraza WAF',
  'security.wafDescription': 'Request Inspection',
  'security.wafProtects': 'SQL injection, XSS, RCE',
  'security.wafDisabledDescription': 'Enable to inspect requests for threats',
  'security.rateLimiting': 'Rate Limiting',
  'security.rateLimitDescription': 'Volume Control',
  'security.rateLimitProtects': 'DDoS attacks, credential stuffing',
  'security.processStopped': 'Process stopped',
  'security.enableCerberusFirst': 'Enable Cerberus first',
  'security.toggleCrowdsec': 'Toggle CrowdSec',
  'security.toggleAcl': 'Toggle Access Control',
  'security.toggleWaf': 'Toggle WAF',
  'security.toggleRateLimit': 'Toggle Rate Limiting',
  'security.manageLists': 'Manage Lists',
  'security.auditLogs': 'Audit Logs',
  'security.notifications': 'Notifications',
  'security.threeHeadsTurn': 'Three heads turn',
  'security.cerberusConfigUpdating': 'Cerberus configuration updating',
  'security.summoningGuardian': 'Summoning the guardian',
  'security.crowdsecStarting': 'CrowdSec is starting',
  'security.guardianRests': 'Guardian rests',
  'security.crowdsecStopping': 'CrowdSec is stopping',
  'security.strengtheningGuard': 'Strengthening guard',
  'security.wardsActivating': 'Wards activating',
  'common.enabled': 'Enabled',
  'common.disabled': 'Disabled',
  'common.save': 'Save',
  'common.configure': 'Configure',
  'common.docs': 'Docs',
  'common.error': 'Error',
  'security.failedToLoadConfiguration': 'Failed to load security configuration',
}

// Mock i18n translation
vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string, options?: { pid?: number }) => {
      // Handle interpolation for runningPid
      if (key === 'security.runningPid' && options?.pid !== undefined) {
        return `Running (pid ${options.pid})`
      }
      return securityTranslations[key] || key
    },
  }),
}))

// Mock LiveLogViewer to avoid WebSocket issues
vi.mock('../../components/LiveLogViewer', () => ({
  LiveLogViewer: () => <div data-testid="live-log-viewer">Mocked Live Log Viewer</div>,
}))

vi.mock('../../components/SecurityNotificationSettingsModal', () => ({
  SecurityNotificationSettingsModal: () => null,
}))

vi.mock('../../components/CrowdSecKeyWarning', () => ({
  CrowdSecKeyWarning: () => null,
}))

// NOTE: CrowdSecBouncerKeyDisplay mock removed (moved to CrowdSecConfig page)

vi.mock('../../hooks/useSecurity', () => ({
  useSecurityConfig: vi.fn(() => ({ data: { config: { admin_whitelist: '10.0.0.0/8' } } })),
  useUpdateSecurityConfig: vi.fn(() => ({ mutate: vi.fn(), isPending: false })),
  useGenerateBreakGlassToken: vi.fn(() => ({ mutate: vi.fn(), isPending: false })),
}))

const mockSecurityStatusAllEnabled = {
  cerberus: { enabled: true },
  crowdsec: { mode: 'local' as const, api_url: 'http://localhost', enabled: true },
  waf: { mode: 'enabled' as const, enabled: true },
  rate_limit: { enabled: true, mode: 'enabled' as const },
  acl: { enabled: true },
}

const mockSecurityStatusCerberusDisabled = {
  cerberus: { enabled: false },
  crowdsec: { mode: 'disabled' as const, api_url: '', enabled: false },
  waf: { mode: 'disabled' as const, enabled: false },
  rate_limit: { enabled: false },
  acl: { enabled: false },
}

const mockSecurityStatusCrowdsecDisabled = {
  cerberus: { enabled: true },
  crowdsec: { mode: 'local' as const, api_url: 'http://localhost', enabled: false },
  waf: { mode: 'enabled' as const, enabled: true },
  rate_limit: { enabled: true },
  acl: { enabled: true },
}

describe('Security Page - Functional Tests', () => {
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

  describe('Page Loading States', () => {
    it('should show skeleton loading state initially', async () => {
      const deferredStatus: { resolve: (value: typeof mockSecurityStatusAllEnabled) => void } = {
        resolve: () => {
          throw new Error('Test setup failed: pending status resolver was not initialized')
        },
      }
      const pendingStatus = new Promise<typeof mockSecurityStatusAllEnabled>((resolve) => {
        deferredStatus.resolve = resolve
      })
      vi.mocked(securityApi.getSecurityStatus).mockReturnValue(pendingStatus)

      await renderSecurityPage()

      const skeletons = document.querySelectorAll('.animate-pulse')
      expect(skeletons.length).toBeGreaterThan(0)

      deferredStatus.resolve(mockSecurityStatusAllEnabled)
      await waitFor(() => {
        expect(screen.getByText(/Cerberus Dashboard/i)).toBeInTheDocument()
      })
    })

    it('should display error message when security status fails to load', async () => {
      vi.mocked(securityApi.getSecurityStatus).mockRejectedValue(new Error('API Error'))

      await renderSecurityPage()

      await waitFor(() => {
        expect(screen.getByText(/Failed to load security configuration/i)).toBeInTheDocument()
      })
    })
  })

  describe('Cerberus Dashboard Header', () => {
    it('should display Cerberus Dashboard title when loaded', async () => {
      vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(mockSecurityStatusAllEnabled)

      await renderSecurityPage()

      await waitFor(() => {
        expect(screen.getByText(/Cerberus Dashboard/i)).toBeInTheDocument()
      })
    })

    it('should show Active badge when Cerberus is enabled', async () => {
      vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(mockSecurityStatusAllEnabled)

      await renderSecurityPage()

      await waitFor(() => {
        // Translation key: cerberusActive = 'Active'
        expect(screen.getByText('Active')).toBeInTheDocument()
      })
    })

    it('should show Disabled badge when Cerberus is disabled', async () => {
      vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(mockSecurityStatusCerberusDisabled)

      await renderSecurityPage()

      await waitFor(() => {
        // Multiple badges show 'Disabled'
        const disabledBadges = screen.getAllByText('Disabled')
        expect(disabledBadges.length).toBeGreaterThanOrEqual(1)
      })
    })
  })

  describe('Cerberus Disabled Warning', () => {
    it('should show warning banner when Cerberus is disabled', async () => {
      vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(mockSecurityStatusCerberusDisabled)

      await renderSecurityPage()

      await waitFor(() => {
        expect(screen.getByText(/Security Features Unavailable/i)).toBeInTheDocument()
      })
    })

    it('should show Learn More button in warning banner', async () => {
      vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(mockSecurityStatusCerberusDisabled)

      await renderSecurityPage()

      await waitFor(() => {
        const learnMoreButton = screen.getByRole('button', { name: /Learn More/i })
        expect(learnMoreButton).toBeInTheDocument()
      })
    })

    it('should not show warning banner when Cerberus is enabled', async () => {
      vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(mockSecurityStatusAllEnabled)

      await renderSecurityPage()

      await waitFor(() => {
        expect(screen.getByText(/Cerberus Dashboard/i)).toBeInTheDocument()
      })
      expect(screen.queryByText(/Security Features Unavailable/i)).not.toBeInTheDocument()
    })
  })

  describe('Security Layer Cards', () => {
    it('should display all 4 security layer cards', async () => {
      vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(mockSecurityStatusAllEnabled)

      await renderSecurityPage()

      await waitFor(() => {
        expect(screen.getByText('Layer 1')).toBeInTheDocument()
        expect(screen.getByText('Layer 2')).toBeInTheDocument()
        expect(screen.getByText('Layer 3')).toBeInTheDocument()
        expect(screen.getByText('Layer 4')).toBeInTheDocument()
      })
    })

    it('should display CrowdSec card title', async () => {
      vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(mockSecurityStatusAllEnabled)

      await renderSecurityPage()

      await waitFor(() => {
        expect(screen.getByText('CrowdSec')).toBeInTheDocument()
      })
    })

    it('should display Access Control card title', async () => {
      vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(mockSecurityStatusAllEnabled)

      await renderSecurityPage()

      await waitFor(() => {
        expect(screen.getByText('Access Control')).toBeInTheDocument()
      })
    })

    it('should display Coraza WAF card title', async () => {
      vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(mockSecurityStatusAllEnabled)

      await renderSecurityPage()

      await waitFor(() => {
        expect(screen.getByText('Coraza WAF')).toBeInTheDocument()
      })
    })

    it('should display Rate Limiting card title', async () => {
      vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(mockSecurityStatusAllEnabled)

      await renderSecurityPage()

      await waitFor(() => {
        expect(screen.getByText('Rate Limiting')).toBeInTheDocument()
      })
    })
  })

  describe('Toggle Switches Disabled State', () => {
    it('should disable all toggles when Cerberus is disabled', async () => {
      vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(mockSecurityStatusCerberusDisabled)

      await renderSecurityPage()

      await waitFor(() => {
        expect(screen.getByTestId('toggle-crowdsec')).toBeDisabled()
        expect(screen.getByTestId('toggle-waf')).toBeDisabled()
        expect(screen.getByTestId('toggle-acl')).toBeDisabled()
        expect(screen.getByTestId('toggle-rate-limit')).toBeDisabled()
      })
    })

    it('should enable toggles when Cerberus is enabled', async () => {
      vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(mockSecurityStatusAllEnabled)

      await renderSecurityPage()

      await waitFor(() => {
        expect(screen.getByTestId('toggle-waf')).not.toBeDisabled()
        expect(screen.getByTestId('toggle-acl')).not.toBeDisabled()
        expect(screen.getByTestId('toggle-rate-limit')).not.toBeDisabled()
      })
    })
  })

  describe('Service Toggle Badges', () => {
    it('should show Enabled badges for enabled services', async () => {
      vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(mockSecurityStatusAllEnabled)

      await renderSecurityPage()

      await waitFor(() => {
        const enabledBadges = screen.getAllByText('Enabled')
        expect(enabledBadges.length).toBeGreaterThanOrEqual(3)
      })
    })

    it('should show Disabled badge for disabled CrowdSec', async () => {
      vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(mockSecurityStatusCrowdsecDisabled)

      await renderSecurityPage()

      await waitFor(() => {
        const disabledBadges = screen.getAllByText('Disabled')
        expect(disabledBadges.length).toBeGreaterThanOrEqual(1)
      })
    })
  })

  describe('Header Actions', () => {
    it('should render Audit Logs button', async () => {
      vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(mockSecurityStatusAllEnabled)

      await renderSecurityPage()

      await waitFor(() => {
        expect(screen.getByRole('button', { name: /Audit Logs/i })).toBeInTheDocument()
      })
    })

    it('should render Notifications button', async () => {
      vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(mockSecurityStatusAllEnabled)

      await renderSecurityPage()

      await waitFor(() => {
        expect(screen.getByRole('button', { name: /Notifications/i })).toBeInTheDocument()
      })
    })

    it('should render Docs button', async () => {
      vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(mockSecurityStatusAllEnabled)

      await renderSecurityPage()

      await waitFor(() => {
        expect(screen.getByRole('button', { name: /Docs/i })).toBeInTheDocument()
      })
    })

    it('should disable Notifications button when Cerberus is disabled', async () => {
      vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(mockSecurityStatusCerberusDisabled)

      await renderSecurityPage()

      await waitFor(() => {
        expect(screen.getByRole('button', { name: /Notifications/i })).toBeDisabled()
      })
    })
  })

  // NOTE: CrowdSec Bouncer Key Display moved to CrowdSecConfig page (Sprint 3)
  // Tests for bouncer key display are now in CrowdSecConfig tests

  describe('Live Log Viewer', () => {
    it('should show live log viewer when Cerberus is enabled', async () => {
      vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(mockSecurityStatusAllEnabled)

      await renderSecurityPage()

      await waitFor(() => {
        expect(screen.getByTestId('live-log-viewer')).toBeInTheDocument()
      })
    })

    it('should not show live log viewer when Cerberus is disabled', async () => {
      vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(mockSecurityStatusCerberusDisabled)

      await renderSecurityPage()

      await waitFor(() => {
        expect(screen.getByText(/Security Features Unavailable/i)).toBeInTheDocument()
      })

      expect(screen.queryByTestId('live-log-viewer')).not.toBeInTheDocument()
    })
  })

  describe('Admin Whitelist', () => {
    it('should display admin whitelist section when Cerberus is enabled', async () => {
      vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(mockSecurityStatusAllEnabled)

      await renderSecurityPage()

      await waitFor(() => {
        expect(screen.getByText('Admin Whitelist')).toBeInTheDocument()
      })
    })

    it('should load admin whitelist value from config', async () => {
      vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(mockSecurityStatusAllEnabled)

      await renderSecurityPage()

      await waitFor(() => {
        expect(screen.getByDisplayValue('10.0.0.0/8')).toBeInTheDocument()
      })
    })

    it('should not show admin whitelist when Cerberus is disabled', async () => {
      vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(mockSecurityStatusCerberusDisabled)

      await renderSecurityPage()

      await waitFor(() => {
        expect(screen.getByText(/Security Features Unavailable/i)).toBeInTheDocument()
      })

      expect(screen.queryByText('Admin Whitelist')).not.toBeInTheDocument()
    })

    it('should have Save and Generate Token buttons', async () => {
      vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(mockSecurityStatusAllEnabled)

      await renderSecurityPage()

      await waitFor(() => {
        expect(screen.getByRole('button', { name: /Save/i })).toBeInTheDocument()
        expect(screen.getByRole('button', { name: /Generate Token/i })).toBeInTheDocument()
      })
    })
  })

  describe('CrowdSec Status Display', () => {
    it('should show running status with PID when CrowdSec is running', async () => {
      vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(mockSecurityStatusAllEnabled)
      vi.mocked(crowdsecApi.statusCrowdsec).mockResolvedValue({ running: true, pid: 5678, lapi_ready: true })

      await renderSecurityPage()

      await waitFor(() => {
        expect(screen.getByText(/Running \(pid 5678\)/i)).toBeInTheDocument()
      })
    })

    it('should show process stopped when CrowdSec is not running', async () => {
      vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(mockSecurityStatusAllEnabled)
      vi.mocked(crowdsecApi.statusCrowdsec).mockResolvedValue({ running: false, pid: 0, lapi_ready: false })

      await renderSecurityPage()

      await waitFor(() => {
        expect(screen.getByText(/Process stopped/i)).toBeInTheDocument()
      })
    })
  })

  describe('Service Toggle Interactions', () => {
    it('should toggle ACL when switch is clicked', async () => {
      const user = userEvent.setup()
      vi.mocked(securityApi.getSecurityStatus).mockResolvedValue({
        ...mockSecurityStatusAllEnabled,
        acl: { enabled: false },
      })

      await renderSecurityPage()

      await waitFor(() => screen.getByTestId('toggle-acl'))
      await user.click(screen.getByTestId('toggle-acl'))

      await waitFor(() => {
        expect(settingsApi.updateSetting).toHaveBeenCalledWith(
          'security.acl.enabled',
          'true',
          'security',
          'bool'
        )
      })
    })

    it('should toggle WAF when switch is clicked', async () => {
      const user = userEvent.setup()
      vi.mocked(securityApi.getSecurityStatus).mockResolvedValue({
        ...mockSecurityStatusAllEnabled,
        waf: { mode: 'enabled' as const, enabled: false },
      })

      await renderSecurityPage()

      await waitFor(() => screen.getByTestId('toggle-waf'))
      await user.click(screen.getByTestId('toggle-waf'))

      await waitFor(() => {
        expect(settingsApi.updateSetting).toHaveBeenCalledWith(
          'security.waf.enabled',
          'true',
          'security',
          'bool'
        )
      })
    })

    it('should toggle Rate Limiting when switch is clicked', async () => {
      const user = userEvent.setup()
      vi.mocked(securityApi.getSecurityStatus).mockResolvedValue({
        ...mockSecurityStatusAllEnabled,
        rate_limit: { enabled: false },
      })

      await renderSecurityPage()

      await waitFor(() => screen.getByTestId('toggle-rate-limit'))
      await user.click(screen.getByTestId('toggle-rate-limit'))

      await waitFor(() => {
        expect(settingsApi.updateSetting).toHaveBeenCalledWith(
          'security.rate_limit.enabled',
          'true',
          'security',
          'bool'
        )
      })
    })
  })

  describe('CrowdSec Power Toggle', () => {
    it('should start CrowdSec when toggle is turned on', async () => {
      const user = userEvent.setup()
      vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(mockSecurityStatusCrowdsecDisabled)
      vi.mocked(crowdsecApi.statusCrowdsec).mockResolvedValue({ running: false, pid: 0, lapi_ready: false })
      vi.mocked(crowdsecApi.startCrowdsec).mockResolvedValue({ status: 'started', pid: 123, lapi_ready: true })

      await renderSecurityPage()

      await waitFor(() => screen.getByTestId('toggle-crowdsec'))
      await user.click(screen.getByTestId('toggle-crowdsec'))

      await waitFor(() => {
        expect(settingsApi.updateSetting).toHaveBeenCalledWith(
          'security.crowdsec.enabled',
          'true',
          'security',
          'bool'
        )
        expect(crowdsecApi.startCrowdsec).toHaveBeenCalled()
      })
    })

    it('should stop CrowdSec when toggle is turned off', async () => {
      const user = userEvent.setup()
      vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(mockSecurityStatusAllEnabled)
      vi.mocked(crowdsecApi.statusCrowdsec).mockResolvedValue({ running: true, pid: 1234, lapi_ready: true })
      vi.mocked(crowdsecApi.stopCrowdsec).mockResolvedValue({ success: true })

      await renderSecurityPage()

      await waitFor(() => screen.getByTestId('toggle-crowdsec'))
      await user.click(screen.getByTestId('toggle-crowdsec'))

      await waitFor(() => {
        expect(settingsApi.updateSetting).toHaveBeenCalledWith(
          'security.crowdsec.enabled',
          'false',
          'security',
          'bool'
        )
        expect(crowdsecApi.stopCrowdsec).toHaveBeenCalled()
      })
    })
  })

  describe('Notification Settings Modal', () => {
    // Skip: Modal component uses WebSocket connections internally
    it.skip('should open notification settings modal when button is clicked', async () => {
      const user = userEvent.setup()
      vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(mockSecurityStatusAllEnabled)

      await renderSecurityPage()

      await waitFor(() => {
        expect(screen.getByRole('button', { name: /Notifications/i })).toBeInTheDocument()
      })

      await user.click(screen.getByRole('button', { name: /Notifications/i }))

      // Modal should open - look for modal content
      await waitFor(() => {
        // The modal has a title "Notification Settings"
        expect(screen.getByRole('dialog')).toBeInTheDocument()
      })
    })
  })

  describe('Documentation Link', () => {
    it('should open docs link when Docs button is clicked', async () => {
      const user = userEvent.setup()
      const mockOpen = vi.spyOn(window, 'open').mockImplementation(() => null)
      vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(mockSecurityStatusAllEnabled)

      await renderSecurityPage()

      await waitFor(() => {
        expect(screen.getByRole('button', { name: /Docs/i })).toBeInTheDocument()
      })

      await user.click(screen.getByRole('button', { name: /Docs/i }))

      expect(mockOpen).toHaveBeenCalledWith('https://wikid82.github.io/charon/security', '_blank')
    })
  })
})
