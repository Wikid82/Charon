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
import { renderWithQueryClient } from '../../test-utils/renderWithQueryClient'
import { toast } from '../../utils/toast'
import CrowdSecConfig from '../CrowdSecConfig'

vi.mock('../../api/security')
vi.mock('../../api/crowdsec')
vi.mock('../../api/presets')
vi.mock('../../api/backups')
vi.mock('../../api/settings')
vi.mock('../../api/featureFlags')
vi.mock('../../hooks/useConsoleEnrollment', () => ({
  useConsoleStatus: vi.fn(() => ({
    data: {
      status: 'not_enrolled',
      tenant: 'default',
      agent_name: 'charon-agent',
      last_error: null,
      last_attempt_at: null,
      enrolled_at: null,
      last_heartbeat_at: null,
      key_present: false,
      correlation_id: 'corr-1',
    },
    isLoading: false,
    isRefetching: false,
  })),
  useEnrollConsole: vi.fn(() => ({
    mutateAsync: vi.fn().mockResolvedValue({
      status: 'enrolling',
      key_present: false,
    }),
    isPending: false,
  })),
  useClearConsoleEnrollment: vi.fn(() => ({
    mutate: vi.fn(),
    isPending: false,
  })),
}))
vi.mock('../../components/CrowdSecBouncerKeyDisplay', () => ({
  CrowdSecBouncerKeyDisplay: () => null,
}))
vi.mock('../../utils/crowdsecExport', () => ({
  buildCrowdsecExportFilename: vi.fn(() => 'crowdsec-default.tar.gz'),
  promptCrowdsecFilename: vi.fn(() => 'crowdsec.tar.gz'),
  downloadCrowdsecExport: vi.fn(),
}))
vi.mock('../../utils/toast', () => ({
  toast: {
    success: vi.fn(),
    error: vi.fn(),
    info: vi.fn(),
  },
}))

const baseStatus = {
  cerberus: { enabled: true },
  crowdsec: { enabled: true, mode: 'local' as const, api_url: '' },
  waf: { enabled: true, mode: 'enabled' as const },
  rate_limit: { enabled: true },
  acl: { enabled: true },
}

const axiosError = (status: number, message: string, data?: Record<string, unknown>) =>
  new AxiosError(message, undefined, undefined, undefined, {
    status,
    statusText: String(status),
    headers: {},
    config: {},
    data: data ?? { error: message },
  } as never)

const curatedPreset = {
  slug: 'geoip-enrichment',
  title: 'GeoIP Enrichment',
  summary: 'Enriches CrowdSec log events with GeoIP data.',
  source: 'charon-curated',
  requires_hub: false,
  available: true,
  cached: false,
  cache_key: 'curated-geoip-enrichment',
}

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

const renderPage = async () => {
  const result = renderWithQueryClient(<CrowdSecConfig />)
  await waitFor(() => screen.getByText('CrowdSec Configuration'))
  return result
}

