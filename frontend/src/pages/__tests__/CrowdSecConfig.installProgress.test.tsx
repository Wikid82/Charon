import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { AxiosError } from 'axios'
import { describe, it, expect, vi, beforeEach } from 'vitest'

import * as crowdsecApi from '../../api/crowdsec'
import * as featureFlagsApi from '../../api/featureFlags'
import * as presetsApi from '../../api/presets'
import * as securityApi from '../../api/security'
import { renderWithQueryClient } from '../../test-utils/renderWithQueryClient'
import CrowdSecConfig from '../CrowdSecConfig'

vi.mock('../../api/security')
vi.mock('../../api/crowdsec')
// Functions are mocked; constants such as PRESET_ERROR_CODES keep their real values.
vi.mock('../../api/presets', async (importOriginal) =>
  Object.fromEntries(Object.entries(await importOriginal<typeof presetsApi>()).map(([name, value]) => [name, typeof value === 'function' ? vi.fn() : value])),
)
vi.mock('../../api/featureFlags')
vi.mock('../../components/CrowdSecBouncerKeyDisplay', () => ({ CrowdSecBouncerKeyDisplay: () => null }))
vi.mock('../../utils/toast', () => ({ toast: { success: vi.fn(), error: vi.fn(), info: vi.fn() } }))

const hubPreset = {
  slug: 'crowdsecurity/base-http-scenarios',
  title: 'Bot Mitigation Essentials',
  summary: 'Hub preset',
  source: 'hub',
  requires_hub: true,
  available: true,
  cached: true,
  cache_key: 'cache-hub',
}

const job = (overrides: Partial<presetsApi.CrowdsecApplyJob> = {}): presetsApi.CrowdsecApplyJob => ({
  job_id: 'job-1',
  slug: hubPreset.slug,
  item_type: 'collections',
  cache_key: 'cache-hub',
  state: 'running',
  phase: 'install',
  warnings: [],
  items_installed: 0,
  items_total: null,
  cancel_requested: false,
  elapsed_seconds: 0,
  created_at: '2026-10-18T10:00:00Z',
  started_at: '2026-10-18T10:00:00Z',
  finished_at: null,
  ...overrides,
})

const axiosError = (status: number, data: Record<string, unknown>) =>
  new AxiosError('Request failed', undefined, undefined, undefined, { status, statusText: String(status), headers: {}, config: {}, data } as never)

const renderPage = async () => {
  renderWithQueryClient(<CrowdSecConfig />)
  await screen.findByText('CrowdSec Configuration')
}

const startApply = async () => {
  await renderPage()
  await waitFor(() => expect(screen.getByTestId('preset-preview')).toHaveTextContent('base-http-scenarios'))
  await userEvent.click(screen.getByTestId('apply-preset-btn'))
}

const progressRow = () => screen.findByRole('status', { name: 'Installing preset' })

