import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { BrowserRouter } from 'react-router-dom'
import CrowdSecConfig from '../CrowdSecConfig'
import * as api from '../../api/security'
import * as crowdsecApi from '../../api/crowdsec'
import * as backupsApi from '../../api/backups'
import * as settingsApi from '../../api/settings'

vi.mock('../../api/security')
vi.mock('../../api/crowdsec')
vi.mock('../../api/backups')
vi.mock('../../api/settings')

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
  beforeEach(() => vi.clearAllMocks())

  it('exports config when clicking Export', async () => {
    vi.mocked(api.getSecurityStatus).mockResolvedValue({ crowdsec: { enabled: true, mode: 'local', api_url: '' }, cerberus: { enabled: true }, waf: { enabled: false, mode: 'disabled' }, rate_limit: { enabled: false }, acl: { enabled: false } } as any)
    vi.mocked(crowdsecApi.listCrowdsecFiles).mockResolvedValue({ files: [] } as any)
    const blob = new Blob(['dummy'])
    vi.mocked(crowdsecApi.exportCrowdsecConfig).mockResolvedValue(blob as any)
    renderWithProviders(<CrowdSecConfig />)
    await waitFor(() => expect(screen.getByText('CrowdSec Configuration')).toBeInTheDocument())
    const exportBtn = screen.getByText('Export')
    await userEvent.click(exportBtn)
    await waitFor(() => expect(crowdsecApi.exportCrowdsecConfig).toHaveBeenCalled())
  })

  it('uploads a file and calls import on Import (backup before save)', async () => {
    vi.mocked(api.getSecurityStatus).mockResolvedValue({ crowdsec: { enabled: true, mode: 'local', api_url: '' }, cerberus: { enabled: true }, waf: { enabled: false, mode: 'disabled' }, rate_limit: { enabled: false }, acl: { enabled: false } } as any)
    vi.mocked(backupsApi.createBackup).mockResolvedValue({ filename: 'backup.tar.gz' } as any)
    vi.mocked(crowdsecApi.listCrowdsecFiles).mockResolvedValue({ files: [] } as any)
    vi.mocked(crowdsecApi.importCrowdsecConfig).mockResolvedValue({ status: 'imported' } as any)
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
    const status = { crowdsec: { enabled: true, mode: 'local', api_url: '' }, cerberus: { enabled: true }, waf: { enabled: false, mode: 'disabled' }, rate_limit: { enabled: false }, acl: { enabled: false } } as any
    vi.mocked(api.getSecurityStatus).mockResolvedValue(status)
    vi.mocked(crowdsecApi.listCrowdsecFiles).mockResolvedValue({ files: ['conf.d/a.conf', 'b.conf'] } as any)
    vi.mocked(crowdsecApi.readCrowdsecFile).mockResolvedValue({ content: 'rule1' } as any)
    vi.mocked(backupsApi.createBackup).mockResolvedValue({ filename: 'backup.tar.gz' } as any)
    vi.mocked(crowdsecApi.writeCrowdsecFile).mockResolvedValue({ status: 'written' } as any)

    renderWithProviders(<CrowdSecConfig />)
    await waitFor(() => expect(screen.getByText('CrowdSec Configuration')).toBeInTheDocument())
    // wait for file list
    await waitFor(() => expect(screen.getByText('conf.d/a.conf')).toBeInTheDocument())
    const selects = screen.getAllByRole('combobox')
    const select = selects[1]
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
    const status = { crowdsec: { enabled: true, mode: 'disabled', api_url: '' }, cerberus: { enabled: true }, waf: { enabled: false, mode: 'disabled' }, rate_limit: { enabled: false }, acl: { enabled: false } } as any
    vi.mocked(api.getSecurityStatus).mockResolvedValue(status)
    vi.mocked(crowdsecApi.listCrowdsecFiles).mockResolvedValue({ files: [] } as any)
    vi.mocked(settingsApi.updateSetting).mockResolvedValue(undefined)

    renderWithProviders(<CrowdSecConfig />)
    await waitFor(() => expect(screen.getByText('CrowdSec Configuration')).toBeInTheDocument())
    const selects = screen.getAllByRole('combobox')
    const modeSelect = selects[0]
    await userEvent.selectOptions(modeSelect, 'local')
    await waitFor(() => expect(settingsApi.updateSetting).toHaveBeenCalledWith('security.crowdsec.mode', 'local', 'security', 'string'))
  })
})
