import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { AxiosError, type AxiosResponse } from 'axios'
import { BrowserRouter } from 'react-router'
import { describe, it, expect, vi, beforeEach } from 'vitest'

import * as crowdsecApi from '../../api/crowdsec'
import * as logsApi from '../../api/logs'
import * as api from '../../api/security'
import * as settingsApi from '../../api/settings'
import { toast } from '../../utils/toast'
import Security from '../Security'

import type { SecurityStatus } from '../../api/security'
import type * as ReactRouterDom from 'react-router'

const BUSY_TEXT = 'CrowdSec is busy installing a preset. Try again when it finishes.'

// The refusal the server sends while a background preset install holds the CrowdSec lock.
const busyError = () =>
  new AxiosError('Request failed with status code 409', 'ERR_BAD_REQUEST', undefined, undefined, {
    status: 409,
    statusText: 'Conflict',
    headers: {},
    config: {},
    data: { error: 'busy', error_code: 'crowdsec_busy', job_id: 'job-1' },
  } as AxiosResponse)

const mockNavigate = vi.fn()

// Sign-in throttle status is admin-only; these suites cover the page for a non-admin session
vi.mock('../../hooks/useAuth', () => ({ useAuth: () => ({ user: { role: 'user' } }) }))

vi.mock('react-router', async () => {
  const actual = await vi.importActual<typeof ReactRouterDom>('react-router')
  return { ...actual, useNavigate: () => mockNavigate }
})

