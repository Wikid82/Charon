import { QueryClientProvider } from '@tanstack/react-query'
import { cleanup, render, screen, waitFor  } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { BrowserRouter } from 'react-router-dom'
import { describe, it, expect, vi, beforeEach } from 'vitest'

import * as crowdsecApi from '../../api/crowdsec'
import * as logsApi from '../../api/logs'
import * as api from '../../api/security'
import * as settingsApi from '../../api/settings'
import { createTestQueryClient } from '../../test/createTestQueryClient'
import Security from '../Security'

import type { SecurityStatus, RuleSetsResponse } from '../../api/security'
import type * as ReactRouterDom from 'react-router-dom'

const mockNavigate = vi.fn()

vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual<typeof ReactRouterDom>('react-router-dom')
  return { ...actual, useNavigate: () => mockNavigate }
})

vi.mock('../../api/security')
vi.mock('../../api/settings')
vi.mock('../../api/crowdsec')
vi.mock('../../api/logs', () => ({
  connectLiveLogs: vi.fn(() => vi.fn()),
  connectSecurityLogs: vi.fn(() => vi.fn()),
}))
vi.mock('../../components/LiveLogViewer', () => ({
  LiveLogViewer: () => <div data-testid="live-log-viewer" />,
}))
vi.mock('../../components/SecurityNotificationSettingsModal', () => ({
  SecurityNotificationSettingsModal: () => <div data-testid="security-notification-modal" />,
}))
vi.mock('../../components/CrowdSecKeyWarning', () => ({
  CrowdSecKeyWarning: () => null,
}))
vi.mock('../../hooks/useNotifications', () => ({
  useSecurityNotificationSettings: () => ({
    data: {
      enabled: false,
      min_log_level: 'warn',
      security_waf_enabled: true,
      security_acl_enabled: true,
      security_rate_limit_enabled: true,
      webhook_url: '',
    },
    isLoading: false,
  }),
  useUpdateSecurityNotificationSettings: () => ({
    mutate: vi.fn(),
    isPending: false,
  }),
}))

const defaultFeatureFlags = {
  'feature.cerberus.enabled': true,
  'feature.uptime.enabled': true,
}

const baseStatus: SecurityStatus = {
  cerberus: { enabled: true },
  crowdsec: { enabled: false, mode: 'disabled' as const, api_url: '' },
  waf: { enabled: false, mode: 'disabled' as const },
  rate_limit: { enabled: false },
  acl: { enabled: false },
}

const createQueryClient = (initialData = []) => createTestQueryClient([
  { key: ['securityConfig'], data: mockSecurityConfig },
  { key: ['securityRulesets'], data: mockRuleSets },
  { key: ['feature-flags'], data: defaultFeatureFlags },
  ...initialData,
])

const renderWithProviders = (ui: React.ReactNode, initialData = []) => {
  const qc = createQueryClient(initialData)
  return render(
    <QueryClientProvider client={qc}>
      <BrowserRouter>
        {ui}
      </BrowserRouter>
    </QueryClientProvider>
  )
}

const mockSecurityConfig = {
  config: {
    name: 'default',
    waf_mode: 'block',
    waf_rules_source: '',
    admin_whitelist: '',
  },
}

const mockRuleSets: RuleSetsResponse = {
  rulesets: [
    { id: 1, uuid: 'uuid-1', name: 'OWASP CRS', source_url: '', mode: 'blocking', last_updated: '', content: '' },
    { id: 2, uuid: 'uuid-2', name: 'Custom Rules', source_url: '', mode: 'detection', last_updated: '', content: '' },
  ],
}
    // Types already imported at top-level; avoid duplicate declarations
