import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { AxiosError } from 'axios'
import { describe, it, expect, vi, beforeEach } from 'vitest'

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
  renderWithQueryClient(<CrowdSecConfig />)
  await screen.findByText('CrowdSec Configuration')
}

const canonicalSlug = 'crowdsecurity/Base-HTTP-Scenarios'

const pulled = {
  status: 'pulled',
  slug: canonicalSlug,
  preview: 'name: base-http-scenarios\n',
  cache_key: 'cache-pulled',
  source: 'hub',
}

const coded = (status: number, error_code: string) => axiosError(status, 'Request failed', { error: 'server text', error_code })

const selectHubAndApply = async () => {
  await renderPage()
  await waitFor(() => expect(screen.getByTestId('preset-preview')).toHaveTextContent('base-http-scenarios'))
  await userEvent.click(screen.getByTestId('apply-preset-btn'))
}

describe('CrowdSecConfig hub preset apply outcomes', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(baseStatus)
    vi.mocked(crowdsecApi.statusCrowdsec).mockResolvedValue({ running: true, pid: 123, lapi_ready: true })
    vi.mocked(crowdsecApi.listCrowdsecFiles).mockResolvedValue({ files: ['acquis.yaml'] })
    vi.mocked(crowdsecApi.readCrowdsecFile).mockResolvedValue({ content: 'file-content' })
    vi.mocked(crowdsecApi.listCrowdsecDecisions).mockResolvedValue({ decisions: [] })
    vi.mocked(presetsApi.listCrowdsecPresets).mockResolvedValue({ presets: [hubPreset] })
    vi.mocked(presetsApi.pullCrowdsecPreset).mockResolvedValue(pulled)
    vi.mocked(presetsApi.applyCrowdsecPreset).mockResolvedValue({ status: 'applied', used_cscli: true, reload_hint: false })
    vi.mocked(presetsApi.getCrowdsecPresetErrorCode).mockImplementation(
      (err) => (err as { response?: { data?: { error_code?: never } } }).response?.data?.error_code,
    )
    vi.mocked(presetsApi.getCrowdsecPresetAmbiguousTypes).mockImplementation(
      (err) => (err as { response?: { data?: { types?: string[] } } }).response?.data?.types ?? [],
    )
    vi.mocked(presetsApi.getCrowdsecPresetTaintedItems).mockImplementation(
      (err) => (err as { response?: { data?: { tainted_items?: string[] } } }).response?.data?.tainted_items ?? [],
    )
    vi.mocked(featureFlagsApi.getFeatureFlags).mockResolvedValue({ 'feature.crowdsec.console_enrollment': false })
    vi.mocked(settingsApi.updateSetting).mockResolvedValue()
  })

  it('applies the canonical slug and the cache key returned by pull', async () => {
    await selectHubAndApply()
    await waitFor(() =>
      expect(presetsApi.applyCrowdsecPreset).toHaveBeenCalledWith({ slug: canonicalSlug, cache_key: 'cache-pulled' }),
    )
    expect(toast.success).toHaveBeenCalled()
  })

  it('disables Apply for a hub preset that has no cache key', async () => {
    vi.mocked(presetsApi.listCrowdsecPresets).mockResolvedValue({ presets: [{ ...hubPreset, cached: false, cache_key: undefined }] })
    vi.mocked(presetsApi.pullCrowdsecPreset).mockRejectedValue(axiosError(500, 'pull failed'))
    await renderPage()
    await screen.findByTestId('preset-status')
    expect(screen.getByTestId('apply-preset-btn')).toBeDisabled()
  })

  it.each([
    ['preset_cache_stale', 409],
    ['preset_cache_key_required', 400],
  ])('asks for a new pull and clears the preview on %s', async (code, status) => {
    vi.mocked(presetsApi.applyCrowdsecPreset).mockRejectedValue(coded(status, code))
    await selectHubAndApply()
    expect(await screen.findByTestId('preset-status')).toHaveTextContent('Pull the preset again')
    expect(screen.getByTestId('preset-preview')).not.toHaveTextContent('base-http-scenarios')
    expect(screen.queryByTestId('preset-meta')).not.toBeInTheDocument()
    expect(screen.getByTestId('apply-preset-btn')).toBeDisabled()
    expect(screen.queryByTestId('preset-validation-error')).not.toBeInTheDocument()
    expect(toast.success).not.toHaveBeenCalled()
    expect(toast.error).not.toHaveBeenCalled()
  })

  it('lets the user pull again after a stale outcome', async () => {
    vi.mocked(presetsApi.applyCrowdsecPreset).mockRejectedValueOnce(coded(409, 'preset_cache_stale'))
    await selectHubAndApply()
    await screen.findByText('Pull the preset again', { exact: false })
    await userEvent.click(screen.getByRole('button', { name: 'Pull Preview' }))
    await waitFor(() => expect(screen.getByTestId('preset-preview')).toHaveTextContent('base-http-scenarios'))
    await waitFor(() => expect(screen.getByTestId('apply-preset-btn')).toBeEnabled())
  })

  it('says the hub is still updating and clears the preview', async () => {
    vi.mocked(presetsApi.applyCrowdsecPreset).mockRejectedValue(coded(409, 'preset_hub_updating'))
    await selectHubAndApply()
    expect(await screen.findByTestId('preset-status')).toHaveTextContent('The hub is still updating, try again later.')
    expect(screen.getByTestId('preset-preview')).not.toHaveTextContent('base-http-scenarios')
    expect(toast.error).not.toHaveBeenCalled()
  })

  it('explains a locally edited item and keeps the preview', async () => {
    vi.mocked(presetsApi.applyCrowdsecPreset).mockRejectedValue(coded(409, 'preset_local_modified'))
    await selectHubAndApply()
    expect(await screen.findByTestId('preset-status')).toHaveTextContent('left unchanged')
    expect(screen.getByTestId('preset-preview')).toHaveTextContent('base-http-scenarios')
    expect(toast.success).not.toHaveBeenCalled()
  })

  describe('edited item names', () => {
    const localModified = (tainted_items?: string[]) =>
      axiosError(409, 'Request failed', { error: 'server text', error_code: 'preset_local_modified', tainted_items })

    it('lists the edited items after the message', async () => {
      vi.mocked(presetsApi.applyCrowdsecPreset).mockRejectedValue(
        localModified(['collections/crowdsecurity/nginx', 'parsers/crowdsecurity/nginx-logs']),
      )
      await selectHubAndApply()
      const status = await screen.findByTestId('preset-status')
      expect(status).toHaveTextContent('left unchanged')
      expect(status).toHaveTextContent('Edited items: collections/crowdsecurity/nginx, parsers/crowdsecurity/nginx-logs')
      expect(toast.error).not.toHaveBeenCalled()
    })

    it('shows the first 10 names and a count of the rest', async () => {
      const names = Array.from({ length: 13 }, (_, i) => `parsers/owner/item-${i + 1}`)
      vi.mocked(presetsApi.applyCrowdsecPreset).mockRejectedValue(localModified(names))
      await selectHubAndApply()
      const status = await screen.findByTestId('preset-status')
      expect(status).toHaveTextContent('parsers/owner/item-10, and 3 more')
      expect(status).not.toHaveTextContent('item-11')
    })

    it('strips control and direction characters from names', async () => {
      vi.mocked(presetsApi.applyCrowdsecPreset).mockRejectedValue(localModified(['parsers/ow\u202Ener/it\u0007em']))
      await selectHubAndApply()
      const status = await screen.findByTestId('preset-status')
      expect(status.textContent).toContain('Edited items: parsers/owner/item')
      expect(status.textContent).not.toContain('\u202E')
      expect(status.textContent).not.toContain('\u0007')
    })

    it.each([[undefined], [[]]])('omits the list when the server sent %j', async (items) => {
      vi.mocked(presetsApi.applyCrowdsecPreset).mockRejectedValue(localModified(items))
      await selectHubAndApply()
      const status = await screen.findByTestId('preset-status')
      expect(status).toHaveTextContent('left unchanged')
      expect(status).not.toHaveTextContent('Edited items')
    })
  })

  it('shows the timeout message', async () => {
    vi.mocked(presetsApi.applyCrowdsecPreset).mockRejectedValue(coded(504, 'preset_apply_timeout'))
    await selectHubAndApply()
    expect(await screen.findByTestId('preset-status')).toHaveTextContent('The preset install took too long and was rolled back. Try again.')
    expect(screen.getByTestId('preset-preview')).toHaveTextContent('base-http-scenarios')
  })

  it('shows the unreachable-hub message and keeps the preview', async () => {
    vi.mocked(presetsApi.applyCrowdsecPreset).mockRejectedValue(coded(503, 'hub_unreachable'))
    await selectHubAndApply()
    expect(await screen.findByTestId('preset-status')).toHaveTextContent('CrowdSec could not reach the hub. Check the network connection and try again.')
    expect(screen.getByTestId('preset-preview')).toHaveTextContent('base-http-scenarios')
    expect(screen.queryByTestId('preset-hub-unavailable')).not.toBeInTheDocument()
  })

  it('shows the cscli message without marking the hub offline', async () => {
    vi.mocked(presetsApi.applyCrowdsecPreset).mockRejectedValue(coded(503, 'cscli_unavailable'))
    await selectHubAndApply()
    expect(await screen.findByTestId('preset-status')).toHaveTextContent('cscli unavailable or not responding')
    expect(screen.queryByTestId('preset-hub-unavailable')).not.toBeInTheDocument()
    expect(screen.getByTestId('apply-preset-btn')).toBeEnabled()
  })

  it('shows apply warnings as a passive note without extra toasts', async () => {
    vi.mocked(presetsApi.applyCrowdsecPreset).mockResolvedValue({
      status: 'applied',
      used_cscli: true,
      warnings: ['dependency crowdsecurity/foo was edited locally'],
    })
    await selectHubAndApply()
    const note = await screen.findByTestId('preset-apply-warnings')
    expect(note).toHaveTextContent('left unchanged')
    expect(note).toHaveTextContent('dependency crowdsecurity/foo was edited locally')
    expect(toast.success).toHaveBeenCalledTimes(1)
    expect(toast.error).not.toHaveBeenCalled()
    expect(toast.info).not.toHaveBeenCalled()
  })

  it('shows no warning note when the result has none', async () => {
    await selectHubAndApply()
    await waitFor(() => expect(toast.success).toHaveBeenCalled())
    expect(screen.queryByTestId('preset-apply-warnings')).not.toBeInTheDocument()
  })

  describe('a name that exists in several hub sections', () => {
    const crs = (type: string) => ({ ...hubPreset, slug: 'crowdsecurity/crs', title: `crs ${type}`, type, cache_key: undefined, cached: false })

    it('lists each section as its own item and sends the type of the selected one', async () => {
      vi.mocked(presetsApi.listCrowdsecPresets).mockResolvedValue({ presets: [crs('appsec-rules'), crs('appsec-configs')] })
      await renderPage()
      await waitFor(() => expect(presetsApi.pullCrowdsecPreset).toHaveBeenCalledWith('crowdsecurity/crs', 'appsec-rules'))

      await userEvent.click(screen.getByText('crs appsec-configs'))

      await waitFor(() => expect(presetsApi.pullCrowdsecPreset).toHaveBeenCalledWith('crowdsecurity/crs', 'appsec-configs'))
      expect(screen.getAllByTestId('preset-type').map((el) => el.textContent).sort()).toEqual(['appsec-configs', 'appsec-rules'])
    })

    it('applies the type that was pulled', async () => {
      vi.mocked(presetsApi.listCrowdsecPresets).mockResolvedValue({ presets: [crs('appsec-rules')] })
      vi.mocked(presetsApi.pullCrowdsecPreset).mockResolvedValue({ ...pulled, slug: 'crowdsecurity/crs', type: 'appsec-rules' })
      await renderPage()
      await waitFor(() => expect(screen.getByTestId('apply-preset-btn')).toBeEnabled())
      await userEvent.click(screen.getByTestId('apply-preset-btn'))

      await waitFor(() =>
        expect(presetsApi.applyCrowdsecPreset).toHaveBeenCalledWith({
          slug: 'crowdsecurity/crs',
          type: 'appsec-rules',
          cache_key: 'cache-pulled',
        }),
      )
    })

    it('asks which section to use when the server cannot tell, then pulls that one', async () => {
      vi.mocked(presetsApi.listCrowdsecPresets).mockResolvedValue({ presets: [{ ...crs('x'), type: undefined }] })
      vi.mocked(presetsApi.pullCrowdsecPreset).mockRejectedValueOnce(
        axiosError(409, 'Request failed', { error: 'x', error_code: 'preset_type_ambiguous', types: ['appsec-configs', 'appsec-rules'] }),
      )
      await renderPage()
      expect(await screen.findByTestId('preset-status')).toHaveTextContent('several hub sections')
      const picker = await screen.findByLabelText('Item type')

      await userEvent.selectOptions(picker, 'appsec-rules')

      await waitFor(() => expect(presetsApi.pullCrowdsecPreset).toHaveBeenLastCalledWith('crowdsecurity/crs', 'appsec-rules'))
      await waitFor(() => expect(screen.queryByTestId('preset-type-picker')).not.toBeInTheDocument())
    })

    it('says the choice is needed when an apply reports the ambiguity', async () => {
      vi.mocked(presetsApi.applyCrowdsecPreset).mockRejectedValue(coded(409, 'preset_type_ambiguous'))
      await selectHubAndApply()
      expect(await screen.findByTestId('preset-status')).toHaveTextContent('several hub sections')
    })
  })
})