vi.mock('../../api/security')
vi.mock('../../api/settings')
vi.mock('../../api/crowdsec')
vi.mock('../../utils/toast', () => ({
  toast: {
    info: vi.fn(),
    success: vi.fn(),
    warning: vi.fn(),
    error: vi.fn(),
    loading: vi.fn(),
    dismiss: vi.fn(),
  },
}))
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

    const user = userEvent.setup()
    renderSecurity()

    const toggle = await screen.findByTestId('toggle-crowdsec')
    await user.click(toggle)

    // While pending, the toggle must reflect the user's intent (checked=true)
    await waitFor(() => {
      expect(toggle).toBeChecked()
    })
  })

  it('CrowdSec badge shows "Starting..." while mutation is pending', async () => {
    vi.mocked(crowdsecApi.startCrowdsec).mockReturnValue(new Promise(() => {}))

    const user = userEvent.setup()
    renderSecurity()

    const toggle = await screen.findByTestId('toggle-crowdsec')
    await user.click(toggle)

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

    const user = userEvent.setup()
    renderSecurity()

    const toggle = await screen.findByTestId('toggle-crowdsec')
    await user.click(toggle)

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
    // Call order: 1st → baseStatus, 2nd → baseStatus, 3rd+ → enabled
    vi.mocked(api.getSecurityStatus)
      .mockResolvedValueOnce(baseStatus)
      .mockResolvedValueOnce(baseStatus)
      .mockResolvedValue({ ...baseStatus, crowdsec: { ...baseStatus.crowdsec, enabled: true } })

    const user = userEvent.setup()
    renderSecurity()

    const toggle = await screen.findByTestId('toggle-crowdsec')
    await user.click(toggle)

    await waitFor(() => {
      expect(toggle).toBeChecked()
    }, { timeout: 3000 })
  })

  it('toggle reverts to unchecked when mutation fails', async () => {
    vi.mocked(crowdsecApi.startCrowdsec).mockRejectedValue(new Error('failed'))

    const user = userEvent.setup()
    renderSecurity()

    const toggle = await screen.findByTestId('toggle-crowdsec')
    await user.click(toggle)

    await waitFor(() => {
      expect(toggle).not.toBeChecked()
    }, { timeout: 3000 })
  })

  it('busy start refusal leaves the setting untouched and shows the busy message', async () => {
    vi.mocked(crowdsecApi.startCrowdsec).mockRejectedValue(busyError())

    const user = userEvent.setup()
    renderSecurity()

    const toggle = await screen.findByTestId('toggle-crowdsec')
    await user.click(toggle)

    await waitFor(() => expect(toast.error).toHaveBeenCalledWith(BUSY_TEXT))
    expect(settingsApi.updateSetting).not.toHaveBeenCalled()
    expect(toggle).not.toBeChecked()
  })

  it('busy stop refusal leaves the setting untouched and shows the busy message', async () => {
    vi.mocked(crowdsecApi.statusCrowdsec).mockResolvedValue({ running: true, pid: 1234, lapi_ready: true })
    vi.mocked(crowdsecApi.stopCrowdsec).mockRejectedValue(busyError())

    const user = userEvent.setup()
    renderSecurity()

    const toggle = await screen.findByTestId('toggle-crowdsec')
    await waitFor(() => expect(toggle).toBeChecked())
    await user.click(toggle)

    await waitFor(() => expect(toast.error).toHaveBeenCalledWith(BUSY_TEXT))
    expect(settingsApi.updateSetting).not.toHaveBeenCalled()
    expect(toggle).toBeChecked()
  })

  it('a start refused because CrowdSec was turned off shows the translated text, not the status code', async () => {
    vi.mocked(crowdsecApi.startCrowdsec).mockRejectedValue(
      new AxiosError('Request failed with status code 409', 'ERR_BAD_REQUEST', undefined, undefined, {
        status: 409,
        statusText: 'Conflict',
        headers: {},
        config: {},
        data: { error: 'CrowdSec was turned off while it was starting', error_code: 'crowdsec_state_changed' },
      } as AxiosResponse),
    )

    const user = userEvent.setup()
    renderSecurity()

    const toggle = await screen.findByTestId('toggle-crowdsec')
    await user.click(toggle)

    await waitFor(() => expect(toast.error).toHaveBeenCalledWith('CrowdSec was turned off while it was starting.'))
    expect(toast.error).not.toHaveBeenCalledWith(expect.stringContaining('Request failed with status code 409'))
    expect(settingsApi.updateSetting).not.toHaveBeenCalled()
  })

  it('non-busy start failure shows the translated start message with the server detail', async () => {
    vi.mocked(crowdsecApi.startCrowdsec).mockRejectedValue(new Error('boom'))

    const user = userEvent.setup()
    renderSecurity()

    const toggle = await screen.findByTestId('toggle-crowdsec')
    await user.click(toggle)

    await waitFor(() => expect(toast.error).toHaveBeenCalledWith('Failed to start CrowdSec: boom'))
    expect(settingsApi.updateSetting).not.toHaveBeenCalled()
  })

  it('non-busy stop failure shows the translated stop message with the server detail', async () => {
    vi.mocked(crowdsecApi.statusCrowdsec).mockResolvedValue({ running: true, pid: 1234, lapi_ready: true })
    vi.mocked(crowdsecApi.stopCrowdsec).mockRejectedValue(new Error('kill failed'))

    const user = userEvent.setup()
    renderSecurity()

    const toggle = await screen.findByTestId('toggle-crowdsec')
    await waitFor(() => expect(toggle).toBeChecked())
    await user.click(toggle)

    await waitFor(() => expect(toast.error).toHaveBeenCalledWith('Failed to stop CrowdSec: kill failed'))
    expect(settingsApi.updateSetting).not.toHaveBeenCalled()
  })

  it('shows the start-verification message when the process is not running after start', async () => {
    vi.mocked(crowdsecApi.startCrowdsec).mockResolvedValue({ status: 'started', pid: 1, lapi_ready: false })
    vi.mocked(crowdsecApi.statusCrowdsec).mockResolvedValue({ running: false, pid: 0, lapi_ready: false })

    const user = userEvent.setup()
    renderSecurity()

    const toggle = await screen.findByTestId('toggle-crowdsec')
    await user.click(toggle)

    await waitFor(() =>
      expect(toast.error).toHaveBeenCalledWith(
        'Failed to start CrowdSec: CrowdSec process failed to start. Check server logs for details.',
      ),
    )
  })

  it('shows the stop-verification message when the process is still running after stop', async () => {
    vi.mocked(crowdsecApi.statusCrowdsec).mockResolvedValue({ running: true, pid: 1234, lapi_ready: true })
    vi.mocked(crowdsecApi.stopCrowdsec).mockResolvedValue(undefined)

    const user = userEvent.setup()
    renderSecurity()

    const toggle = await screen.findByTestId('toggle-crowdsec')
    await waitFor(() => expect(toggle).toBeChecked())
    await user.click(toggle)

    await waitFor(() =>
      expect(toast.error).toHaveBeenCalledWith(
        'Failed to stop CrowdSec: CrowdSec process still running. Check server logs for details.',
      ),
    )
  })

  it('announces the start and reports LAPI readiness with translated toasts', async () => {
    vi.mocked(crowdsecApi.startCrowdsec).mockResolvedValue({ status: 'started', pid: 123, lapi_ready: true })
    vi.mocked(crowdsecApi.statusCrowdsec)
      .mockResolvedValueOnce({ running: false, pid: 0, lapi_ready: false })
      .mockResolvedValue({ running: true, pid: 123, lapi_ready: true })

    const user = userEvent.setup()
    renderSecurity()

    const toggle = await screen.findByTestId('toggle-crowdsec')
    await user.click(toggle)

    await waitFor(() => expect(toast.info).toHaveBeenCalledWith('Starting CrowdSec... This may take up to 30 seconds'))
    await waitFor(() => expect(toast.success).toHaveBeenCalledWith('CrowdSec started and LAPI is ready'))
  })

  it('falls back to the generic translated started toast when the start response carries no LAPI status', async () => {
    vi.mocked(crowdsecApi.startCrowdsec).mockResolvedValue({ status: 'started', pid: 123 })
    vi.mocked(crowdsecApi.statusCrowdsec)
      .mockResolvedValueOnce({ running: false, pid: 0, lapi_ready: false })
      .mockResolvedValue({ running: true, pid: 123, lapi_ready: false })

    const user = userEvent.setup()
    renderSecurity()

    const toggle = await screen.findByTestId('toggle-crowdsec')
    await user.click(toggle)

    await waitFor(() => expect(toast.success).toHaveBeenCalledWith('CrowdSec started'))
    expect(toast.warning).not.toHaveBeenCalledWith(expect.stringContaining('LAPI'))
  })

  it('warns when CrowdSec started but LAPI is still initializing', async () => {
    vi.mocked(crowdsecApi.startCrowdsec).mockResolvedValue({ status: 'started', pid: 123, lapi_ready: false })
    vi.mocked(crowdsecApi.statusCrowdsec)
      .mockResolvedValueOnce({ running: false, pid: 0, lapi_ready: false })
      .mockResolvedValue({ running: true, pid: 123, lapi_ready: false })

    const user = userEvent.setup()
    renderSecurity()

    const toggle = await screen.findByTestId('toggle-crowdsec')
    await user.click(toggle)

    await waitFor(() =>
      expect(toast.warning).toHaveBeenCalledWith(
        'CrowdSec started but LAPI is still initializing. Please wait before enrolling.',
      ),
    )
  })

  it('reports a successful stop with the translated toast', async () => {
    vi.mocked(crowdsecApi.statusCrowdsec)
      .mockResolvedValueOnce({ running: true, pid: 1234, lapi_ready: true })
      .mockResolvedValue({ running: false, pid: 0, lapi_ready: false })
    vi.mocked(crowdsecApi.stopCrowdsec).mockResolvedValue(undefined)

    const user = userEvent.setup()
    renderSecurity()

    const toggle = await screen.findByTestId('toggle-crowdsec')
    await waitFor(() => expect(toggle).toBeChecked())
    await user.click(toggle)

    await waitFor(() => expect(toast.success).toHaveBeenCalledWith('CrowdSec stopped'))
  })

  it('refreshes the settings query after a refused start so the setting cannot go stale', async () => {
    vi.mocked(crowdsecApi.startCrowdsec).mockRejectedValue(busyError())

    const user = userEvent.setup()
    const { qc } = renderSecurity()
    const invalidateSpy = vi.spyOn(qc, 'invalidateQueries')

    const toggle = await screen.findByTestId('toggle-crowdsec')
    await user.click(toggle)

    await waitFor(() => expect(toast.error).toHaveBeenCalledWith(BUSY_TEXT))
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ['settings'] })
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ['security-status'] })
  })
})
