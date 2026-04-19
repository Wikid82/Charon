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
  useEnrollConsole: vi.fn(() => ({
    mutateAsync: vi.fn().mockResolvedValue({ status: 'enrolling', key_present: false }),
    isPending: false,
  })),
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

// The i18n mock in test setup returns the translation key when no translation is found.
// These constants keep assertions in sync with what the component actually renders.
const TAB_WHITELIST = 'crowdsecConfig.whitelist.tabLabel'
const MODAL_TITLE = 'crowdsecConfig.whitelist.deleteModal.title'
const BTN_REMOVE = 'crowdsecConfig.whitelist.deleteModal.submit'

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

const mockWhitelistEntries = [
  {
    uuid: 'uuid-1',
    ip_or_cidr: '192.168.1.1',
    reason: 'Home IP',
    created_at: '2024-01-01T00:00:00Z',
    updated_at: '2024-01-01T00:00:00Z',
  },
  {
    uuid: 'uuid-2',
    ip_or_cidr: '10.0.0.0/8',
    reason: 'LAN',
    created_at: '2024-01-02T00:00:00Z',
    updated_at: '2024-01-02T00:00:00Z',
  },
]

const renderPage = async () => {
  renderWithQueryClient(<CrowdSecConfig />)
  await waitFor(() => screen.getByText('CrowdSec Configuration'))
}

const goToWhitelistTab = async () => {
  await userEvent.click(screen.getByRole('tab', { name: TAB_WHITELIST }))
  await waitFor(() => screen.getByTestId('whitelist-ip-input'))
}