describe('CrowdSecConfig curated preset apply', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(baseStatus)
    vi.mocked(crowdsecApi.statusCrowdsec).mockResolvedValue({ running: true, pid: 123, lapi_ready: true })
    vi.mocked(crowdsecApi.listCrowdsecFiles).mockResolvedValue({ files: ['acquis.yaml'] })
    vi.mocked(crowdsecApi.readCrowdsecFile).mockResolvedValue({ content: 'file-content' })
    vi.mocked(crowdsecApi.writeCrowdsecFile).mockResolvedValue(undefined)
    vi.mocked(crowdsecApi.listCrowdsecDecisions).mockResolvedValue({ decisions: [] })
    vi.mocked(presetsApi.listCrowdsecPresets).mockResolvedValue({ presets: [curatedPreset] })
    vi.mocked(presetsApi.pullCrowdsecPreset).mockResolvedValue({
      status: 'pulled',
      slug: curatedPreset.slug,
      preview: 'parsers: crowdsecurity/geoip-enrich',
      cache_key: curatedPreset.cache_key,
      source: 'charon-curated',
    })
    vi.mocked(presetsApi.applyCrowdsecPreset).mockResolvedValue({
      status: 'applied',
      backup: 'crowdsec.backup.20260101-000000.000000',
      reload_hint: true,
      used_cscli: true,
      cache_key: curatedPreset.cache_key,
      slug: curatedPreset.slug,
    })
    vi.mocked(backupsApi.createBackup).mockResolvedValue({ job_id: 'job-1', type: 'create', status: 'pending' })
    vi.mocked(settingsApi.updateSetting).mockResolvedValue()
    vi.mocked(featureFlagsApi.getFeatureFlags).mockResolvedValue({
      'feature.crowdsec.console_enrollment': false,
    })
  })

  it('shows the success toast only when the server reports applied', async () => {
    await renderPage()
    await userEvent.click(screen.getByTestId('apply-preset-btn'))
    await waitFor(() => expect(toast.success).toHaveBeenCalledWith('Preset applied via backend (reload required)'))
    expect(screen.getByTestId('preset-apply-info')).toHaveTextContent('Backup: crowdsec.backup.20260101-000000.000000 (stored next to the CrowdSec data directory)')
    expect(screen.getByTestId('preset-apply-info').textContent).not.toMatch(/(^|\s)\//)
    expect(toast.error).not.toHaveBeenCalled()
  })

  it.each(['failed', 'pending', ''])('never shows success for a 2xx response with status %j', async (status) => {
    vi.mocked(presetsApi.applyCrowdsecPreset).mockResolvedValue({ status })
    await renderPage()
    await userEvent.click(screen.getByTestId('apply-preset-btn'))
    await waitFor(() => expect(toast.error).toHaveBeenCalledWith(expect.stringContaining('did not complete')))
    expect(toast.success).not.toHaveBeenCalled()
    expect(crowdsecApi.writeCrowdsecFile).not.toHaveBeenCalled()
  })

  it.each([400, 500, 501, 503, 504])(
    'surfaces the server error and never applies locally for curated presets on %i',
    async (status) => {
      vi.mocked(presetsApi.applyCrowdsecPreset).mockRejectedValue(
        axiosError(status, 'request failed', { error: `server says ${status}`, backup: 'crowdsec.backup.20260101-000000.000000' }),
      )
      await renderPage()
      await userEvent.click(screen.getByTestId('apply-preset-btn'))
      await waitFor(() =>
        expect(toast.error).toHaveBeenCalledWith(
          `Apply failed: server says ${status}. Backup created: crowdsec.backup.20260101-000000.000000 (stored next to the CrowdSec data directory)`,
        ),
      )
      expect(toast.success).not.toHaveBeenCalled()
      expect(toast.info).not.toHaveBeenCalled()
      expect(backupsApi.createBackup).not.toHaveBeenCalled()
      expect(crowdsecApi.writeCrowdsecFile).not.toHaveBeenCalled()
      expect(screen.getByTestId('preset-apply-info')).toHaveTextContent('failed')
    },
  )

  it('records a validation error for curated 400 responses', async () => {
    vi.mocked(presetsApi.applyCrowdsecPreset).mockRejectedValue(axiosError(400, 'bad', { error: 'server says 400' }))
    await renderPage()
    await userEvent.click(screen.getByTestId('apply-preset-btn'))
    expect(await screen.findByTestId('preset-validation-error')).toHaveTextContent('server says 400')
  })

  it('falls back to the error message when the server omits an error body', async () => {
    vi.mocked(presetsApi.applyCrowdsecPreset).mockRejectedValue(axiosError(501, 'Request failed', {}))
    await renderPage()
    await userEvent.click(screen.getByTestId('apply-preset-btn'))
    await waitFor(() => expect(toast.error).toHaveBeenCalledWith('Apply failed: Request failed'))
    expect(crowdsecApi.writeCrowdsecFile).not.toHaveBeenCalled()
  })

  it.each([
    [501, 'apply unsupported'],
    [503, 'hub unavailable upstream'],
    [500, 'apply blew up'],
    [504, 'upstream timed out'],
  ])('surfaces the server message for hub preset %i and performs no file write', async (status, message) => {
    vi.mocked(presetsApi.listCrowdsecPresets).mockResolvedValue({ presets: [hubPreset] })
    vi.mocked(presetsApi.pullCrowdsecPreset).mockResolvedValue({
      status: 'pulled',
      slug: hubPreset.slug,
      preview: 'configs:\n  collections:\n    - crowdsecurity/base-http-scenarios\n',
      cache_key: hubPreset.cache_key,
      source: 'hub',
    })
    vi.mocked(presetsApi.applyCrowdsecPreset).mockRejectedValue(axiosError(status, 'Request failed', { error: message }))
    await renderPage()
    await userEvent.selectOptions(screen.getByTestId('crowdsec-file-select'), 'acquis.yaml')
    await waitFor(() => expect(screen.getByTestId('preset-preview')).toHaveTextContent('base-http-scenarios'))
    await userEvent.click(screen.getByTestId('apply-preset-btn'))
    await waitFor(() => expect(toast.error).toHaveBeenCalledWith(`Apply failed: ${message}`))
    expect(toast.success).not.toHaveBeenCalled()
    expect(toast.info).not.toHaveBeenCalled()
    expect(crowdsecApi.writeCrowdsecFile).not.toHaveBeenCalled()
    expect(backupsApi.createBackup).not.toHaveBeenCalled()
  })

  it('shows a generic error for non-HTTP failures on curated presets', async () => {
    vi.mocked(presetsApi.applyCrowdsecPreset).mockRejectedValue(new Error('network down'))
    await renderPage()
    await userEvent.click(screen.getByTestId('apply-preset-btn'))
    await waitFor(() => expect(toast.error).toHaveBeenCalledWith('Failed to apply preset'))
    expect(crowdsecApi.writeCrowdsecFile).not.toHaveBeenCalled()
  })

  it('shows the static curated metadata only when the server list fails to load', async () => {
    vi.mocked(presetsApi.listCrowdsecPresets).mockRejectedValue(new Error('list failed'))
    await renderPage()
    expect(await screen.findByRole('button', { name: /GeoIP Enrichment/i })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /Geolocation/i })).not.toBeInTheDocument()
  })
})
