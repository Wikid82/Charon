import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { MemoryRouter } from 'react-router-dom'
import CrowdSecConfig from '../CrowdSecConfig'
import * as securityApi from '../../api/security'
import * as crowdsecApi from '../../api/crowdsec'
import * as backupsApi from '../../api/backups'
import * as settingsApi from '../../api/settings'
import * as presetsApi from '../../api/presets'
import { toast } from '../../utils/toast'

vi.mock('../../api/security')
vi.mock('../../api/crowdsec')
vi.mock('../../api/backups')
vi.mock('../../api/settings')
vi.mock('../../api/presets')
vi.mock('../../utils/toast', () => ({
  toast: {
    success: vi.fn(),
    error: vi.fn(),
  },
}))

describe('CrowdSecConfig', () => {
  const createClient = () =>
    new QueryClient({
      defaultOptions: {
        queries: { retry: false },
        mutations: { retry: false },
      },
    })

  const renderWithProviders = () => {
    const queryClient = createClient()
    return render(
      <QueryClientProvider client={queryClient}>
        <MemoryRouter>
          <CrowdSecConfig />
        </MemoryRouter>
      </QueryClientProvider>
    )
  }

  beforeEach(() => {
    vi.clearAllMocks()
    vi.mocked(securityApi.getSecurityStatus).mockResolvedValue({
      cerberus: { enabled: true },
      crowdsec: { mode: 'local', api_url: 'http://localhost', enabled: true },
      waf: { mode: 'enabled', enabled: true },
      rate_limit: { enabled: true },
      acl: { enabled: true },
    })
    vi.mocked(crowdsecApi.listCrowdsecFiles).mockResolvedValue({ files: [] })
    vi.mocked(crowdsecApi.readCrowdsecFile).mockResolvedValue({ content: '' })
    vi.mocked(crowdsecApi.writeCrowdsecFile).mockResolvedValue({})
    vi.mocked(crowdsecApi.listCrowdsecDecisions).mockResolvedValue({ decisions: [] })
    vi.mocked(crowdsecApi.exportCrowdsecConfig).mockResolvedValue(new Blob(['data']))
    vi.mocked(crowdsecApi.importCrowdsecConfig).mockResolvedValue({})
    vi.mocked(settingsApi.updateSetting).mockResolvedValue()
    vi.mocked(backupsApi.createBackup).mockResolvedValue({ filename: 'backup.tar.gz' })
    vi.mocked(presetsApi.listCrowdsecPresets).mockResolvedValue({ presets: [] })
    vi.mocked(presetsApi.pullCrowdsecPreset).mockResolvedValue({
      status: 'pulled',
      slug: 'bot-mitigation-essentials',
      preview: 'configs: {}',
      cache_key: 'cache-123',
    })
    vi.mocked(presetsApi.applyCrowdsecPreset).mockResolvedValue({ status: 'applied', backup: '/tmp/backup.tar.gz', cache_key: 'cache-123' })
    vi.mocked(presetsApi.getCrowdsecPresetCache).mockResolvedValue({ preview: 'configs: {}', cache_key: 'cache-123' })
    vi.spyOn(window, 'prompt').mockReturnValue('crowdsec-export.tar.gz')
    window.URL.createObjectURL = vi.fn(() => 'blob:url')
    window.URL.revokeObjectURL = vi.fn()
    vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => {})
  })

  it('toggles mode between local and disabled', async () => {
    renderWithProviders()

    await waitFor(() => screen.getByTestId('crowdsec-mode-toggle'))
    const toggle = screen.getByTestId('crowdsec-mode-toggle')

    await userEvent.click(toggle)

    await waitFor(() => {
      expect(settingsApi.updateSetting).toHaveBeenCalledWith(
        'security.crowdsec.mode',
        'disabled',
        'security',
        'string'
      )
      expect(toast.success).toHaveBeenCalledWith('CrowdSec disabled')
    })
  })

  it('exports configuration packages with prompted filename', async () => {
    renderWithProviders()

    await waitFor(() => screen.getByRole('button', { name: /Export/i }))
    const exportButton = screen.getByRole('button', { name: /Export/i })

    await userEvent.click(exportButton)

    await waitFor(() => {
      expect(crowdsecApi.exportCrowdsecConfig).toHaveBeenCalled()
      expect(toast.success).toHaveBeenCalledWith('CrowdSec configuration exported')
    })
  })

  it('shows Configuration Packages heading', async () => {
    renderWithProviders()

    await waitFor(() => screen.getByText('Configuration Packages'))

    expect(screen.getByText('Configuration Packages')).toBeInTheDocument()
  })
})