describe('CrowdSecConfig – whitelist tab', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(baseStatus)
    vi.mocked(crowdsecApi.statusCrowdsec).mockResolvedValue({ running: true, pid: 123, lapi_ready: true })
    vi.mocked(crowdsecApi.listCrowdsecFiles).mockResolvedValue({ files: ['acquis.yaml'] })
    vi.mocked(crowdsecApi.readCrowdsecFile).mockResolvedValue({ content: '' })
    vi.mocked(crowdsecApi.writeCrowdsecFile).mockResolvedValue(undefined)
    vi.mocked(crowdsecApi.listCrowdsecDecisions).mockResolvedValue({ decisions: [] })
    vi.mocked(crowdsecApi.banIP).mockResolvedValue(undefined)
    vi.mocked(crowdsecApi.unbanIP).mockResolvedValue(undefined)
    vi.mocked(crowdsecApi.exportCrowdsecConfig).mockResolvedValue(new Blob(['data']))
    vi.mocked(crowdsecApi.importCrowdsecConfig).mockResolvedValue(undefined)
    vi.mocked(crowdsecApi.listWhitelists).mockResolvedValue([])
    vi.mocked(crowdsecApi.addWhitelist).mockResolvedValue({
      uuid: 'uuid-new',
      ip_or_cidr: '1.2.3.4',
      reason: '',
      created_at: '',
      updated_at: '',
    })
    vi.mocked(crowdsecApi.deleteWhitelist).mockResolvedValue(undefined)
    vi.mocked(presetsApi.listCrowdsecPresets).mockResolvedValue({ presets: [] })
    vi.mocked(presetsApi.pullCrowdsecPreset).mockResolvedValue({
      status: 'pulled',
      slug: '',
      preview: '',
      cache_key: '',
      etag: '',
      retrieved_at: '',
      source: 'hub',
    })
    vi.mocked(presetsApi.applyCrowdsecPreset).mockResolvedValue({
      status: 'applied',
      backup: '',
      reload_hint: false,
      used_cscli: false,
      cache_key: '',
      slug: '',
    })
    vi.mocked(presetsApi.getCrowdsecPresetCache).mockResolvedValue({
      preview: '',
      cache_key: '',
      etag: '',
    })
    vi.mocked(backupsApi.createBackup).mockResolvedValue({ filename: 'backup.tar.gz' })
    vi.mocked(settingsApi.updateSetting).mockResolvedValue()
    vi.mocked(featureFlagsApi.getFeatureFlags).mockResolvedValue({
      'feature.crowdsec.console_enrollment': false,
    })
    vi.mocked(systemApi.getMyIP).mockResolvedValue({ ip: '203.0.113.1', source: 'cloudflare' })
  })

  it('shows whitelist tab trigger in local mode', async () => {
    await renderPage()
    expect(screen.getByRole('tab', { name: TAB_WHITELIST })).toBeInTheDocument()
  })

  it('does not show whitelist tab in disabled mode', async () => {
    vi.mocked(securityApi.getSecurityStatus).mockResolvedValue({
      ...baseStatus,
      crowdsec: { enabled: true, mode: 'disabled' as const, api_url: '' },
    })
    await renderPage()
    expect(screen.queryByRole('tab', { name: TAB_WHITELIST })).not.toBeInTheDocument()
  })

  it('shows empty state when there are no whitelist entries', async () => {
    await renderPage()
    await goToWhitelistTab()
    expect(screen.getByTestId('whitelist-empty')).toBeInTheDocument()
  })

  it('renders whitelist entries in the table', async () => {
    vi.mocked(crowdsecApi.listWhitelists).mockResolvedValue(mockWhitelistEntries)
    await renderPage()
    await goToWhitelistTab()
    expect(await screen.findByText('192.168.1.1')).toBeInTheDocument()
    expect(screen.getByText('10.0.0.0/8')).toBeInTheDocument()
    expect(screen.getByText('Home IP')).toBeInTheDocument()
    expect(screen.getByText('LAN')).toBeInTheDocument()
  })

  it('submits a new whitelist entry', async () => {
    await renderPage()
    await goToWhitelistTab()
    await userEvent.type(screen.getByTestId('whitelist-ip-input'), '1.2.3.4')
    await userEvent.type(screen.getByTestId('whitelist-reason-input'), 'Test reason')
    await userEvent.click(screen.getByTestId('whitelist-add-btn'))
    await waitFor(() =>
      expect(crowdsecApi.addWhitelist).toHaveBeenCalledWith({
        ip_or_cidr: '1.2.3.4',
        reason: 'Test reason',
      }),
    )
  })

  it('shows add-whitelist loading overlay while mutation is pending', async () => {
    let resolveAdd!: (v: (typeof mockWhitelistEntries)[0]) => void
    vi.mocked(crowdsecApi.addWhitelist).mockImplementationOnce(
      () =>
        new Promise((resolve) => {
          resolveAdd = resolve
        }),
    )
    await renderPage()
    await goToWhitelistTab()
    await userEvent.type(screen.getByTestId('whitelist-ip-input'), '1.2.3.4')
    await userEvent.click(screen.getByTestId('whitelist-add-btn'))
    await waitFor(() => expect(screen.getByText('Adding IP to whitelist')).toBeInTheDocument())
    resolveAdd(mockWhitelistEntries[0])
  })

  it('displays inline error when adding a whitelist entry fails', async () => {
    vi.mocked(crowdsecApi.addWhitelist).mockRejectedValueOnce(
      axiosError(400, 'Invalid IP', { error: 'bad ip format' }),
    )
    await renderPage()
    await goToWhitelistTab()
    await userEvent.type(screen.getByTestId('whitelist-ip-input'), 'bad-ip')
    await userEvent.click(screen.getByTestId('whitelist-add-btn'))
    await waitFor(() => expect(screen.getByTestId('whitelist-ip-error')).toBeInTheDocument())
  })

  it('opens delete confirmation dialog', async () => {
    vi.mocked(crowdsecApi.listWhitelists).mockResolvedValue(mockWhitelistEntries)
    await renderPage()
    await goToWhitelistTab()
    await userEvent.click((await screen.findAllByTestId('whitelist-delete-btn'))[0])
    expect(await screen.findByRole('dialog', { name: MODAL_TITLE })).toBeInTheDocument()
  })

  it('cancels whitelist deletion via Cancel button', async () => {
    vi.mocked(crowdsecApi.listWhitelists).mockResolvedValue(mockWhitelistEntries)
    await renderPage()
    await goToWhitelistTab()
    await userEvent.click((await screen.findAllByTestId('whitelist-delete-btn'))[0])
    await userEvent.click(await screen.findByRole('button', { name: 'Cancel' }))
    await waitFor(() =>
      expect(screen.queryByRole('dialog', { name: MODAL_TITLE })).not.toBeInTheDocument(),
    )
    expect(crowdsecApi.deleteWhitelist).not.toHaveBeenCalled()
  })

  it('confirms whitelist entry deletion via Remove button', async () => {
    vi.mocked(crowdsecApi.listWhitelists).mockResolvedValue(mockWhitelistEntries)
    await renderPage()
    await goToWhitelistTab()
    await userEvent.click((await screen.findAllByTestId('whitelist-delete-btn'))[0])
    await userEvent.click(await screen.findByRole('button', { name: BTN_REMOVE }))
    await waitFor(() => expect(crowdsecApi.deleteWhitelist).toHaveBeenCalledWith('uuid-1'))
  })

  it('shows delete-whitelist loading overlay while mutation is pending', async () => {
    vi.mocked(crowdsecApi.listWhitelists).mockResolvedValue(mockWhitelistEntries)
    let resolveDelete!: () => void
    vi.mocked(crowdsecApi.deleteWhitelist).mockImplementationOnce(
      () =>
        new Promise<void>((resolve) => {
          resolveDelete = resolve
        }),
    )
    await renderPage()
    await goToWhitelistTab()
    await userEvent.click((await screen.findAllByTestId('whitelist-delete-btn'))[0])
    await userEvent.click(await screen.findByRole('button', { name: BTN_REMOVE }))
    await waitFor(() => expect(screen.getByText('Removing from whitelist')).toBeInTheDocument())
    resolveDelete()
  })

  it('closes delete dialog on Escape key', async () => {
    vi.mocked(crowdsecApi.listWhitelists).mockResolvedValue(mockWhitelistEntries)
    await renderPage()
    await goToWhitelistTab()
    await userEvent.click((await screen.findAllByTestId('whitelist-delete-btn'))[0])
    expect(await screen.findByRole('dialog', { name: MODAL_TITLE })).toBeInTheDocument()
    await userEvent.keyboard('{Escape}')
    await waitFor(() =>
      expect(screen.queryByRole('dialog', { name: MODAL_TITLE })).not.toBeInTheDocument(),
    )
  })

  it('closes delete dialog when backdrop is clicked', async () => {
    vi.mocked(crowdsecApi.listWhitelists).mockResolvedValue(mockWhitelistEntries)
    await renderPage()
    await goToWhitelistTab()
    await userEvent.click((await screen.findAllByTestId('whitelist-delete-btn'))[0])
    expect(await screen.findByRole('dialog', { name: MODAL_TITLE })).toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: /close/i }))
    await waitFor(() =>
      expect(screen.queryByRole('dialog', { name: MODAL_TITLE })).not.toBeInTheDocument(),
    )
  })

  it('fills IP input when Add My IP is clicked', async () => {
    await renderPage()
    await goToWhitelistTab()
    await userEvent.click(screen.getByTestId('whitelist-add-my-ip-btn'))
    await waitFor(() => {
      const input = screen.getByTestId('whitelist-ip-input') as HTMLInputElement
      expect(input.value).toBe('203.0.113.1')
    })
  })

  it('shows error toast when Add My IP request fails', async () => {
    vi.mocked(systemApi.getMyIP).mockRejectedValueOnce(new Error('network error'))
    await renderPage()
    await goToWhitelistTab()
    await userEvent.click(screen.getByTestId('whitelist-add-my-ip-btn'))
    await waitFor(() =>
      expect(toast.error).toHaveBeenCalledWith('Failed to detect your IP address'),
    )
  })
})
