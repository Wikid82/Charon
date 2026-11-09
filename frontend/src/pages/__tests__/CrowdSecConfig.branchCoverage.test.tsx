import { screen, waitFor, within } from '@testing-library/react'
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
import { CROWDSEC_PRESETS } from '../../data/crowdsecPresets'
import * as consoleHooks from '../../hooks/useConsoleEnrollment'
import { renderWithQueryClient } from '../../test-utils/renderWithQueryClient'
import { toast } from '../../utils/toast'
import CrowdSecConfig from '../CrowdSecConfig'

const { enrollMock } = vi.hoisted(() => ({ enrollMock: vi.fn() }))

vi.mock('../../api/security')
vi.mock('../../api/crowdsec')
vi.mock('../../api/presets')
vi.mock('../../api/backups')
vi.mock('../../api/settings')
vi.mock('../../api/featureFlags')
vi.mock('../../api/system')
vi.mock('../../hooks/useConsoleEnrollment', () => ({
  useConsoleStatus: vi.fn(),
  useEnrollConsole: vi.fn(() => ({ mutateAsync: enrollMock, isPending: false })),
  useClearConsoleEnrollment: vi.fn(() => ({ mutate: vi.fn(), isPending: false })),
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
  toast: { success: vi.fn(), error: vi.fn(), info: vi.fn() },
}))

type ConsoleStatusResult = ReturnType<typeof consoleHooks.useConsoleStatus>

const baseStatus = {
  cerberus: { enabled: true },
  crowdsec: { enabled: true, mode: 'local' as const, api_url: '' },
  waf: { enabled: true, mode: 'enabled' as const },
  rate_limit: { enabled: true },
  acl: { enabled: true },
}

const baseConsole: Record<string, string | boolean | null> = {
  status: 'not_enrolled',
  key_present: false,
  last_error: null,
  last_attempt_at: null,
  enrolled_at: null,
  last_heartbeat_at: null,
  correlation_id: 'corr-1',
  tenant: 'default',
  agent_name: 'charon-agent',
}

/** Console status as the hook returns it, with the given fields overriding the defaults. */
const consoleStatusWith = (overrides: Partial<Record<keyof typeof baseConsole, string | null>>): ConsoleStatusResult =>
  ({
    data: { ...baseConsole, ...overrides },
    isLoading: false,
    isRefetching: false,
  }) as never

const axiosError = (status: number, message: string, data?: Record<string, unknown>) =>
  new AxiosError(message, undefined, undefined, undefined, {
    status,
    statusText: String(status),
    headers: {},
    config: {},
    data: data ?? { error: message },
  } as never)

const preset = (overrides: Partial<presetsApi.CrowdsecPresetSummary>): presetsApi.CrowdsecPresetSummary => ({
  slug: 'crowdsecurity/base-http-scenarios',
  title: 'Bot Mitigation Essentials',
  summary: 'Hub preset',
  source: 'hub',
  type: 'collections',
  requires_hub: true,
  available: true,
  cached: false,
  cache_key: 'cache-hub',
  ...overrides,
})

const renderPage = async () => {
  renderWithQueryClient(<CrowdSecConfig />)
  await screen.findByText('CrowdSec Configuration')
}

/** Fills the enrolment form and submits it, so each test only varies the refusal. */
const submitEnrollment = async () => {
  await userEvent.type(screen.getByTestId('console-enrollment-token'), 'token-value')
  await userEvent.click(screen.getByTestId('console-ack-checkbox'))
  await userEvent.click(screen.getByTestId('console-enroll-btn'))
}