describe('Security page', () => {
  beforeEach(() => {
    vi.resetAllMocks()
    vi.mocked(api.getSecurityStatus).mockResolvedValue(baseStatus as SecurityStatus)
    vi.mocked(api.getSecurityConfig).mockResolvedValue(mockSecurityConfig)
    vi.mocked(api.getRuleSets).mockResolvedValue(mockRuleSets)
    vi.mocked(api.updateSecurityConfig).mockResolvedValue({})
    // Mock WebSocket connections for LiveLogViewer
    vi.mocked(logsApi.connectLiveLogs).mockReturnValue(vi.fn())
    vi.mocked(logsApi.connectSecurityLogs).mockReturnValue(vi.fn())
    vi.mocked(crowdsecApi.statusCrowdsec).mockResolvedValue({ running: false, pid: 0, lapi_ready: false })
    vi.mocked(crowdsecApi.getCrowdsecKeyStatus).mockResolvedValue({
      env_key_rejected: false,
      key_source: 'auto-generated',
      current_key_preview: '...',
      message: 'OK'
    })
  })

  it('shows banner when all services are disabled and links to docs', async () => {
    const status: SecurityStatus = {
      cerberus: { enabled: false },
      crowdsec: { enabled: false, mode: 'disabled' as const, api_url: '' },
      waf: { enabled: false, mode: 'disabled' as const },
      rate_limit: { enabled: false },
      acl: { enabled: false },
    }
    vi.mocked(api.getSecurityStatus).mockResolvedValueOnce(status as SecurityStatus)
    vi.mocked(api.getSecurityStatus).mockResolvedValueOnce({
      ...status,
      crowdsec: { ...status.crowdsec, enabled: true }
    } as SecurityStatus)

    renderWithProviders(<Security />)
    expect(await screen.findByText('Security Features Unavailable')).toBeInTheDocument()
    const docBtns = screen.getAllByText('Learn More')
    expect(docBtns.length).toBeGreaterThan(0)
  })

  it('renders per-service toggles and calls updateSetting on change', async () => {
    const status: SecurityStatus = {
      cerberus: { enabled: true },
      crowdsec: { enabled: false, mode: 'disabled' as const, api_url: '' },
      waf: { enabled: false, mode: 'disabled' as const },
      rate_limit: { enabled: false },
      acl: { enabled: false },
    }
    vi.mocked(api.getSecurityStatus).mockResolvedValue(status as SecurityStatus)
    vi.mocked(settingsApi.updateSetting).mockResolvedValue()

    renderWithProviders(<Security />)
    expect(await screen.findByText('Cerberus Dashboard')).toBeInTheDocument()
    const crowdsecToggle = screen.getByTestId('toggle-crowdsec') as HTMLInputElement
    expect(crowdsecToggle.disabled).toBe(false)
    // Ensure enable-all controls were removed
    expect(screen.queryByTestId('enable-all-btn')).toBeNull()
  })

  it('calls updateSetting when toggling ACL', async () => {
    const status: SecurityStatus = {
      cerberus: { enabled: true },
      crowdsec: { enabled: false, mode: 'disabled' as const, api_url: '' },
      waf: { enabled: false, mode: 'disabled' as const },
      rate_limit: { enabled: false },
      acl: { enabled: false },
    }
    vi.mocked(api.getSecurityStatus).mockResolvedValue(status as SecurityStatus)
    const updateSpy = vi.mocked(settingsApi.updateSetting)
    renderWithProviders(<Security />)
    expect(await screen.findByText('Cerberus Dashboard')).toBeInTheDocument()
    const aclToggle = screen.getByTestId('toggle-acl')
    await userEvent.click(aclToggle)
    await waitFor(() => expect(updateSpy).toHaveBeenCalledWith('security.acl.enabled', 'true', 'security', 'bool'))
  })

  // Export button is in CrowdSecConfig component, not Security page

  it('calls start/stop endpoints for CrowdSec via toggle', async () => {
    const user = userEvent.setup()
    const baseStatus: SecurityStatus = {
      cerberus: { enabled: true },
      crowdsec: { enabled: false, mode: 'disabled' as const, api_url: '' },
      waf: { enabled: false, mode: 'disabled' as const },
      rate_limit: { enabled: false },
      acl: { enabled: false },
    }

    vi.mocked(api.getSecurityStatus).mockResolvedValue(baseStatus as SecurityStatus)
    vi.mocked(crowdsecApi.statusCrowdsec).mockResolvedValue({ running: false, pid: 0, lapi_ready: false })
    vi.mocked(crowdsecApi.startCrowdsec).mockResolvedValue({ status: 'started', pid: 123, lapi_ready: true })
    vi.mocked(settingsApi.updateSetting).mockResolvedValue()

    renderWithProviders(<Security />)
    expect(await screen.findByText('Cerberus Dashboard')).toBeInTheDocument()
    const toggle = screen.getByTestId('toggle-crowdsec')
    await user.click(toggle)
    await waitFor(() => expect(crowdsecApi.startCrowdsec).toHaveBeenCalled())

    cleanup()

    const enabledStatus: SecurityStatus = { ...baseStatus, crowdsec: { enabled: true, mode: 'local' as const, api_url: '' } }
    vi.mocked(api.getSecurityStatus).mockResolvedValue(enabledStatus as SecurityStatus)
    vi.mocked(crowdsecApi.statusCrowdsec).mockResolvedValue({ running: true, pid: 123, lapi_ready: true })
    vi.mocked(crowdsecApi.stopCrowdsec).mockResolvedValue(undefined)

    renderWithProviders(<Security />)
    expect(await screen.findByText('Cerberus Dashboard')).toBeInTheDocument()
    const stopToggle = screen.getByTestId('toggle-crowdsec')
    await user.click(stopToggle)
    await waitFor(() => expect(crowdsecApi.stopCrowdsec).toHaveBeenCalled())
  })

  it('disables service toggles when cerberus is off', async () => {
    const status: SecurityStatus = {
      cerberus: { enabled: false },
      crowdsec: { enabled: false, mode: 'disabled' as const, api_url: '' },
      waf: { enabled: false, mode: 'disabled' as const },
      rate_limit: { enabled: false },
      acl: { enabled: false },
    }
    vi.mocked(api.getSecurityStatus).mockResolvedValue(status as SecurityStatus)
    renderWithProviders(<Security />)
    expect(await screen.findByText('Security Features Unavailable')).toBeInTheDocument()
    const crowdsecToggle = screen.getByTestId('toggle-crowdsec')
    expect(crowdsecToggle).toBeDisabled()
  })

  it('displays correct WAF threat protection summary when enabled', async () => {
    const status: SecurityStatus = {
      cerberus: { enabled: true },
      crowdsec: { enabled: false, mode: 'disabled' as const, api_url: '' },
      waf: { enabled: true, mode: 'enabled' as const },
      rate_limit: { enabled: false },
      acl: { enabled: false },
    }
    vi.mocked(api.getSecurityStatus).mockResolvedValue(status as SecurityStatus)
    vi.mocked(api.getSecurityConfig).mockResolvedValue({
      config: { ...mockSecurityConfig.config, waf_mode: 'monitor' },
    })
    vi.mocked(api.getRuleSets).mockResolvedValue(mockRuleSets)

    renderWithProviders(<Security />)
    // WAF now shows threat protection summary instead of mode text
    expect(await screen.findByText(/SQL injection, XSS, RCE/i)).toBeInTheDocument()
  })
})
