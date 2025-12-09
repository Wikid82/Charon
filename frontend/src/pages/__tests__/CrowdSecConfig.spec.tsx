import { describe, it, expect, vi, beforeEach } from 'vitest'
import { AxiosError } from 'axios'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { BrowserRouter } from 'react-router-dom'
import CrowdSecConfig from '../CrowdSecConfig'
import * as api from '../../api/security'
import * as crowdsecApi from '../../api/crowdsec'
import * as backupsApi from '../../api/backups'
import * as settingsApi from '../../api/settings'
import * as presetsApi from '../../api/presets'
import { CROWDSEC_PRESETS } from '../../data/crowdsecPresets'

vi.mock('../../api/security')
vi.mock('../../api/crowdsec')
vi.mock('../../api/backups')
vi.mock('../../api/settings')
vi.mock('../../api/presets')

const createQueryClient = () => new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } })
const renderWithProviders = (ui: React.ReactNode) => {
  const qc = createQueryClient()
  return render(
    <QueryClientProvider client={qc}>
      <BrowserRouter>
        {ui}
      </BrowserRouter>
    </QueryClientProvider>
  )
}

describe('CrowdSecConfig', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.mocked(presetsApi.listCrowdsecPresets).mockResolvedValue({
      presets: CROWDSEC_PRESETS.map((preset) => ({
        slug: preset.slug,
        title: preset.title,
        summary: preset.description,
        source: 'charon',
        requires_hub: false,
        available: true,
        cached: false,
      })),
    })
    vi.mocked(presetsApi.pullCrowdsecPreset).mockResolvedValue({
      status: 'pulled',
      slug: 'bot-mitigation-essentials',
      preview: CROWDSEC_PRESETS[0].content,
      cache_key: 'cache-123',
      etag: 'etag-123',
      retrieved_at: '2024-01-01T00:00:00Z',
      source: 'hub',
    })
    vi.mocked(presetsApi.applyCrowdsecPreset).mockResolvedValue({
      status: 'applied',
      backup: '/tmp/backup.tar.gz',
      reload_hint: 'CrowdSec reloaded',
      used_cscli: true,
      cache_key: 'cache-123',
      slug: 'bot-mitigation-essentials',
    })
    vi.mocked(presetsApi.getCrowdsecPresetCache).mockResolvedValue({ preview: 'cached', cache_key: 'cache-123', etag: 'etag-123' })
    vi.mocked(crowdsecApi.listCrowdsecDecisions).mockResolvedValue({ decisions: [] })
  })

  it('exports config when clicking Export', async () => {
    vi.mocked(api.getSecurityStatus).mockResolvedValue({ crowdsec: { enabled: true, mode: 'local', api_url: '' }, cerberus: { enabled: true }, waf: { enabled: false, mode: 'disabled' }, rate_limit: { enabled: false }, acl: { enabled: false } })
    vi.mocked(crowdsecApi.listCrowdsecFiles).mockResolvedValue({ files: [] })
    const blob = new Blob(['dummy'])
    vi.mocked(crowdsecApi.exportCrowdsecConfig).mockResolvedValue(blob)
    vi.spyOn(window, 'prompt').mockReturnValue('crowdsec-export')
    renderWithProviders(<CrowdSecConfig />)
    await waitFor(() => expect(screen.getByText('CrowdSec Configuration')).toBeInTheDocument())
    const exportBtn = screen.getByText('Export')
    await userEvent.click(exportBtn)
    await waitFor(() => expect(crowdsecApi.exportCrowdsecConfig).toHaveBeenCalled())
  })

  it('uploads a file and calls import on Import (backup before save)', async () => {
    vi.mocked(api.getSecurityStatus).mockResolvedValue({ crowdsec: { enabled: true, mode: 'local', api_url: '' }, cerberus: { enabled: true }, waf: { enabled: false, mode: 'disabled' }, rate_limit: { enabled: false }, acl: { enabled: false } })
    vi.mocked(backupsApi.createBackup).mockResolvedValue({ filename: 'backup.tar.gz' })
    vi.mocked(crowdsecApi.listCrowdsecFiles).mockResolvedValue({ files: [] })
    vi.mocked(crowdsecApi.importCrowdsecConfig).mockResolvedValue({ status: 'imported' })
    renderWithProviders(<CrowdSecConfig />)
    await waitFor(() => expect(screen.getByText('CrowdSec Configuration')).toBeInTheDocument())
    const input = screen.getByTestId('import-file') as HTMLInputElement
    const file = new File(['dummy'], 'cfg.tar.gz')
    await userEvent.upload(input, file)
    const btn = screen.getByTestId('import-btn')
    await userEvent.click(btn)
    await waitFor(() => expect(backupsApi.createBackup).toHaveBeenCalled())
    await waitFor(() => expect(crowdsecApi.importCrowdsecConfig).toHaveBeenCalled())
  })

  it('lists files, reads file content and can save edits (backup before save)', async () => {
    const status = { crowdsec: { enabled: true, mode: 'local' as const, api_url: '' }, cerberus: { enabled: true }, waf: { enabled: false, mode: 'disabled' as const }, rate_limit: { enabled: false }, acl: { enabled: false } }
    vi.mocked(api.getSecurityStatus).mockResolvedValue(status)
    vi.mocked(crowdsecApi.listCrowdsecFiles).mockResolvedValue({ files: ['conf.d/a.conf', 'b.conf'] })
    vi.mocked(crowdsecApi.readCrowdsecFile).mockResolvedValue({ content: 'rule1' })
    vi.mocked(backupsApi.createBackup).mockResolvedValue({ filename: 'backup.tar.gz' })
    vi.mocked(crowdsecApi.writeCrowdsecFile).mockResolvedValue({ status: 'written' })

    renderWithProviders(<CrowdSecConfig />)
    await waitFor(() => expect(screen.getByText('CrowdSec Configuration')).toBeInTheDocument())
    // wait for file list
    await waitFor(() => expect(screen.getByText('conf.d/a.conf')).toBeInTheDocument())
    const select = screen.getByTestId('crowdsec-file-select')
    await userEvent.selectOptions(select, 'conf.d/a.conf')
    await waitFor(() => expect(crowdsecApi.readCrowdsecFile).toHaveBeenCalledWith('conf.d/a.conf'))
    // ensure textarea populated
    const textarea = screen.getByRole('textbox')
    expect(textarea).toHaveValue('rule1')
    // edit and save
    await userEvent.clear(textarea)
    await userEvent.type(textarea, 'updated')
    const saveBtn = screen.getByText('Save')
    await userEvent.click(saveBtn)
    await waitFor(() => expect(backupsApi.createBackup).toHaveBeenCalled())
    await waitFor(() => expect(crowdsecApi.writeCrowdsecFile).toHaveBeenCalledWith('conf.d/a.conf', 'updated'))
  })

  it('persists crowdsec.mode via settings when changed', async () => {
    const status = { crowdsec: { enabled: true, mode: 'disabled' as const, api_url: '' }, cerberus: { enabled: true }, waf: { enabled: false, mode: 'disabled' as const }, rate_limit: { enabled: false }, acl: { enabled: false } }
    vi.mocked(api.getSecurityStatus).mockResolvedValue(status)
    vi.mocked(crowdsecApi.listCrowdsecFiles).mockResolvedValue({ files: [] })
    vi.mocked(settingsApi.updateSetting).mockResolvedValue(undefined)

    renderWithProviders(<CrowdSecConfig />)
    await waitFor(() => expect(screen.getByText('CrowdSec Configuration')).toBeInTheDocument())
    const modeToggle = screen.getByTestId('crowdsec-mode-toggle')
    await userEvent.click(modeToggle)
    await waitFor(() => expect(settingsApi.updateSetting).toHaveBeenCalledWith('security.crowdsec.mode', 'local', 'security', 'string'))
  })

  it('renders preset preview and applies with backup when backend apply is unavailable', async () => {
    const status = { crowdsec: { enabled: true, mode: 'local' as const, api_url: '' }, cerberus: { enabled: true }, waf: { enabled: false, mode: 'disabled' as const }, rate_limit: { enabled: false }, acl: { enabled: false } }
    const presetContent = CROWDSEC_PRESETS.find((preset) => preset.slug === 'bot-mitigation-essentials')?.content || ''
    vi.mocked(api.getSecurityStatus).mockResolvedValue(status)
    vi.mocked(crowdsecApi.listCrowdsecFiles).mockResolvedValue({ files: ['acquis.yaml'] })
    vi.mocked(crowdsecApi.readCrowdsecFile).mockResolvedValue({ content: '' })
    vi.mocked(backupsApi.createBackup).mockResolvedValue({ filename: 'backup.tar.gz' })
    vi.mocked(crowdsecApi.writeCrowdsecFile).mockResolvedValue({ status: 'written' })
    const axiosError = new AxiosError('not implemented', undefined, undefined, undefined, {
      status: 501,
      statusText: 'Not Implemented',
      headers: {},
      config: {},
      data: {},
    } as any)
    vi.mocked(presetsApi.applyCrowdsecPreset).mockRejectedValue(axiosError)

    renderWithProviders(<CrowdSecConfig />)
    await waitFor(() => expect(screen.getByText('CrowdSec Configuration')).toBeInTheDocument())
    await waitFor(() => expect(screen.getByTestId('preset-preview')).toHaveTextContent('configs:'))
    const fileSelect = screen.getByTestId('crowdsec-file-select')
    await userEvent.selectOptions(fileSelect, 'acquis.yaml')
    const applyBtn = screen.getByTestId('apply-preset-btn')
    await userEvent.click(applyBtn)

    await waitFor(() => expect(presetsApi.applyCrowdsecPreset).toHaveBeenCalledWith({ slug: 'bot-mitigation-essentials', cache_key: 'cache-123' }))
    await waitFor(() => expect(backupsApi.createBackup).toHaveBeenCalled())
    await waitFor(() => expect(crowdsecApi.writeCrowdsecFile).toHaveBeenCalledWith('acquis.yaml', presetContent))
  })

  it('surfaces validation error when slug is invalid', async () => {
    const status = { crowdsec: { enabled: true, mode: 'local' as const, api_url: '' }, cerberus: { enabled: true }, waf: { enabled: false, mode: 'disabled' as const }, rate_limit: { enabled: false }, acl: { enabled: false } }
    vi.mocked(api.getSecurityStatus).mockResolvedValue(status)
    vi.mocked(crowdsecApi.listCrowdsecFiles).mockResolvedValue({ files: [] })
    const validationError = new AxiosError('invalid', undefined, undefined, undefined, {
      status: 400,
      statusText: 'Bad Request',
      headers: {},
      config: {},
      data: { error: 'slug invalid' },
    } as any)
    vi.mocked(presetsApi.pullCrowdsecPreset).mockRejectedValueOnce(validationError)

    renderWithProviders(<CrowdSecConfig />)

    await waitFor(() => expect(screen.getByTestId('preset-validation-error')).toHaveTextContent('slug invalid'))
  })

  it('disables apply and offers cached preview when hub is unavailable', async () => {
    const status = { crowdsec: { enabled: true, mode: 'local' as const, api_url: '' }, cerberus: { enabled: true }, waf: { enabled: false, mode: 'disabled' as const }, rate_limit: { enabled: false }, acl: { enabled: false } }
    vi.mocked(api.getSecurityStatus).mockResolvedValue(status)
    vi.mocked(crowdsecApi.listCrowdsecFiles).mockResolvedValue({ files: [] })
    vi.mocked(presetsApi.listCrowdsecPresets).mockResolvedValueOnce({
      presets: [
        {
          slug: 'hub-only',
          title: 'Hub Only',
          summary: 'Needs hub',
          source: 'hub',
          requires_hub: true,
          available: true,
          cached: true,
          cache_key: 'cache-hub',
          etag: 'etag-hub',
        },
      ],
    })
    const hubError = new AxiosError('unavailable', undefined, undefined, undefined, {
      status: 503,
      statusText: 'Service Unavailable',
      headers: {},
      config: {},
      data: { error: 'hub service unavailable' },
    } as any)
    vi.mocked(presetsApi.pullCrowdsecPreset).mockRejectedValue(hubError)
    vi.mocked(presetsApi.getCrowdsecPresetCache).mockResolvedValue({ preview: 'cached-preview', cache_key: 'cache-hub', etag: 'etag-hub' })

    renderWithProviders(<CrowdSecConfig />)

  const select = await screen.findByTestId('preset-select')
  await waitFor(() => expect(screen.getByText('Hub Only')).toBeInTheDocument())
  await userEvent.selectOptions(select, 'hub-only')

    await waitFor(() => expect(screen.getByTestId('preset-hub-unavailable')).toBeInTheDocument())

    const applyBtn = screen.getByTestId('apply-preset-btn') as HTMLButtonElement
    expect(applyBtn.disabled).toBe(true)

    await userEvent.click(screen.getByText('Use cached preview'))
    await waitFor(() => expect(screen.getByTestId('preset-preview')).toHaveTextContent('cached-preview'))
  })

  it('shows apply response metadata including backup path', async () => {
    const status = { crowdsec: { enabled: true, mode: 'local' as const, api_url: '' }, cerberus: { enabled: true }, waf: { enabled: false, mode: 'disabled' as const }, rate_limit: { enabled: false }, acl: { enabled: false } }
    vi.mocked(api.getSecurityStatus).mockResolvedValue(status)
    vi.mocked(crowdsecApi.listCrowdsecFiles).mockResolvedValue({ files: ['acquis.yaml'] })
    vi.mocked(crowdsecApi.readCrowdsecFile).mockResolvedValue({ content: '' })
    vi.mocked(presetsApi.applyCrowdsecPreset).mockResolvedValueOnce({
      status: 'applied',
      backup: '/tmp/crowdsec-backup',
      reload_hint: 'crowdsec reloaded',
      used_cscli: true,
      cache_key: 'cache-123',
      slug: 'bot-mitigation-essentials',
    })

    renderWithProviders(<CrowdSecConfig />)

    const applyBtn = await screen.findByTestId('apply-preset-btn')
    await userEvent.click(applyBtn)

    await waitFor(() => expect(screen.getByTestId('preset-apply-info')).toHaveTextContent('/tmp/crowdsec-backup'))
    expect(screen.getByTestId('preset-apply-info')).toHaveTextContent('crowdsec reloaded')
  })
})