describe('CrowdSecConfig branch coverage', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    enrollMock.mockReset()
    vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(baseStatus)
    vi.mocked(crowdsecApi.statusCrowdsec).mockResolvedValue({ running: true, pid: 123, lapi_ready: true })
    vi.mocked(crowdsecApi.listCrowdsecFiles).mockResolvedValue({ files: ['acquis.yaml'] })
    vi.mocked(crowdsecApi.readCrowdsecFile).mockResolvedValue({ content: '' })
    vi.mocked(crowdsecApi.listCrowdsecDecisions).mockResolvedValue({ decisions: [] })
    vi.mocked(crowdsecApi.listWhitelists).mockResolvedValue([])
    vi.mocked(crowdsecApi.banIP).mockResolvedValue(undefined)
    vi.mocked(presetsApi.listCrowdsecPresets).mockResolvedValue({ presets: [] })
    vi.mocked(presetsApi.pullCrowdsecPreset).mockResolvedValue({
      status: 'pulled',
      slug: 'crowdsecurity/base-http-scenarios',
      preview: 'name: base-http-scenarios\n',
      cache_key: 'cache-pulled',
      etag: '',
      retrieved_at: '',
      source: 'hub',
    })
    vi.mocked(presetsApi.getCrowdsecPresetCache).mockResolvedValue({
      preview: 'cached preview',
      cache_key: 'cache-cached',
      etag: '',
      retrieved_at: '',
      source: 'hub',
    } as never)
    vi.mocked(backupsApi.createBackup).mockResolvedValue({ job_id: 'job-1', type: 'create', status: 'pending' })
    vi.mocked(settingsApi.updateSetting).mockResolvedValue()
    vi.mocked(featureFlagsApi.getFeatureFlags).mockResolvedValue({ 'feature.crowdsec.console_enrollment': true })
    vi.mocked(systemApi.getMyIP).mockResolvedValue({ ip: '203.0.113.1', source: 'cloudflare' })
    vi.mocked(consoleHooks.useConsoleStatus).mockReturnValue(consoleStatusWith({}))
  })

  describe('preset catalog', () => {
    it('falls back to the curated title when the server sends a blank title', async () => {
      const curated = CROWDSEC_PRESETS[0]
      vi.mocked(presetsApi.listCrowdsecPresets).mockResolvedValue({
        presets: [preset({ slug: curated.slug, title: '' })],
      })
      await renderPage()
      expect((await screen.findAllByText(curated.title)).length).toBeGreaterThan(0)
    })

    it('falls back to the slug when neither the server nor the curated list names the preset', async () => {
      vi.mocked(presetsApi.listCrowdsecPresets).mockResolvedValue({
        presets: [preset({ slug: 'crowdsecurity/unlisted', title: '' })],
      })
      await renderPage()
      expect((await screen.findAllByText('crowdsecurity/unlisted')).length).toBeGreaterThan(0)
    })

    it('filters by title, slug and description', async () => {
      vi.mocked(presetsApi.listCrowdsecPresets).mockResolvedValue({
        presets: [
          preset({}),
          preset({ slug: 'crowdsecurity/zoo', title: 'Zoo Guard', summary: 'Blocks zebra scanners', source: 'hub' }),
        ],
      })
      await renderPage()
      const search = screen.getByPlaceholderText('Search presets...')
      const bot = () => screen.queryByRole('button', { name: /Bot Mitigation Essentials/ })
      const zoo = () => screen.queryByRole('button', { name: /Zoo Guard/ })

      await waitFor(() => expect(bot()).toBeInTheDocument())
      expect(zoo()).toBeInTheDocument()

      await userEvent.type(search, 'zebra')
      expect(bot()).not.toBeInTheDocument()
      expect(zoo()).toBeInTheDocument()

      await userEvent.clear(search)
      await userEvent.type(search, 'base-http')
      expect(bot()).toBeInTheDocument()
      expect(zoo()).not.toBeInTheDocument()

      await userEvent.clear(search)
      await userEvent.type(search, 'bot')
      expect(bot()).toBeInTheDocument()
      expect(zoo()).not.toBeInTheDocument()
    })

    it('sorts alphabetically by default and by source when asked, with missing sources last', async () => {
      vi.mocked(presetsApi.listCrowdsecPresets).mockResolvedValue({
        presets: [
          preset({ slug: 'crowdsecurity/bot', title: 'Bot Mitigation Essentials', source: 'hub' }),
          preset({ slug: 'crowdsecurity/aard', title: 'Aardvark Preset', source: '' }),
          preset({ slug: 'crowdsecurity/alpha', title: 'Alpha Preset', source: 'charon-curated' }),
        ],
      })
      await renderPage()
      const titles = ['Aardvark Preset', 'Alpha Preset', 'Bot Mitigation Essentials']
      const orderOf = () =>
        screen
          .getAllByRole('button', { name: /Aardvark Preset|Alpha Preset|Bot Mitigation Essentials/ })
          .map((row) => titles.find((title) => row.textContent?.includes(title)))

      await waitFor(() => expect(orderOf()).toHaveLength(3))
      expect(orderOf()).toEqual(['Aardvark Preset', 'Alpha Preset', 'Bot Mitigation Essentials'])

      await userEvent.selectOptions(screen.getByRole('combobox', { name: 'Sort presets' }), 'source')
      expect(orderOf()).toEqual(['Alpha Preset', 'Bot Mitigation Essentials', 'Aardvark Preset'])
    })

    it('breaks source ties by title when sorting by source', async () => {
      vi.mocked(presetsApi.listCrowdsecPresets).mockResolvedValue({
        presets: [
          preset({ slug: 'crowdsecurity/zeta', title: 'Zeta Preset', source: 'hub' }),
          preset({ slug: 'crowdsecurity/beta', title: 'Beta Preset', source: 'hub' }),
        ],
      })
      await renderPage()
      const titles = ['Beta Preset', 'Zeta Preset']
      const orderOf = () =>
        screen
          .getAllByRole('button', { name: /Beta Preset|Zeta Preset/ })
          .map((row) => titles.find((title) => row.textContent?.includes(title)))

      await waitFor(() => expect(orderOf()).toHaveLength(2))
      await userEvent.selectOptions(screen.getByRole('combobox', { name: 'Sort presets' }), 'source')
      expect(orderOf()).toEqual(['Beta Preset', 'Zeta Preset'])
    })
  })

  describe('preset pull failures', () => {
    it.each([
      ['a non-HTTP error', new Error('network down'), 'Failed to pull preset preview'],
      ['a 400 without a message', axiosError(400, 'bad request', {}), 'Preset slug is invalid'],
      ['a 500 without a message', axiosError(500, 'server exploded', {}), 'Failed to pull preset preview'],
    ])('shows the right text for %s', async (_label, rejection, text) => {
      vi.mocked(presetsApi.pullCrowdsecPreset).mockRejectedValue(rejection)
      vi.mocked(presetsApi.listCrowdsecPresets).mockResolvedValue({ presets: [preset({})] })
      await renderPage()
      expect(await screen.findByText(text)).toBeInTheDocument()
    })
  })

  describe('cached preview failures', () => {
    it.each([
      ['a non-HTTP error', new Error('boom'), 'Failed to load cached preview'],
      ['an HTTP error without a body message', axiosError(500, 'Cache is gone', {}), 'Cache is gone'],
      ['an HTTP error with a body message', axiosError(500, 'Cache missing'), 'Cache missing'],
    ])('reports %s when loading the cached preview', async (_label, rejection, text) => {
      // The cached-preview action is offered only while the hub is unreachable.
      vi.mocked(presetsApi.listCrowdsecPresets).mockResolvedValue({ presets: [preset({ cached: true })] })
      vi.mocked(presetsApi.pullCrowdsecPreset).mockRejectedValue(axiosError(503, 'hub unavailable', {}))
      vi.mocked(presetsApi.getCrowdsecPresetCache).mockRejectedValue(rejection)
      await renderPage()

      await userEvent.click(await screen.findByRole('button', { name: 'Use Cached' }))

      await waitFor(() => expect(toast.error).toHaveBeenCalledWith(text))
    })
  })

  describe('console enrolment', () => {
    it.each([
      ['a plain Error', new Error('Network dropped'), 'Network dropped'],
      ['a value that is not an Error', 'oops', 'Console enrollment failed'],
      ['an HTTP error without a body message', axiosError(400, 'Enrollment rejected by server', {}), 'Enrollment rejected by server'],
    ])('shows the text for %s', async (_label, rejection, text) => {
      enrollMock.mockRejectedValue(rejection)
      await renderPage()

      await submitEnrollment()

      await waitFor(() => expect(toast.error).toHaveBeenCalledWith(text))
    })

    it('asks for the token when a retry is submitted with an empty token', async () => {
      vi.mocked(consoleHooks.useConsoleStatus).mockReturnValue(consoleStatusWith({ status: 'degraded' }))
      await renderPage()

      await userEvent.click(screen.getByTestId('console-retry-btn'))

      expect(await screen.findByText('Enrollment token is required')).toBeInTheDocument()
      expect(enrollMock).not.toHaveBeenCalled()
    })

    it('disables retry and rotate and explains why while LAPI is not ready', async () => {
      vi.mocked(consoleHooks.useConsoleStatus).mockReturnValue(consoleStatusWith({ status: 'degraded' }))
      vi.mocked(crowdsecApi.statusCrowdsec).mockResolvedValue({ running: true, pid: 123, lapi_ready: false })
      await renderPage()

      expect(await screen.findByTestId('lapi-warning')).toBeInTheDocument()
      const retry = screen.getByTestId('console-retry-btn')
      expect(retry).toBeDisabled()
      expect(retry).toHaveAttribute('title', 'LAPI must be ready to retry enrollment')
      expect(screen.getByTestId('console-rotate-btn')).toBeDisabled()
    })

    it('shows the pending acceptance instructions', async () => {
      vi.mocked(consoleHooks.useConsoleStatus).mockReturnValue(consoleStatusWith({ status: 'pending_acceptance' }))
      await renderPage()
      expect(await screen.findByTestId('pending-acceptance-info')).toBeInTheDocument()
    })

    it('shows the last attempt and enrolment times when the console reports them', async () => {
      const attempt = '2026-01-02T03:04:05Z'
      const enrolled = '2026-01-01T01:00:00Z'
      vi.mocked(consoleHooks.useConsoleStatus).mockReturnValue(
        consoleStatusWith({ status: 'enrolled', last_attempt_at: attempt, enrolled_at: enrolled }),
      )
      await renderPage()

      expect(await screen.findByText(/^Last attempt:/)).toHaveTextContent(new Date(attempt).toLocaleString())
      expect(screen.getByText(/^Enrolled:/)).toHaveTextContent(new Date(enrolled).toLocaleString())
    })
  })

  describe('ban and whitelist tables', () => {
    it('shows dashes and a manual source for decisions that carry no details', async () => {
      vi.mocked(crowdsecApi.listCrowdsecDecisions).mockResolvedValue({
        decisions: [
          { id: 'd1', ip: '192.0.2.10', reason: '', duration: '4h', created_at: '', source: '' },
          { id: 'd2', ip: '192.0.2.11', reason: 'scanner', duration: '1h', created_at: '2026-01-02T03:04:05Z', source: 'crowdsec' },
        ],
      })
      await renderPage()

      const bare = (await screen.findByText('192.0.2.10')).closest('tr')
      expect(bare).not.toBeNull()
      expect(within(bare as HTMLElement).getByText('manual')).toBeInTheDocument()
      expect(within(bare as HTMLElement).getAllByText('-')).toHaveLength(2)

      const detailed = screen.getByText('192.0.2.11').closest('tr')
      expect(within(detailed as HTMLElement).getByText('scanner')).toBeInTheDocument()
      expect(within(detailed as HTMLElement).getByText('crowdsec')).toBeInTheDocument()
    })

    it('shows dashes for whitelist entries without a reason or date', async () => {
      vi.mocked(crowdsecApi.listWhitelists).mockResolvedValue([
        { uuid: 'uuid-bare', ip_or_cidr: '198.51.100.7', reason: '', created_at: '', updated_at: '' },
      ])
      renderWithQueryClient(<CrowdSecConfig />)
      await screen.findByText('CrowdSec Configuration')
      await userEvent.click(screen.getByRole('tab', { name: 'Whitelist' }))

      const row = (await screen.findByText('198.51.100.7')).closest('tr')
      expect(within(row as HTMLElement).getAllByText('-')).toHaveLength(2)
    })

    it('closes the whitelist delete dialog on Escape', async () => {
      vi.mocked(crowdsecApi.listWhitelists).mockResolvedValue([
        { uuid: 'uuid-1', ip_or_cidr: '192.168.1.1', reason: 'Home', created_at: '', updated_at: '' },
      ])
      renderWithQueryClient(<CrowdSecConfig />)
      await screen.findByText('CrowdSec Configuration')
      await userEvent.click(screen.getByRole('tab', { name: 'Whitelist' }))
      await userEvent.click(await screen.findByTestId('whitelist-delete-btn'))

      const dialog = await screen.findByRole('dialog')
      within(dialog).getByRole('button', { name: 'Cancel' }).focus()
      await userEvent.keyboard('{Escape}')

      await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
    })

    it('closes the ban dialog on Escape and does not ban while the IP is empty', async () => {
      await renderPage()
      await userEvent.click(await screen.findByTestId('ban-ip-trigger'))

      const ipInput = await screen.findByLabelText('IP Address')
      await userEvent.type(ipInput, '{Enter}')
      expect(crowdsecApi.banIP).not.toHaveBeenCalled()

      await userEvent.keyboard('{Escape}')
      await waitFor(() => expect(screen.queryByLabelText('IP Address')).not.toBeInTheDocument())
    })

    it('bans from the reason field on Enter when an IP is entered', async () => {
      await renderPage()
      await userEvent.click(await screen.findByTestId('ban-ip-trigger'))

      await userEvent.type(await screen.findByLabelText('IP Address'), '192.0.2.99')
      const reason = screen.getByLabelText('Reason')
      await userEvent.click(reason)
      await userEvent.keyboard('{Enter}')
      await waitFor(() => expect(crowdsecApi.banIP).toHaveBeenCalledWith('192.0.2.99', expect.anything(), expect.anything()))
    })
  })
})
