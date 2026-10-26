import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { AxiosError } from 'axios'
import { describe, it, expect, vi, beforeEach } from 'vitest'

import * as backupsApi from '../../api/backups'
import * as crowdsecApi from '../../api/crowdsec'
import * as featureFlagsApi from '../../api/featureFlags'
import * as presetsApi from '../../api/presets'
import * as securityApi from '../../api/security'
import * as settingsApi from '../../api/settings'
import * as systemApi from '../../api/system'
import { renderWithQueryClient } from '../../test-utils/renderWithQueryClient'
import { toast } from '../../utils/toast'
import CrowdSecConfig from '../CrowdSecConfig'

const enrollMock = vi.hoisted(() => vi.fn())

vi.mock('../../api/security')
vi.mock('../../api/crowdsec')
vi.mock('../../api/presets')
vi.mock('../../api/backups')
vi.mock('../../api/settings')
vi.mock('../../api/featureFlags')
vi.mock('../../api/system')
vi.mock('../../hooks/useConsoleEnrollment', () => ({
  useConsoleStatus: vi.fn(() => ({
    data: {
      status: 'not_enrolled',
      key_present: false,
      last_error: null,
      last_attempt_at: null,
      enrolled_at: null,
      last_heartbeat_at: null,
      correlation_id: 'corr-1',
      tenant: 'default',
      agent_name: 'charon-agent',
    },
    isLoading: false,
    isRefetching: false,
  })),
  useEnrollConsole: () => ({ mutateAsync: enrollMock, isPending: false }),
  useClearConsoleEnrollment: vi.fn(() => ({ mutate: vi.fn(), isPending: false })),
}))
vi.mock('../../components/CrowdSecBouncerKeyDisplay', () => ({
  CrowdSecBouncerKeyDisplay: () => null,
}))
vi.mock('../../utils/toast', () => ({
  toast: { success: vi.fn(), error: vi.fn(), info: vi.fn() },
}))

const BUSY_TEXT = 'CrowdSec is busy installing a preset. Try again when it finishes.'

const baseStatus = {
  cerberus: { enabled: true },
  crowdsec: { enabled: true, mode: 'local' as const, api_url: '' },
  waf: { enabled: true, mode: 'enabled' as const },
  rate_limit: { enabled: true },
  acl: { enabled: true },
}

const axiosError = (status: number, data: Record<string, unknown>) =>
  new AxiosError('Request failed', undefined, undefined, undefined, {
    status,
    statusText: String(status),
    headers: {},
    config: {},
    data,
  } as never)

const renderPage = async () => {
  renderWithQueryClient(<CrowdSecConfig />)
  await screen.findByText('CrowdSec Configuration')
}

// Fills the enrolment form and submits it, so each test only varies the refusal.
const submitEnrollment = async () => {
  await userEvent.type(screen.getByTestId('console-enrollment-token'), 'token-value')
  await userEvent.click(screen.getByTestId('console-ack-checkbox'))
  await userEvent.click(screen.getByTestId('console-enroll-btn'))
}

describe('CrowdSecConfig – console enrolment refusals', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    enrollMock.mockReset()
    vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(baseStatus)
    vi.mocked(crowdsecApi.statusCrowdsec).mockResolvedValue({ running: true, pid: 123, lapi_ready: true })
    vi.mocked(crowdsecApi.listCrowdsecFiles).mockResolvedValue({ files: ['acquis.yaml'] })
    vi.mocked(crowdsecApi.readCrowdsecFile).mockResolvedValue({ content: '' })
    vi.mocked(crowdsecApi.listCrowdsecDecisions).mockResolvedValue({ decisions: [] })
    vi.mocked(crowdsecApi.listWhitelists).mockResolvedValue([])
    vi.mocked(presetsApi.listCrowdsecPresets).mockResolvedValue({ presets: [] })
    vi.mocked(backupsApi.createBackup).mockResolvedValue({ job_id: 'job-1', type: 'create', status: 'pending' })
    vi.mocked(settingsApi.updateSetting).mockResolvedValue()
    vi.mocked(featureFlagsApi.getFeatureFlags).mockResolvedValue({
      'feature.crowdsec.console_enrollment': true,
    })
    vi.mocked(systemApi.getMyIP).mockResolvedValue({ ip: '203.0.113.1', source: 'cloudflare' })
  })

  it('shows the busy message when an install refuses the enrolment', async () => {
    enrollMock.mockRejectedValue(axiosError(409, { error: 'busy', error_code: 'crowdsec_busy', job_id: 'job-1' }))
    await renderPage()

    await submitEnrollment()

    expect(await screen.findByText(BUSY_TEXT)).toBeInTheDocument()
    expect(screen.getByTestId('console-enroll-error')).toHaveTextContent(BUSY_TEXT)
    expect(toast.error).toHaveBeenCalledWith(BUSY_TEXT)
  })

  it('keeps the server text for other enrolment refusals', async () => {
    enrollMock.mockRejectedValue(axiosError(400, { error: 'Enrollment key rejected' }))
    await renderPage()

    await submitEnrollment()

    await waitFor(() => expect(toast.error).toHaveBeenCalledWith('Enrollment key rejected'))
    expect(screen.queryByText(BUSY_TEXT)).not.toBeInTheDocument()
  })
})
