import { type QueryClient } from '@tanstack/react-query'
import { screen, waitFor, act, cleanup, within, fireEvent } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { AxiosError } from 'axios'
import { describe, it, expect, vi, beforeEach } from 'vitest'

import * as backupsApi from '../../api/backups'
import * as crowdsecApi from '../../api/crowdsec'
import * as featureFlagsApi from '../../api/featureFlags'
import * as presetsApi from '../../api/presets'
import * as securityApi from '../../api/security'
import * as settingsApi from '../../api/settings'
import { CROWDSEC_PRESETS } from '../../data/crowdsecPresets'
import { renderWithQueryClient, createTestQueryClient } from '../../test-utils/renderWithQueryClient'
import * as exportUtils from '../../utils/crowdsecExport'
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

const disabledStatus = {
  ...baseStatus,
  crowdsec: { ...baseStatus.crowdsec, enabled: true, mode: 'disabled' as const },
}

const presetFromCatalog = CROWDSEC_PRESETS[0]

const axiosError = (status: number, message: string, data?: Record<string, unknown>) =>
  new AxiosError(message, undefined, undefined, undefined, {
    status,
    statusText: String(status),
    headers: {},
    config: {},
    data: data ?? { error: message },
  } as never)

const defaultFileList = ['acquis.yaml', 'collections.yaml']

const renderPage = async (client?: QueryClient) => {
  const result = renderWithQueryClient(<CrowdSecConfig />, { client })
  await waitFor(() => screen.getByText('CrowdSec Configuration'))
  return result
}

