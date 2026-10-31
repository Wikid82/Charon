import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, it, expect, vi, beforeEach } from 'vitest'

import * as crowdsecApi from '../../api/crowdsec'
import * as featureFlagsApi from '../../api/featureFlags'
import * as presetsApi from '../../api/presets'
import * as securityApi from '../../api/security'
import * as settingsApi from '../../api/settings'
import { renderWithQueryClient } from '../../test-utils/renderWithQueryClient'
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
    mutateAsync: vi.fn().mockResolvedValue({ status: 'enrolling', key_present: false }),
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

/** Hub-backed row that is not cached and cannot be fetched while the hub is offline. */
const OFFLINE_HUB_ROW: presetsApi.CrowdsecPresetSummary = {
  slug: 'crowdsecurity/offline-hub-preset',
  title: 'Offline Hub Preset',
  summary: 'Hub only',
  source: 'hub',
  requires_hub: true,
  available: false,
  cached: false,
}

/** Second unavailable hub row, so tests can cover a list where nothing is selectable. */
const SECOND_OFFLINE_HUB_ROW: presetsApi.CrowdsecPresetSummary = {
  slug: 'crowdsecurity/second-offline-hub-preset',
  title: 'Second Offline Hub Preset',
  summary: 'Hub only too',
  source: 'hub',
  requires_hub: true,
  available: false,
  cached: false,
}

/** Built-in preset that needs no hub, so it stays selectable offline. */
const CURATED_ROW: presetsApi.CrowdsecPresetSummary = {
  slug: 'crowdsecurity/selectable-curated-preset',
  title: 'Selectable Curated Preset',
  summary: 'Built in',
  source: 'charon-curated',
  requires_hub: false,
  available: true,
  cached: false,
}

/** Hub-backed row that is online (listed as available) and not cached, so it is selectable. */
const ONLINE_HUB_ROW: presetsApi.CrowdsecPresetSummary = {
  slug: 'crowdsecurity/online-hub-preset',
  title: 'Online Hub Preset',
  summary: 'Hub, reachable',
  source: 'hub',
  requires_hub: true,
  available: true,
  cached: false,
}

/** Hub-backed row that is offline but already cached, so it still works. */
const CACHED_OFFLINE_HUB_ROW: presetsApi.CrowdsecPresetSummary = {
  slug: 'crowdsecurity/cached-offline-hub-preset',
  title: 'Cached Offline Hub Preset',
  summary: 'Cached hub',
  source: 'hub',
  requires_hub: true,
  available: false,
  cached: true,
  cache_key: 'cache-cached',
}

const HUB_OFFLINE_NOTE = 'Showing built-in and downloaded presets. The hub list was not loaded.'
const UNAVAILABLE_OFFLINE_NOTE = 'Needs the CrowdSec hub. Not available while the hub is offline.'

const pulledFor = (slug: string): presetsApi.PullCrowdsecPresetResponse => ({
  status: 'pulled',
  slug,
  preview: `name: ${slug}\n`,
  cache_key: `cache-${slug}`,
  source: 'hub',
})

/** The row whose accessible name starts with the title (anchored, so "Offline Hub Preset" does not match "Second Offline Hub Preset"). */
const startsWithTitle = (title: string) => (accessibleName: string) => accessibleName.startsWith(title)
const presetRow = (title: string) => screen.getByRole('button', { name: startsWithTitle(title) })
const findPresetRow = (title: string) => screen.findByRole('button', { name: startsWithTitle(title) })

/** Every slug the page asked to pull, in call order. */
const pulledSlugs = (): string[] => vi.mocked(presetsApi.pullCrowdsecPreset).mock.calls.map(([slug]) => slug)

const renderPage = async () => {
  const view = renderWithQueryClient(<CrowdSecConfig />)
  await screen.findByText('CrowdSec Configuration')
  return view
}