describe('CrowdSecConfig background preset install', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.mocked(securityApi.getSecurityStatus).mockResolvedValue({
      cerberus: { enabled: true },
      crowdsec: { enabled: true, mode: 'local' as const, api_url: '' },
      waf: { enabled: true, mode: 'enabled' as const },
      rate_limit: { enabled: true },
      acl: { enabled: true },
    })
    vi.mocked(crowdsecApi.statusCrowdsec).mockResolvedValue({ running: true, pid: 1, lapi_ready: true })
    vi.mocked(crowdsecApi.listCrowdsecFiles).mockResolvedValue({ files: [] })
    vi.mocked(crowdsecApi.listCrowdsecDecisions).mockResolvedValue({ decisions: [] })
    vi.mocked(presetsApi.listCrowdsecPresets).mockResolvedValue({ presets: [hubPreset] })
    vi.mocked(presetsApi.pullCrowdsecPreset).mockResolvedValue({
      status: 'pulled',
      slug: hubPreset.slug,
      preview: 'name: base-http-scenarios\n',
      cache_key: 'cache-pulled',
      source: 'hub',
    })
    vi.mocked(presetsApi.getCurrentApplyJob).mockResolvedValue(null)
    vi.mocked(presetsApi.applyPresetAsync).mockResolvedValue(job({ state: 'queued', phase: 'preflight' }))
    vi.mocked(presetsApi.getApplyJob).mockResolvedValue(job({ phase: 'install', items_installed: 12, elapsed_seconds: 200 }))
    vi.mocked(presetsApi.getApplyJobErrorCode).mockReturnValue(undefined)
    vi.mocked(presetsApi.getCrowdsecBusyJobId).mockReturnValue(undefined)
    vi.mocked(presetsApi.getCrowdsecPresetErrorCode).mockImplementation(
      (err) => (err as { response?: { data?: { error_code?: never } } }).response?.data?.error_code,
    )
    vi.mocked(presetsApi.getCrowdsecPresetTaintedItems).mockReturnValue([])
    vi.mocked(presetsApi.getCrowdsecPresetAmbiguousTypes).mockReturnValue([])
    vi.mocked(featureFlagsApi.getFeatureFlags).mockResolvedValue({ 'feature.crowdsec.console_enrollment': false })
  })

  it('starts the install as a job and shows a passive progress row', async () => {
    await startApply()

    const row = await progressRow()
    expect(presetsApi.applyPresetAsync).toHaveBeenCalledWith(expect.objectContaining({ slug: hubPreset.slug, cache_key: 'cache-pulled' }))
    expect(presetsApi.applyCrowdsecPreset).not.toHaveBeenCalled()
    await waitFor(() => expect(row).toHaveTextContent('Installing, 12 items so far, 3 min 20 s'))
    expect(within(row).getByRole('button', { name: 'Cancel' })).toBeEnabled()
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
    expect(screen.queryByRole('progressbar')).not.toBeInTheDocument()
    expect(screen.getByTestId('apply-preset-btn')).toBeDisabled()
  })

  it('shows the phase text of each running phase', async () => {
    vi.mocked(presetsApi.getApplyJob).mockResolvedValue(job({ state: 'verifying', phase: 'verify', items_installed: 4, elapsed_seconds: 5 }))
    await startApply()
    await waitFor(async () => expect(await progressRow()).toHaveTextContent('Verifying, 4 items so far, 5 s'))
  })

  it('cancels the running job from the row', async () => {
    vi.mocked(presetsApi.cancelApplyJob).mockResolvedValue(job({ cancel_requested: true, phase: 'rollback' }))
    await startApply()
    const row = await progressRow()

    await userEvent.click(within(row).getByRole('button', { name: 'Cancel' }))

    await waitFor(() => expect(vi.mocked(presetsApi.cancelApplyJob).mock.calls[0][0]).toBe('job-1'))
    await waitFor(() => expect(within(row).getByRole('button', { name: 'Cancelling...' })).toBeDisabled())
  })

  it('resumes an active job after a reload without starting a new one', async () => {
    vi.mocked(presetsApi.getCurrentApplyJob).mockResolvedValue(job({ items_installed: 18, elapsed_seconds: 95 }))
    vi.mocked(presetsApi.getApplyJob).mockResolvedValue(job({ items_installed: 18, elapsed_seconds: 95 }))

    await renderPage()

    expect(await progressRow()).toHaveTextContent('Installing, 18 items so far, 1 min 35 s')
    expect(presetsApi.applyPresetAsync).not.toHaveBeenCalled()
  })

  it('says it is waiting for the server when a poll fails and keeps the last numbers', async () => {
    vi.mocked(presetsApi.getApplyJob).mockRejectedValue(new Error('network'))
    vi.mocked(presetsApi.getCurrentApplyJob).mockResolvedValue(job({ items_installed: 7, elapsed_seconds: 8 }))

    await renderPage()

    await waitFor(async () => expect(await progressRow()).toHaveTextContent('Waiting for the server...'))
    expect(await progressRow()).toHaveTextContent('7 items so far')
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  })

  it('shows the success result once and enables Apply again', async () => {
    vi.mocked(presetsApi.getApplyJob).mockResolvedValue(
      job({ state: 'succeeded', phase: 'reload', items_installed: 41, elapsed_seconds: 360, finished_at: '2026-10-18T10:06:00Z' }),
    )
    await startApply()

    const result = await screen.findByTestId('preset-install-result')
    expect(result).toHaveTextContent('Preset applied. 41 items installed')
    expect(screen.queryByRole('status', { name: 'Installing preset' })).not.toBeInTheDocument()
    expect(screen.getByTestId('apply-preset-btn')).toBeEnabled()
  })

  it('shows a recently finished job from a previous visit', async () => {
    vi.mocked(presetsApi.getCurrentApplyJob).mockResolvedValue(job({ state: 'succeeded', items_installed: 3, finished_at: '2026-10-18T10:06:00Z' }))

    await renderPage()

    expect(await screen.findByTestId('preset-install-result')).toHaveTextContent('Preset applied.')
    expect(screen.queryByRole('button', { name: 'Cancel' })).not.toBeInTheDocument()
  })

  it.each([
    ['preset_local_modified', 'was edited on this server'],
    ['preset_hub_updating', 'hub is still updating'],
    ['preset_cache_stale', 'preview is out of date'],
    ['hub_unreachable', 'could not reach the hub'],
    ['cscli_unavailable', 'cscli unavailable'],
  ] as const)('maps the %s failure to its text and notes the rollback', async (code, text) => {
    vi.mocked(presetsApi.getApplyJob).mockResolvedValue(job({ state: 'failed', phase: 'rollback', error_code: code }))
    await startApply()

    const result = await screen.findByTestId('preset-install-result')
    expect(result).toHaveTextContent('Install failed.')
    expect(result).toHaveTextContent(text)
    expect(result).toHaveTextContent('The previous configuration was restored.')
  })

  it('uses a generic text for a failure without a known code', async () => {
    vi.mocked(presetsApi.getApplyJob).mockResolvedValue(job({ state: 'failed', phase: 'rollback', error: 'boom' }))
    await startApply()

    expect(await screen.findByTestId('preset-install-result')).toHaveTextContent('Install failed. The preset could not be installed.')
  })

  it.each([
    ['cancelled', undefined, 'Install cancelled.'],
    ['timed_out', 'preset_apply_timeout', 'The install took too long and was stopped.'],
    ['failed', 'interrupted', 'The install was interrupted by a restart.'],
  ] as const)('explains the %s state with a rollback note', async (state, code, text) => {
    vi.mocked(presetsApi.getApplyJob).mockResolvedValue(job({ state, phase: 'rollback', error_code: code }))
    await startApply()

    const result = await screen.findByTestId('preset-install-result')
    expect(result).toHaveTextContent(text)
    expect(result).toHaveTextContent('The previous configuration was restored.')
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  })

  it('shows the busy text when another install holds the lock', async () => {
    vi.mocked(presetsApi.applyPresetAsync).mockRejectedValue(axiosError(409, { error: 'busy', error_code: 'crowdsec_busy', job_id: 'job-1' }))
    await startApply()

    expect(await screen.findByTestId('preset-status')).toHaveTextContent('CrowdSec is busy installing a preset. Try again when it finishes.')
    expect(screen.queryByTestId('preset-install-result')).not.toBeInTheDocument()
  })
})