describe('CrowdSecConfig coverage', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(baseStatus)
    vi.mocked(crowdsecApi.statusCrowdsec).mockResolvedValue({ running: true, pid: 123, lapi_ready: true })
    vi.mocked(crowdsecApi.listCrowdsecFiles).mockResolvedValue({ files: defaultFileList })
    vi.mocked(crowdsecApi.readCrowdsecFile).mockResolvedValue({ content: 'file-content' })
    vi.mocked(crowdsecApi.writeCrowdsecFile).mockResolvedValue(undefined)
    vi.mocked(crowdsecApi.listCrowdsecDecisions).mockResolvedValue({ decisions: [] })
    vi.mocked(crowdsecApi.banIP).mockResolvedValue(undefined)
    vi.mocked(crowdsecApi.unbanIP).mockResolvedValue(undefined)
    vi.mocked(crowdsecApi.exportCrowdsecConfig).mockResolvedValue(new Blob(['data']))
    vi.mocked(crowdsecApi.importCrowdsecConfig).mockResolvedValue(undefined)
    vi.mocked(presetsApi.listCrowdsecPresets).mockResolvedValue({
      presets: [
        {
          slug: presetFromCatalog.slug,
          title: presetFromCatalog.title,
          summary: presetFromCatalog.description,
          source: 'hub',
          requires_hub: false,
          available: true,
          cached: false,
          cache_key: 'cache-123',
          etag: 'etag-123',
          retrieved_at: '2024-01-01T00:00:00Z',
        },
      ],
    })
    vi.mocked(presetsApi.pullCrowdsecPreset).mockResolvedValue({
      status: 'pulled',
      slug: presetFromCatalog.slug,
      preview: presetFromCatalog.content,
      cache_key: 'cache-123',
      etag: 'etag-123',
      retrieved_at: '2024-01-01T00:00:00Z',
      source: 'hub',
    })
    vi.mocked(presetsApi.applyCrowdsecPreset).mockResolvedValue({
      status: 'applied',
      backup: '/tmp/backup.tar.gz',
      reload_hint: true,
      used_cscli: true,
      cache_key: 'cache-123',
      slug: presetFromCatalog.slug,
    })
    vi.mocked(presetsApi.getCrowdsecPresetCache).mockResolvedValue({
      preview: 'cached-preview',
      cache_key: 'cache-123',
      etag: 'etag-123',
    })
    vi.mocked(backupsApi.createBackup).mockResolvedValue({ filename: 'backup.tar.gz' })
    vi.mocked(settingsApi.updateSetting).mockResolvedValue()
    vi.mocked(featureFlagsApi.getFeatureFlags).mockResolvedValue({
      'feature.crowdsec.console_enrollment': false,
    })
  })

  it('renders loading and error boundaries', async () => {
    vi.mocked(securityApi.getSecurityStatus).mockReturnValue(new Promise(() => {}))
    renderWithQueryClient(<CrowdSecConfig />)
    expect(await screen.findByText('Loading CrowdSec configuration...')).toBeInTheDocument()

    cleanup()

    vi.mocked(securityApi.getSecurityStatus).mockRejectedValue(new Error('boom'))
    renderWithQueryClient(<CrowdSecConfig />)
    expect(await screen.findByText(/Error loading CrowdSec status/)).toBeInTheDocument()
  })

  it('handles missing status and missing crowdsec sections', async () => {
    vi.mocked(securityApi.getSecurityStatus).mockRejectedValueOnce(new Error('data is undefined'))
    renderWithQueryClient(<CrowdSecConfig />)
    expect(await screen.findByText(/Error loading CrowdSec status/)).toBeInTheDocument()

    cleanup()

    vi.mocked(securityApi.getSecurityStatus).mockResolvedValueOnce({ cerberus: { enabled: true } } as never)
    renderWithQueryClient(<CrowdSecConfig />)
    expect(await screen.findByText('CrowdSec configuration not found in security status')).toBeInTheDocument()
  })

  it('renders disabled mode message and bans control disabled', async () => {
    vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(disabledStatus)
    await renderPage(createTestQueryClient())
    expect(screen.getByText('Enable CrowdSec to manage banned IPs')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /Ban IP/ })).toBeDisabled()
  })

  it('shows info banner directing to Security Dashboard', async () => {
    await renderPage()
    expect(screen.getByText(/CrowdSec is controlled via the toggle on the/i)).toBeInTheDocument()
    expect(screen.getByRole('link', { name: /Security/i })).toHaveAttribute('href', '/security')
  })

  it('guards import without a file and shows error on import failure', async () => {
    await renderPage()
    const importBtn = screen.getByTestId('import-btn')
    await userEvent.click(importBtn)
    expect(backupsApi.createBackup).not.toHaveBeenCalled()

    const fileInput = screen.getByTestId('import-file') as HTMLInputElement
    const file = new File(['data'], 'cfg.tar.gz')
    await userEvent.upload(fileInput, file)
    vi.mocked(crowdsecApi.importCrowdsecConfig).mockRejectedValueOnce(new Error('bad import'))
    await userEvent.click(importBtn)
    await waitFor(() => expect(toast.error).toHaveBeenCalledWith('bad import'))
  })

  it('imports configuration after creating a backup', async () => {
    await renderPage()
    const fileInput = screen.getByTestId('import-file') as HTMLInputElement
    await userEvent.upload(fileInput, new File(['data'], 'cfg.tar.gz'))
    await userEvent.click(screen.getByTestId('import-btn'))
    await waitFor(() => expect(backupsApi.createBackup).toHaveBeenCalled())
    await waitFor(() => expect(crowdsecApi.importCrowdsecConfig).toHaveBeenCalled())
  })

  it('exports configuration success and failure', async () => {
    await renderPage()
    await userEvent.click(screen.getByRole('button', { name: 'Export' }))
    await waitFor(() => expect(crowdsecApi.exportCrowdsecConfig).toHaveBeenCalled())
    expect(exportUtils.downloadCrowdsecExport).toHaveBeenCalled()
    expect(toast.success).toHaveBeenCalledWith('CrowdSec configuration exported')

    vi.mocked(exportUtils.promptCrowdsecFilename).mockReturnValueOnce('crowdsec.tar.gz')
    vi.mocked(crowdsecApi.exportCrowdsecConfig).mockRejectedValueOnce(new Error('fail'))
    await userEvent.click(screen.getByRole('button', { name: 'Export' }))
    await waitFor(() => expect(toast.error).toHaveBeenCalledWith('Failed to export CrowdSec configuration'))
  })

  it('auto-selects first preset and pulls preview', async () => {
    await renderPage()
    // Component auto-selects first preset from the list on render
    await waitFor(() => expect(presetsApi.pullCrowdsecPreset).toHaveBeenCalledWith(presetFromCatalog.slug))
    const previewText = screen.getByTestId('preset-preview').textContent?.replace(/\s+/g, ' ')
    expect(previewText).toContain('crowdsecurity/http-cve')
    expect(screen.getByTestId('preset-meta')).toHaveTextContent('cache-123')
  })

  it('handles pull validation, hub unavailable, and generic errors', async () => {
    vi.mocked(presetsApi.pullCrowdsecPreset).mockRejectedValueOnce(axiosError(400, 'slug invalid', { error: 'slug invalid' }))
    await renderPage()
    expect(await screen.findByTestId('preset-validation-error')).toHaveTextContent('slug invalid')

    vi.mocked(presetsApi.pullCrowdsecPreset).mockRejectedValueOnce(axiosError(503, 'hub down', { error: 'hub down' }))
    await userEvent.click(screen.getByText('Pull Preview'))
    expect(await screen.findByTestId('preset-hub-unavailable')).toBeInTheDocument()

    vi.mocked(presetsApi.pullCrowdsecPreset).mockRejectedValueOnce(axiosError(500, 'boom', { error: 'boom' }))
    await userEvent.click(screen.getByText('Pull Preview'))
    await waitFor(() => expect(screen.getByTestId('preset-status')).toHaveTextContent('boom'))
  })

  it('loads cached preview and reports cache errors', async () => {
    vi.mocked(presetsApi.listCrowdsecPresets).mockResolvedValueOnce({
      presets: [
        {
          slug: presetFromCatalog.slug,
          title: presetFromCatalog.title,
          summary: presetFromCatalog.description,
          source: 'hub',
          requires_hub: false,
          available: true,
          cached: true,
          cache_key: 'cache-123',
          etag: 'etag-123',
          retrieved_at: '2024-01-01T00:00:00Z',
        },
      ],
    })
    await renderPage()
    await userEvent.click(screen.getByText('Pull Preview'))
    await waitFor(() => {
      const preview = screen.getByTestId('preset-preview').textContent?.replace(/\s+/g, ' ')
      expect(preview).toContain('crowdsecurity/http-cve')
    })
    await userEvent.click(screen.getByText('Load cached preview'))
    await waitFor(() => expect(screen.getByTestId('preset-preview')).toHaveTextContent('cached-preview'))

    vi.mocked(presetsApi.getCrowdsecPresetCache).mockRejectedValueOnce(axiosError(500, 'cache-miss'))
    await userEvent.click(screen.getByText('Load cached preview'))
    await waitFor(() => expect(toast.error).toHaveBeenCalledWith('cache-miss'))
  })

  it('sets apply info on backend success', async () => {
    await renderPage()
    await userEvent.click(screen.getByTestId('apply-preset-btn'))
    await waitFor(() => expect(screen.getByTestId('preset-apply-info')).toHaveTextContent('Backup: /tmp/backup.tar.gz'))
  })

  it('supports keyboard selection for preset cards (Enter and Space)', async () => {
    vi.mocked(presetsApi.listCrowdsecPresets).mockResolvedValueOnce({
      presets: [
        {
          slug: CROWDSEC_PRESETS[0].slug,
          title: CROWDSEC_PRESETS[0].title,
          summary: CROWDSEC_PRESETS[0].description,
          source: 'hub',
          requires_hub: false,
          available: true,
          cached: false,
          cache_key: 'cache-a',
        },
        {
          slug: CROWDSEC_PRESETS[1].slug,
          title: CROWDSEC_PRESETS[1].title,
          summary: CROWDSEC_PRESETS[1].description,
          source: 'hub',
          requires_hub: false,
          available: true,
          cached: false,
          cache_key: 'cache-b',
        },
      ],
    })

    await renderPage()

    const firstCard = await screen.findByRole('button', { name: new RegExp(CROWDSEC_PRESETS[0].title, 'i') })
    const secondCard = await screen.findByRole('button', { name: new RegExp(CROWDSEC_PRESETS[1].title, 'i') })

    firstCard.focus()
    await userEvent.keyboard('{Enter}')
    secondCard.focus()
    await userEvent.keyboard(' ')

    await waitFor(() => expect(presetsApi.pullCrowdsecPreset).toHaveBeenCalledTimes(2))
  })

  it('falls back to local apply on 501 and covers validation/hub/offline branches', async () => {
    vi.mocked(crowdsecApi.writeCrowdsecFile).mockResolvedValue({})
    vi.mocked(presetsApi.applyCrowdsecPreset).mockRejectedValueOnce(axiosError(501, 'not implemented'))
    await renderPage()
    const applyBtn = screen.getByTestId('apply-preset-btn')
    await userEvent.click(applyBtn)
    await waitFor(() => expect(toast.info).toHaveBeenCalledWith('Preset apply is not available on the server; applying locally instead'))
    await waitFor(() => expect(crowdsecApi.writeCrowdsecFile).toHaveBeenCalled())

    vi.mocked(presetsApi.applyCrowdsecPreset).mockRejectedValueOnce(axiosError(400, 'bad', { error: 'validation failed' }))
    await userEvent.click(applyBtn)
    await waitFor(() => expect(screen.getByTestId('preset-validation-error')).toHaveTextContent('validation failed'))

    vi.mocked(presetsApi.applyCrowdsecPreset).mockRejectedValueOnce(axiosError(503, 'hub'))
    await userEvent.click(applyBtn)
    expect(await screen.findByTestId('preset-hub-unavailable')).toBeInTheDocument()

    vi.mocked(presetsApi.applyCrowdsecPreset).mockRejectedValueOnce(axiosError(500, 'not cached', { error: 'Pull the preset first' }))
    await userEvent.click(applyBtn)
    await waitFor(() => expect(screen.getByTestId('preset-validation-error')).toHaveTextContent('Preset must be pulled'))
  })

  it('records backup info on apply failure and generic errors', async () => {
    vi.mocked(presetsApi.applyCrowdsecPreset).mockRejectedValueOnce(axiosError(500, 'failed', { error: 'boom', backup: '/tmp/backup' }))
    await renderPage()
    await userEvent.click(screen.getByTestId('apply-preset-btn'))
    await waitFor(() => expect(screen.getByTestId('preset-apply-info')).toHaveTextContent('/tmp/backup'))

    cleanup()

    vi.mocked(presetsApi.applyCrowdsecPreset).mockRejectedValueOnce(new Error('unexpected'))
    await renderPage()
    await userEvent.click(screen.getByTestId('apply-preset-btn'))
    await waitFor(() => expect(toast.error).toHaveBeenCalledWith('Failed to apply preset'))
  })

  it('disables apply when hub is unavailable for hub-only preset', async () => {
    vi.mocked(presetsApi.listCrowdsecPresets).mockResolvedValueOnce({
      presets: [
        {
          slug: 'hub-only',
          title: 'Hub Only',
          summary: 'needs hub',
          source: 'hub',
          requires_hub: true,
          available: true,
          cached: true,
          cache_key: 'cache-hub',
          etag: 'etag-hub',
        },
      ],
    })
    vi.mocked(presetsApi.pullCrowdsecPreset).mockRejectedValueOnce(axiosError(503, 'hub'))
    await renderPage()
    expect(await screen.findByTestId('preset-hub-unavailable')).toBeInTheDocument()
    expect((screen.getByTestId('apply-preset-btn') as HTMLButtonElement).disabled).toBe(true)
  })

  it('guards local apply prerequisites and succeeds when content exists', async () => {
    vi.mocked(crowdsecApi.listCrowdsecFiles).mockResolvedValueOnce({ files: [] })
    vi.mocked(presetsApi.applyCrowdsecPreset).mockRejectedValueOnce(axiosError(501, 'not implemented'))
    await renderPage()
    await userEvent.click(screen.getByTestId('apply-preset-btn'))
    await waitFor(() => expect(toast.error).toHaveBeenCalledWith('Select a configuration file to apply the preset'))

    cleanup()
  vi.mocked(toast.error).mockClear()

    vi.mocked(crowdsecApi.listCrowdsecFiles).mockResolvedValue({ files: ['acquis.yaml'] })
    vi.mocked(presetsApi.listCrowdsecPresets).mockResolvedValue({
      presets: [
        {
          slug: 'custom-empty',
          title: 'Empty',
          summary: 'empty preset',
          source: 'hub',
          requires_hub: false,
          available: true,
          cached: false,
          cache_key: 'cache-empty',
          etag: 'etag-empty',
        },
      ],
    })
    vi.mocked(presetsApi.pullCrowdsecPreset).mockResolvedValue({
      status: 'pulled',
      slug: 'custom-empty',
      preview: '',
      cache_key: 'cache-empty',
    })
    vi.mocked(presetsApi.applyCrowdsecPreset).mockRejectedValueOnce(axiosError(501, 'not implemented'))
    await renderPage()
    await userEvent.selectOptions(screen.getByTestId('crowdsec-file-select'), 'acquis.yaml')
    await userEvent.click(screen.getByTestId('apply-preset-btn'))
    await waitFor(() => expect(toast.error).toHaveBeenCalledWith('Preset preview is unavailable; retry pulling before applying'))

    cleanup()
  vi.mocked(toast.error).mockClear()

    vi.mocked(presetsApi.pullCrowdsecPreset).mockResolvedValue({
      status: 'pulled',
      slug: presetFromCatalog.slug,
      preview: 'content',
      cache_key: 'cache-123',
    })
    vi.mocked(presetsApi.applyCrowdsecPreset).mockRejectedValueOnce(axiosError(501, 'not implemented'))
    vi.mocked(crowdsecApi.writeCrowdsecFile).mockResolvedValue({})
    await renderPage()
    await userEvent.selectOptions(screen.getByTestId('crowdsec-file-select'), 'acquis.yaml')
    await userEvent.click(screen.getByTestId('apply-preset-btn'))
    await waitFor(() => expect(crowdsecApi.writeCrowdsecFile).toHaveBeenCalled())
  })

  it('reads, edits, saves, and closes files', async () => {
    await renderPage()
    await userEvent.selectOptions(screen.getByTestId('crowdsec-file-select'), 'acquis.yaml')
    await waitFor(() => expect(crowdsecApi.readCrowdsecFile).toHaveBeenCalledWith('acquis.yaml'))
    // Use getAllByRole and filter for textarea (not the search input)
    const textareas = screen.getAllByRole('textbox')
    const textarea = textareas.find(el => el.tagName.toLowerCase() === 'textarea') as HTMLTextAreaElement
    expect(textarea.value).toBe('file-content')
    await userEvent.clear(textarea)
    await userEvent.type(textarea, 'updated')
    await userEvent.click(screen.getByText('Save'))
    await waitFor(() => expect(backupsApi.createBackup).toHaveBeenCalled())
    await waitFor(() => expect(crowdsecApi.writeCrowdsecFile).toHaveBeenCalledWith('acquis.yaml', 'updated'))

    await userEvent.click(screen.getByText('Close'))
    expect((screen.getByTestId('crowdsec-file-select') as HTMLSelectElement).value).toBe('')
  })

  it('shows decisions table, handles loading/error/empty states, and unban errors', async () => {
    vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(disabledStatus)
    await renderPage()
    expect(screen.getByText('Enable CrowdSec to manage banned IPs')).toBeInTheDocument()

    cleanup()

    vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(baseStatus)
    vi.mocked(crowdsecApi.listCrowdsecDecisions).mockReturnValue(new Promise(() => {}))
    await renderPage()
    expect(screen.getByText('Loading banned IPs...')).toBeInTheDocument()

    cleanup()

    vi.mocked(crowdsecApi.listCrowdsecDecisions).mockRejectedValueOnce(new Error('decisions'))
    await renderPage()
    expect(await screen.findByText('Failed to load banned IPs')).toBeInTheDocument()

    cleanup()

    vi.mocked(crowdsecApi.listCrowdsecDecisions).mockResolvedValueOnce({ decisions: [] })
    await renderPage()
    expect(await screen.findByText('No banned IPs')).toBeInTheDocument()

    cleanup()

    vi.mocked(crowdsecApi.listCrowdsecDecisions).mockResolvedValueOnce({
      decisions: [
        { id: '1', ip: '1.1.1.1', reason: 'bot', duration: '24h', created_at: '2024-01-01T00:00:00Z', source: 'manual' },
      ],
    })
    await renderPage()
    expect(await screen.findByText('1.1.1.1')).toBeInTheDocument()

    vi.mocked(crowdsecApi.unbanIP).mockRejectedValueOnce(new Error('unban fail'))
    await userEvent.click(screen.getAllByText('Unban')[0])
    const confirmModal = screen.getByText('Confirm Unban').closest('div') as HTMLElement
    await userEvent.click(within(confirmModal).getByRole('button', { name: 'Unban' }))
    await waitFor(() => expect(toast.error).toHaveBeenCalledWith('unban fail'))
  })

  it('supports ban modal click and keyboard interactions', async () => {
    await renderPage()

    await userEvent.click(screen.getByRole('button', { name: /Ban IP/ }))
    expect(await screen.findByText('Ban IP Address')).toBeInTheDocument()

    const banDialog = screen.getByRole('dialog', { name: 'Ban IP Address' })
    const banOverlay = banDialog.parentElement?.querySelector('[class*="bg-black/60"]') as HTMLElement
    fireEvent.click(banOverlay)
    await waitFor(() => expect(screen.queryByText('Ban IP Address')).not.toBeInTheDocument())

    await userEvent.click(screen.getByRole('button', { name: /Ban IP/ }))
    const modalContainer = screen.getByRole('dialog', { name: 'Ban IP Address' })
    const ipInput = within(modalContainer).getByPlaceholderText('192.168.1.100')
    await userEvent.type(ipInput, '9.9.9.9')

    await userEvent.keyboard('{Control>}{Enter}{/Control}')
    await waitFor(() => expect(crowdsecApi.banIP).toHaveBeenCalledWith('9.9.9.9', '24h', ''))

    await userEvent.click(screen.getByRole('button', { name: /Ban IP/ }))
    const secondModalContainer = screen.getByRole('dialog', { name: 'Ban IP Address' })
    const secondIpInput = within(secondModalContainer).getByPlaceholderText('192.168.1.100')
    await userEvent.type(secondIpInput, '8.8.8.8')
    await userEvent.keyboard('{Enter}')
    await waitFor(() => expect(crowdsecApi.banIP).toHaveBeenCalledWith('8.8.8.8', '24h', ''))

    await userEvent.click(screen.getByRole('button', { name: /Ban IP/ }))
    const thirdModalContainer = screen.getByRole('dialog', { name: 'Ban IP Address' })
    const thirdIpInput = within(thirdModalContainer).getByPlaceholderText('192.168.1.100')
    await userEvent.type(thirdIpInput, '8.8.8.8')
    const reasonInput = within(thirdModalContainer).getByLabelText('Reason')
    await userEvent.type(reasonInput, 'manual reason{Enter}')
    await waitFor(() => expect(crowdsecApi.banIP).toHaveBeenCalledWith('8.8.8.8', '24h', 'manual reason'))
  })

  it('supports unban modal overlay, Escape, Enter, and cancel button', async () => {
    vi.mocked(crowdsecApi.listCrowdsecDecisions).mockResolvedValueOnce({
      decisions: [
        { id: '1', ip: '7.7.7.7', reason: 'bot', duration: '24h', created_at: '2024-01-01T00:00:00Z', source: 'manual' },
      ],
    })

    await renderPage()
    await userEvent.click(await screen.findByRole('button', { name: 'Unban' }))
    expect(await screen.findByText('Confirm Unban')).toBeInTheDocument()

    const unbanDialog = screen.getByRole('dialog', { name: 'Confirm Unban' })
    const unbanOverlay = unbanDialog.parentElement?.querySelector('[class*="bg-black/60"]') as HTMLElement
    fireEvent.click(unbanOverlay)
    await waitFor(() => expect(screen.queryByText('Confirm Unban')).not.toBeInTheDocument())

    await userEvent.click(await screen.findByRole('button', { name: 'Unban' }))
    expect(await screen.findByText('Confirm Unban')).toBeInTheDocument()
    await userEvent.keyboard('{Escape}')
    await waitFor(() => expect(screen.queryByText('Confirm Unban')).not.toBeInTheDocument())

    await userEvent.click(await screen.findByRole('button', { name: 'Unban' }))
    expect(await screen.findByText('Confirm Unban')).toBeInTheDocument()
    await userEvent.keyboard('{Enter}')
    await waitFor(() => expect(crowdsecApi.unbanIP).toHaveBeenCalledWith('7.7.7.7'))

    vi.mocked(crowdsecApi.listCrowdsecDecisions).mockResolvedValueOnce({
      decisions: [
        { id: '1', ip: '7.7.7.7', reason: 'bot', duration: '24h', created_at: '2024-01-01T00:00:00Z', source: 'manual' },
      ],
    })
    await renderPage()
    await userEvent.click(await screen.findByRole('button', { name: 'Unban' }))
    const confirmContainer = screen.getByRole('dialog', { name: 'Confirm Unban' })
    await userEvent.click(within(confirmContainer).getByRole('button', { name: 'Cancel' }))
    await waitFor(() => expect(screen.queryByText('Confirm Unban')).not.toBeInTheDocument())
  })

  it('bans and unbans IPs with overlay messaging', async () => {
    vi.mocked(crowdsecApi.listCrowdsecDecisions).mockResolvedValue({
      decisions: [
        { id: '1', ip: '1.1.1.1', reason: 'bot', duration: '24h', created_at: '2024-01-01T00:00:00Z', source: 'manual' },
      ],
    })
    await renderPage()
    await userEvent.click(screen.getByRole('button', { name: /Ban IP/ }))
    const banModal = screen.getByText('Ban IP Address').closest('div') as HTMLElement
    const ipInput = within(banModal).getByPlaceholderText('192.168.1.100') as HTMLInputElement
    await userEvent.type(ipInput, '2.2.2.2')
    await userEvent.click(within(banModal).getByRole('button', { name: 'Ban IP' }))
    await waitFor(() => expect(crowdsecApi.banIP).toHaveBeenCalledWith('2.2.2.2', '24h', ''))

    // keep ban pending to assert overlay message
    let resolveBan: (() => void) | undefined
    vi.mocked(crowdsecApi.banIP).mockImplementationOnce(
      () =>
        new Promise<void>((resolve) => {
          resolveBan = () => resolve()
        }),
    )
    await userEvent.click(screen.getByRole('button', { name: /Ban IP/ }))
    const banModalSecond = screen.getByText('Ban IP Address').closest('div') as HTMLElement
    await userEvent.type(within(banModalSecond).getByPlaceholderText('192.168.1.100'), '3.3.3.3')
    await userEvent.click(within(banModalSecond).getByRole('button', { name: 'Ban IP' }))
    expect(await screen.findByText('Guardian raises shield...')).toBeInTheDocument()
    resolveBan?.()

    vi.mocked(crowdsecApi.unbanIP).mockImplementationOnce(() => new Promise(() => {}))
    const unbanButtons = await screen.findAllByText('Unban')
    await userEvent.click(unbanButtons[0])
    const confirmDialog = screen.getByText('Confirm Unban').closest('div') as HTMLElement
    await userEvent.click(within(confirmDialog).getByRole('button', { name: 'Unban' }))
    expect(await screen.findByText('Guardian lowers shield...')).toBeInTheDocument()
  })

  it('shows overlay messaging for preset pull, apply, import, write, and mode updates', async () => {
    // pull pending
    vi.mocked(presetsApi.pullCrowdsecPreset).mockImplementation(() => new Promise(() => {}))
    await renderPage()
    await userEvent.click(screen.getByText('Pull Preview'))
    expect(await screen.findByText('Fetching preset...')).toBeInTheDocument()

    cleanup()
    vi.mocked(presetsApi.pullCrowdsecPreset).mockReset()
    vi.mocked(presetsApi.pullCrowdsecPreset).mockResolvedValue({
      status: 'pulled',
      slug: presetFromCatalog.slug,
      preview: presetFromCatalog.content,
      cache_key: 'cache-123',
    })

    // apply pending
    vi.mocked(presetsApi.pullCrowdsecPreset).mockResolvedValueOnce({
      status: 'pulled',
      slug: presetFromCatalog.slug,
      preview: presetFromCatalog.content,
      cache_key: 'cache-123',
    })
    let resolveApply: (() => void) | undefined
    vi.mocked(presetsApi.applyCrowdsecPreset).mockImplementationOnce(
      () =>
        new Promise((resolve) => {
          resolveApply = () => resolve({ status: 'applied', cache_key: 'cache-123' } as never)
        }),
    )
    await renderPage()
    await userEvent.click(screen.getAllByTestId('apply-preset-btn')[0])
    expect(await screen.findByText('Loading preset...')).toBeInTheDocument()
    resolveApply?.()

    cleanup()

    // import pending
    vi.mocked(presetsApi.pullCrowdsecPreset).mockResolvedValueOnce({
      status: 'pulled',
      slug: presetFromCatalog.slug,
      preview: presetFromCatalog.content,
      cache_key: 'cache-123',
    })
    let resolveImport: (() => void) | undefined
    vi.mocked(crowdsecApi.importCrowdsecConfig).mockImplementationOnce(
      () =>
        new Promise((resolve) => {
          resolveImport = () => resolve({})
        }),
    )
    const { queryClient } = await renderPage(createTestQueryClient())
    expect(await screen.findByTestId('preset-preview')).toBeInTheDocument()
    const fileInput = screen.getByTestId('import-file') as HTMLInputElement
    await userEvent.upload(fileInput, new File(['data'], 'cfg.tar.gz'))
    await userEvent.click(screen.getByTestId('import-btn'))
    expect(await screen.findByText('Summoning the guardian...')).toBeInTheDocument()
    resolveImport?.()
    await act(async () => queryClient.cancelQueries())

    cleanup()

    // write pending shows loading overlay
    let resolveWrite: (() => void) | undefined
    vi.mocked(crowdsecApi.writeCrowdsecFile).mockImplementationOnce(
      () =>
        new Promise((resolve) => {
          resolveWrite = () => resolve({})
        }),
    )
    await renderPage()
    expect(await screen.findByTestId('preset-preview')).toBeInTheDocument()
    await userEvent.selectOptions(screen.getByTestId('crowdsec-file-select'), 'acquis.yaml')
    // Use getAllByRole and filter for textarea (not the search input)
    const textareas = screen.getAllByRole('textbox')
    const textarea = textareas.find(el => el.tagName.toLowerCase() === 'textarea') as HTMLTextAreaElement
    await userEvent.type(textarea, 'x')
    await userEvent.click(screen.getByText('Save'))
    expect(await screen.findByText('Guardian inscribes...')).toBeInTheDocument()
    resolveWrite?.()
  })
})