describe('CrowdSecConfig presets served without the hub', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(baseStatus)
    vi.mocked(crowdsecApi.statusCrowdsec).mockResolvedValue({ running: true, pid: 123, lapi_ready: true })
    vi.mocked(crowdsecApi.listCrowdsecFiles).mockResolvedValue({ files: ['acquis.yaml'] })
    vi.mocked(crowdsecApi.readCrowdsecFile).mockResolvedValue({ content: 'file-content' })
    vi.mocked(crowdsecApi.listCrowdsecDecisions).mockResolvedValue({ decisions: [] })
    vi.mocked(presetsApi.listCrowdsecPresets).mockResolvedValue({ presets: [] })
    vi.mocked(presetsApi.pullCrowdsecPreset).mockImplementation(async (slug) => pulledFor(slug))
    vi.mocked(presetsApi.getCurrentApplyJob).mockResolvedValue(null)
    vi.mocked(featureFlagsApi.getFeatureFlags).mockResolvedValue({ 'feature.crowdsec.console_enrollment': false })
    vi.mocked(settingsApi.updateSetting).mockResolvedValue()
  })

  describe('hub offline note', () => {
    it('shows the muted note with no button, banner or retry when hub_offline is true', async () => {
      vi.mocked(presetsApi.listCrowdsecPresets).mockResolvedValue({ presets: [CURATED_ROW], hub_offline: true })
      await renderPage()

      const note = await screen.findByTestId('preset-hub-offline')
      expect(note).toHaveTextContent(HUB_OFFLINE_NOTE)
      expect(within(note).queryByRole('button')).not.toBeInTheDocument()
      expect(screen.queryByTestId('preset-hub-unavailable')).not.toBeInTheDocument()
      expect(screen.queryByRole('button', { name: 'Retry' })).not.toBeInTheDocument()
    })

    it('shows no note when hub_offline is false', async () => {
      vi.mocked(presetsApi.listCrowdsecPresets).mockResolvedValue({ presets: [CURATED_ROW, OFFLINE_HUB_ROW], hub_offline: false })
      await renderPage()

      await waitFor(() => expect(presetRow(CURATED_ROW.title)).toBeInTheDocument())
      expect(screen.queryByTestId('preset-hub-offline')).not.toBeInTheDocument()
    })

    it('shows no note when the server omits hub_offline (older backend)', async () => {
      vi.mocked(presetsApi.listCrowdsecPresets).mockResolvedValue({ presets: [CURATED_ROW] })
      await renderPage()

      await waitFor(() => expect(presetRow(CURATED_ROW.title)).toBeInTheDocument())
      expect(screen.queryByTestId('preset-hub-offline')).not.toBeInTheDocument()
    })
  })

  describe('unavailable hub rows', () => {
    it('marks an unavailable hub row aria-disabled with its note and keeps it unpressed', async () => {
      vi.mocked(presetsApi.listCrowdsecPresets).mockResolvedValue({ presets: [OFFLINE_HUB_ROW, CURATED_ROW], hub_offline: true })
      await renderPage()

      const row = await findPresetRow(OFFLINE_HUB_ROW.title)
      expect(row).toHaveAttribute('aria-disabled', 'true')
      expect(row).toHaveAttribute('aria-pressed', 'false')
      expect(within(row).getByTestId('preset-unavailable-offline')).toHaveTextContent(UNAVAILABLE_OFFLINE_NOTE)
    })

    it('does not mark a selectable row as disabled', async () => {
      vi.mocked(presetsApi.listCrowdsecPresets).mockResolvedValue({ presets: [CURATED_ROW], hub_offline: true })
      await renderPage()

      const row = await findPresetRow(CURATED_ROW.title)
      await waitFor(() => expect(row).toHaveAttribute('aria-pressed', 'true'))
      expect(row).not.toHaveAttribute('aria-disabled')
      expect(row).not.toHaveTextContent(UNAVAILABLE_OFFLINE_NOTE)
    })

    it('does not select an unavailable row on click and sends no pull for it', async () => {
      vi.mocked(presetsApi.listCrowdsecPresets).mockResolvedValue({ presets: [OFFLINE_HUB_ROW, CURATED_ROW], hub_offline: true })
      await renderPage()
      await waitFor(() => expect(presetRow(CURATED_ROW.title)).toHaveAttribute('aria-pressed', 'true'))

      await userEvent.click(presetRow(OFFLINE_HUB_ROW.title))

      expect(presetRow(OFFLINE_HUB_ROW.title)).toHaveAttribute('aria-pressed', 'false')
      expect(presetRow(CURATED_ROW.title)).toHaveAttribute('aria-pressed', 'true')
      expect(pulledSlugs()).not.toContain(OFFLINE_HUB_ROW.slug)
    })

    it('does not select an unavailable row with the keyboard', async () => {
      vi.mocked(presetsApi.listCrowdsecPresets).mockResolvedValue({ presets: [OFFLINE_HUB_ROW, CURATED_ROW], hub_offline: true })
      await renderPage()
      await waitFor(() => expect(presetRow(CURATED_ROW.title)).toHaveAttribute('aria-pressed', 'true'))

      presetRow(OFFLINE_HUB_ROW.title).focus()
      await userEvent.keyboard('{Enter}')
      await userEvent.keyboard(' ')

      expect(presetRow(OFFLINE_HUB_ROW.title)).toHaveAttribute('aria-pressed', 'false')
      expect(presetRow(CURATED_ROW.title)).toHaveAttribute('aria-pressed', 'true')
      expect(pulledSlugs()).not.toContain(OFFLINE_HUB_ROW.slug)
    })

    it('keeps a cached hub row selectable while the hub is offline', async () => {
      vi.mocked(presetsApi.listCrowdsecPresets).mockResolvedValue({ presets: [CACHED_OFFLINE_HUB_ROW, OFFLINE_HUB_ROW], hub_offline: true })
      await renderPage()

      const cachedRow = await findPresetRow(CACHED_OFFLINE_HUB_ROW.title)
      await waitFor(() => expect(cachedRow).toHaveAttribute('aria-pressed', 'true'))
      expect(cachedRow).not.toHaveAttribute('aria-disabled')
      expect(within(cachedRow).queryByTestId('preset-unavailable-offline')).not.toBeInTheDocument()
    })

    it('makes a hub row selectable again once the hub is back and the list refreshes', async () => {
      vi.mocked(presetsApi.listCrowdsecPresets).mockResolvedValue({ presets: [OFFLINE_HUB_ROW, CURATED_ROW], hub_offline: true })
      const { queryClient } = await renderPage()
      await waitFor(() => expect(presetRow(CURATED_ROW.title)).toHaveAttribute('aria-pressed', 'true'))

      vi.mocked(presetsApi.listCrowdsecPresets).mockResolvedValue({
        presets: [{ ...OFFLINE_HUB_ROW, available: true }, CURATED_ROW],
        hub_offline: false,
      })
      await queryClient.invalidateQueries({ queryKey: ['crowdsec-presets'] })

      await waitFor(() => expect(presetRow(OFFLINE_HUB_ROW.title)).not.toHaveAttribute('aria-disabled'))
      expect(screen.queryByTestId('preset-hub-offline')).not.toBeInTheDocument()
      expect(presetRow(CURATED_ROW.title)).toHaveAttribute('aria-pressed', 'true')
    })
  })

  describe('auto-select', () => {
    it('skips an unavailable first row and selects the first selectable row', async () => {
      vi.mocked(presetsApi.listCrowdsecPresets).mockResolvedValue({ presets: [OFFLINE_HUB_ROW, CURATED_ROW], hub_offline: true })
      await renderPage()

      await waitFor(() => expect(presetRow(CURATED_ROW.title)).toHaveAttribute('aria-pressed', 'true'))
      expect(presetRow(OFFLINE_HUB_ROW.title)).toHaveAttribute('aria-pressed', 'false')
      await waitFor(() => expect(pulledSlugs()).toContain(CURATED_ROW.slug))
      expect(pulledSlugs()).not.toContain(OFFLINE_HUB_ROW.slug)
    })

    it('selects nothing and sends no pull when every row is unavailable', async () => {
      vi.mocked(presetsApi.listCrowdsecPresets).mockResolvedValue({
        presets: [OFFLINE_HUB_ROW, SECOND_OFFLINE_HUB_ROW],
        hub_offline: true,
      })
      await renderPage()

      for (const row of [OFFLINE_HUB_ROW, SECOND_OFFLINE_HUB_ROW]) {
        await waitFor(() => expect(presetRow(row.title)).toHaveAttribute('aria-disabled', 'true'))
        expect(presetRow(row.title)).toHaveAttribute('aria-pressed', 'false')
      }
      expect(screen.queryByRole('alert')).not.toBeInTheDocument()
      expect(screen.queryByTestId('preset-hub-unavailable')).not.toBeInTheDocument()
      expect(screen.getByRole('button', { name: 'Pull Preview' })).toBeDisabled()
      expect(pulledSlugs()).toEqual([])
    })

    it('moves the selection to the first selectable row when the selected row becomes unavailable', async () => {
      vi.mocked(presetsApi.listCrowdsecPresets).mockResolvedValue({ presets: [ONLINE_HUB_ROW, CURATED_ROW], hub_offline: false })
      const { queryClient } = await renderPage()
      await waitFor(() => expect(presetRow(ONLINE_HUB_ROW.title)).toHaveAttribute('aria-pressed', 'true'))

      vi.mocked(presetsApi.listCrowdsecPresets).mockResolvedValue({
        presets: [{ ...ONLINE_HUB_ROW, available: false }, CURATED_ROW],
        hub_offline: true,
      })
      await queryClient.invalidateQueries({ queryKey: ['crowdsec-presets'] })

      await waitFor(() => expect(presetRow(CURATED_ROW.title)).toHaveAttribute('aria-pressed', 'true'))
      expect(presetRow(ONLINE_HUB_ROW.title)).toHaveAttribute('aria-pressed', 'false')
      expect(presetRow(ONLINE_HUB_ROW.title)).toHaveAttribute('aria-disabled', 'true')
    })
  })
})
