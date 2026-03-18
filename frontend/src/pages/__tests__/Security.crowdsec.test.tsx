import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { BrowserRouter } from 'react-router-dom'
import { describe, it, expect, vi, beforeEach } from 'vitest'

import * as crowdsecApi from '../../api/crowdsec'
import * as logsApi from '../../api/logs'
import * as api from '../../api/security'
import * as settingsApi from '../../api/settings'
import Security from '../Security'

import type { SecurityStatus } from '../../api/security'
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
  CrowdSecKeyWarning: () => <div data-testid="crowdsec-key-warning">CrowdSec API Key Updated</div>,
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

const baseStatus: SecurityStatus = {
  cerberus: { enabled: true },
  crowdsec: { enabled: false, mode: 'disabled' as const, api_url: '' },
  waf: { enabled: false, mode: 'disabled' as const },
  rate_limit: { enabled: false },
  acl: { enabled: false },
}

function createQueryClient() {
  return new QueryClient({
    defaultOptions: {
      queries: { retry: false, gcTime: Infinity },
      mutations: { retry: false },
    },
  })
}

function renderSecurity(queryClient?: QueryClient) {
  const qc = queryClient ?? createQueryClient()
  return {
    qc,
    ...render(
      <QueryClientProvider client={qc}>
        <BrowserRouter>
          <Security />
        </BrowserRouter>
      </QueryClientProvider>
    ),
  }
}

describe('Security CrowdSec mutation UX', () => {
  beforeEach(() => {
    vi.resetAllMocks()
    vi.mocked(api.getSecurityStatus).mockResolvedValue(baseStatus)
    vi.mocked(api.getSecurityConfig).mockResolvedValue({ config: { name: 'default', waf_mode: 'block', waf_rules_source: '', admin_whitelist: '' } })
    vi.mocked(api.getRuleSets).mockResolvedValue({ rulesets: [] })
    vi.mocked(api.updateSecurityConfig).mockResolvedValue({})
    vi.mocked(logsApi.connectLiveLogs).mockReturnValue(vi.fn())
    vi.mocked(logsApi.connectSecurityLogs).mockReturnValue(vi.fn())
    vi.mocked(crowdsecApi.statusCrowdsec).mockResolvedValue({ running: false, pid: 0, lapi_ready: false })
    vi.mocked(crowdsecApi.getCrowdsecKeyStatus).mockResolvedValue({
      env_key_rejected: false,
      key_source: 'auto-generated',
      current_key_preview: '...',
      message: 'OK',
    })
    vi.mocked(settingsApi.updateSetting).mockResolvedValue(undefined)
  })

  it('toggle stays checked while crowdsecPowerMutation is pending', async () => {
    // startCrowdsec never resolves — keeps mutation pending
    vi.mocked(crowdsecApi.startCrowdsec).mockReturnValue(new Promise(() => {}))

    renderSecurity()

    const toggle = await screen.findByTestId('toggle-crowdsec')
    await userEvent.click(toggle)

    // While pending, the toggle must reflect the user's intent (checked=true)
    await waitFor(() => {
      expect(toggle).toBeChecked()
    })
  })

  it('CrowdSec badge shows "Starting..." while mutation is pending', async () => {
    vi.mocked(crowdsecApi.startCrowdsec).mockReturnValue(new Promise(() => {}))

    renderSecurity()

    const toggle = await screen.findByTestId('toggle-crowdsec')
    await userEvent.click(toggle)

    await waitFor(() => {
      expect(screen.getByText('Starting...')).toBeInTheDocument()
    })
  })

  it('CrowdSecKeyWarning is not rendered while crowdsecPowerMutation is pending', async () => {
    vi.mocked(crowdsecApi.startCrowdsec).mockReturnValue(new Promise(() => {}))
    vi.mocked(crowdsecApi.getCrowdsecKeyStatus).mockResolvedValue({
      env_key_rejected: true,
      key_source: 'env',
      full_key: 'abc123',
      current_key_preview: 'abc...',
      rejected_key_preview: 'def...',
      message: 'Key rejected',
    })

    renderSecurity()

    const toggle = await screen.findByTestId('toggle-crowdsec')
    await userEvent.click(toggle)

    await waitFor(() => {
      expect(toggle).toBeChecked()
    })

    expect(screen.queryByTestId('crowdsec-key-warning')).not.toBeInTheDocument()
  })

  it('toggle reflects correct final state after mutation succeeds', async () => {
    vi.mocked(crowdsecApi.startCrowdsec).mockResolvedValue({ status: 'started', pid: 123, lapi_ready: true })
    vi.mocked(crowdsecApi.statusCrowdsec)
      .mockResolvedValueOnce({ running: false, pid: 0, lapi_ready: false })
      .mockResolvedValue({ running: true, pid: 123, lapi_ready: true })
    vi.mocked(api.getSecurityStatus)
      .mockResolvedValue(baseStatus)
      .mockResolvedValueOnce(baseStatus)
      .mockResolvedValue({ ...baseStatus, crowdsec: { ...baseStatus.crowdsec, enabled: true } })

    renderSecurity()

    const toggle = await screen.findByTestId('toggle-crowdsec')
    await userEvent.click(toggle)

    await waitFor(() => {
      expect(toggle).toBeChecked()
    }, { timeout: 3000 })
  })

  it('toggle reverts to unchecked when mutation fails', async () => {
    vi.mocked(crowdsecApi.startCrowdsec).mockRejectedValue(new Error('failed'))

    renderSecurity()

    const toggle = await screen.findByTestId('toggle-crowdsec')
    await userEvent.click(toggle)

    await waitFor(() => {
      expect(toggle).not.toBeChecked()
    }, { timeout: 3000 })
  })
})
