import { describe, it, expect, vi, beforeEach } from 'vitest'
import { AxiosError, AxiosResponse } from 'axios'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { BrowserRouter } from 'react-router-dom'
import CrowdSecConfig from '../CrowdSecConfig'
import * as api from '../../api/security'
import * as crowdsecApi from '../../api/crowdsec'
import * as backupsApi from '../../api/backups'
import * as presetsApi from '../../api/presets'
import * as featureFlagsApi from '../../api/featureFlags'
import { CROWDSEC_PRESETS } from '../../data/crowdsecPresets'
import type { ConsoleEnrollmentStatus } from '../../api/consoleEnrollment'

vi.mock('../../api/security')
vi.mock('../../api/crowdsec')
vi.mock('../../api/backups')
vi.mock('../../api/settings')
vi.mock('../../api/presets')
vi.mock('../../api/featureFlags')
vi.mock('../../components/CrowdSecBouncerKeyDisplay', () => ({
  CrowdSecBouncerKeyDisplay: () => null,
}))
const consoleStatusMock = vi.fn<() => ConsoleEnrollmentStatus>(() => ({ status: 'not_enrolled', key_present: false }))
const enrollConsoleMock = vi.fn()
const clearConsoleEnrollmentMock = vi.fn()

vi.mock('../../hooks/useConsoleEnrollment', () => ({
  useConsoleStatus: vi.fn(() => ({
    data: consoleStatusMock(),
    isLoading: false,
    isRefetching: false,
  })),
  useEnrollConsole: vi.fn(() => ({
    mutateAsync: enrollConsoleMock,
    isPending: false,
  })),
  useClearConsoleEnrollment: vi.fn(() => ({
    mutate: clearConsoleEnrollmentMock,
    isPending: false,
  })),
}))

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
      reload_hint: true,
      used_cscli: true,
      cache_key: 'cache-123',
      slug: 'bot-mitigation-essentials',
    })
    vi.mocked(presetsApi.getCrowdsecPresetCache).mockResolvedValue({ preview: 'cached', cache_key: 'cache-123', etag: 'etag-123' })
    vi.mocked(crowdsecApi.listCrowdsecDecisions).mockResolvedValue({ decisions: [] })
    vi.mocked(crowdsecApi.statusCrowdsec).mockResolvedValue({ running: true, pid: 1234, lapi_ready: true })
    vi.mocked(featureFlagsApi.getFeatureFlags).mockResolvedValue({
      'feature.crowdsec.console_enrollment': false,
    })
    consoleStatusMock.mockReturnValue({ status: 'not_enrolled', key_present: false })
    enrollConsoleMock.mockResolvedValue({ status: 'enrolling', key_present: true })
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

  it('hides console enrollment when feature flag is off', async () => {
    vi.mocked(api.getSecurityStatus).mockResolvedValue({ crowdsec: { enabled: true, mode: 'local' as const, api_url: '' }, cerberus: { enabled: true }, waf: { enabled: false, mode: 'disabled' as const }, rate_limit: { enabled: false }, acl: { enabled: false } })
    vi.mocked(crowdsecApi.listCrowdsecFiles).mockResolvedValue({ files: [] })

    renderWithProviders(<CrowdSecConfig />)

    await waitFor(() => expect(screen.getByText('CrowdSec Configuration')).toBeInTheDocument())
    expect(screen.queryByTestId('console-enrollment-card')).not.toBeInTheDocument()
  })

  it('shows console enrollment form when feature flag is on', async () => {
    vi.mocked(featureFlagsApi.getFeatureFlags).mockResolvedValue({ 'feature.crowdsec.console_enrollment': true })
    vi.mocked(api.getSecurityStatus).mockResolvedValue({ crowdsec: { enabled: true, mode: 'local' as const, api_url: '' }, cerberus: { enabled: true }, waf: { enabled: false, mode: 'disabled' as const }, rate_limit: { enabled: false }, acl: { enabled: false } })
    vi.mocked(crowdsecApi.listCrowdsecFiles).mockResolvedValue({ files: [] })

    renderWithProviders(<CrowdSecConfig />)

    await waitFor(() => expect(screen.getByTestId('console-enrollment-card')).toBeInTheDocument())
    expect(screen.getByTestId('console-enrollment-token')).toBeInTheDocument()
  })

  it('validates required console enrollment fields and acknowledgement', async () => {
    vi.mocked(featureFlagsApi.getFeatureFlags).mockResolvedValue({ 'feature.crowdsec.console_enrollment': true })
    vi.mocked(api.getSecurityStatus).mockResolvedValue({ crowdsec: { enabled: true, mode: 'local' as const, api_url: '' }, cerberus: { enabled: true }, waf: { enabled: false, mode: 'disabled' as const }, rate_limit: { enabled: false }, acl: { enabled: false } })
    vi.mocked(crowdsecApi.listCrowdsecFiles).mockResolvedValue({ files: [] })

    renderWithProviders(<CrowdSecConfig />)

    const enrollBtn = await screen.findByTestId('console-enroll-btn')

    // Button should be disabled when enrollment token is empty
    expect(enrollBtn).toBeDisabled()

    // Type only token (missing agent name, tenant, and ack)
    await userEvent.type(screen.getByTestId('console-enrollment-token'), 'token-123')

    // Now button should be enabled, click it
    await waitFor(() => expect(enrollBtn).not.toBeDisabled())
    await userEvent.click(enrollBtn)

    // Should show validation errors for missing fields
    const errors = await screen.findAllByTestId('console-enroll-error')
    expect(errors.length).toBeGreaterThan(0)
    expect(enrollConsoleMock).not.toHaveBeenCalled()
  })

  it('submits console enrollment payload with snake_case fields', async () => {
    vi.mocked(featureFlagsApi.getFeatureFlags).mockResolvedValue({ 'feature.crowdsec.console_enrollment': true })
    vi.mocked(api.getSecurityStatus).mockResolvedValue({ crowdsec: { enabled: true, mode: 'local' as const, api_url: '' }, cerberus: { enabled: true }, waf: { enabled: false, mode: 'disabled' as const }, rate_limit: { enabled: false }, acl: { enabled: false } })
    vi.mocked(crowdsecApi.listCrowdsecFiles).mockResolvedValue({ files: [] })
    enrollConsoleMock.mockResolvedValue({ status: 'enrolled', key_present: true, agent_name: 'agent-one', tenant: 'tenant-inc' })

    renderWithProviders(<CrowdSecConfig />)

    await waitFor(() => expect(screen.getByTestId('console-enrollment-card')).toBeInTheDocument())
    await userEvent.type(screen.getByTestId('console-enrollment-token'), 'secret-1234567890')
    await userEvent.clear(screen.getByTestId('console-agent-name'))
    await userEvent.type(screen.getByTestId('console-agent-name'), 'agent-one')
    await userEvent.type(screen.getByTestId('console-tenant'), 'tenant-inc')
    await userEvent.click(screen.getByTestId('console-ack-checkbox'))
    await userEvent.click(screen.getByTestId('console-enroll-btn'))

    await waitFor(() => expect(enrollConsoleMock).toHaveBeenCalledWith({
      enrollment_key: 'secret-1234567890',
      agent_name: 'agent-one',
      tenant: 'tenant-inc',
      force: false,
    }))

    expect((screen.getByTestId('console-enrollment-token') as HTMLInputElement).value).toBe('')
  })

  it('renders masked key state in console status', async () => {
    vi.mocked(featureFlagsApi.getFeatureFlags).mockResolvedValue({ 'feature.crowdsec.console_enrollment': true })
    vi.mocked(api.getSecurityStatus).mockResolvedValue({ crowdsec: { enabled: true, mode: 'local' as const, api_url: '' }, cerberus: { enabled: true }, waf: { enabled: false, mode: 'disabled' as const }, rate_limit: { enabled: false }, acl: { enabled: false } })
    vi.mocked(crowdsecApi.listCrowdsecFiles).mockResolvedValue({ files: [] })
    consoleStatusMock.mockReturnValue({ status: 'enrolled', key_present: true, agent_name: 'a1', tenant: 't1', last_heartbeat_at: '2024-01-01T00:00:00Z' })

    renderWithProviders(<CrowdSecConfig />)

    await waitFor(() => expect(screen.getByTestId('console-token-state')).toHaveTextContent('Stored (masked)'))
  })

  it('retries degraded enrollment and rotates key when enrolled', async () => {
    vi.mocked(featureFlagsApi.getFeatureFlags).mockResolvedValue({ 'feature.crowdsec.console_enrollment': true })
    vi.mocked(api.getSecurityStatus).mockResolvedValue({ crowdsec: { enabled: true, mode: 'local' as const, api_url: '' }, cerberus: { enabled: true }, waf: { enabled: false, mode: 'disabled' as const }, rate_limit: { enabled: false }, acl: { enabled: false } })
    vi.mocked(crowdsecApi.listCrowdsecFiles).mockResolvedValue({ files: [] })
    consoleStatusMock.mockReturnValue({ status: 'failed', key_present: true, last_error: 'network' })

    renderWithProviders(<CrowdSecConfig />)

    await waitFor(() => expect(screen.getByTestId('console-ack-checkbox')).toBeInTheDocument())
    await userEvent.type(screen.getByTestId('console-enrollment-token'), 'another-secret-123456')
    await userEvent.click(screen.getByTestId('console-ack-checkbox'))
    await userEvent.click(screen.getByTestId('console-retry-btn'))
    await waitFor(() => expect(enrollConsoleMock).toHaveBeenCalledWith(expect.objectContaining({ force: true })))

    await waitFor(() => expect(screen.getByTestId('console-rotate-btn')).not.toBeDisabled())
    await userEvent.type(screen.getByTestId('console-enrollment-token'), 'rotate-token-987654321')
    await userEvent.click(screen.getByTestId('console-rotate-btn'))
    await waitFor(() => expect(enrollConsoleMock).toHaveBeenCalledWith(expect.objectContaining({
      enrollment_key: 'rotate-token-987654321',
      force: true,
    })))
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
    // ensure textarea populated - use getAllByRole and filter for textarea (not the search input)
    const textareas = screen.getAllByRole('textbox')
    const textarea = textareas.find(el => el.tagName.toLowerCase() === 'textarea')!
    expect(textarea).toHaveValue('rule1')
    // edit and save
    await userEvent.clear(textarea)
    await userEvent.type(textarea, 'updated')
    const saveBtn = screen.getByText('Save')
    await userEvent.click(saveBtn)
    await waitFor(() => expect(backupsApi.createBackup).toHaveBeenCalled())
    await waitFor(() => expect(crowdsecApi.writeCrowdsecFile).toHaveBeenCalledWith('conf.d/a.conf', 'updated'))
  })

  it('shows info banner directing to Security Dashboard for mode control', async () => {
    const status = { crowdsec: { enabled: true, mode: 'local' as const, api_url: '' }, cerberus: { enabled: true }, waf: { enabled: false, mode: 'disabled' as const }, rate_limit: { enabled: false }, acl: { enabled: false } }
    vi.mocked(api.getSecurityStatus).mockResolvedValue(status)
    vi.mocked(crowdsecApi.listCrowdsecFiles).mockResolvedValue({ files: [] })

    renderWithProviders(<CrowdSecConfig />)
    await waitFor(() => expect(screen.getByText('CrowdSec Configuration')).toBeInTheDocument())
    expect(screen.getByText(/CrowdSec is controlled via the toggle on the/i)).toBeInTheDocument()
    expect(screen.getByRole('link', { name: /Security/i })).toHaveAttribute('href', '/security')
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
      config: { headers: {} },
      data: {},
    } as AxiosResponse)
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
      config: { headers: {} },
      data: { error: 'slug invalid' },
    } as AxiosResponse)
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
      config: { headers: {} },
      data: { error: 'hub service unavailable' },
    } as AxiosResponse)
    vi.mocked(presetsApi.pullCrowdsecPreset).mockRejectedValue(hubError)
    vi.mocked(presetsApi.getCrowdsecPresetCache).mockResolvedValue({ preview: 'cached-preview', cache_key: 'cache-hub', etag: 'etag-hub' })

    renderWithProviders(<CrowdSecConfig />)

  // Wait for presets to load and click on the preset card
  const presetCard = await screen.findByText('Hub Only')
  await userEvent.click(presetCard)

    await waitFor(() => expect(screen.getByTestId('preset-hub-unavailable')).toBeInTheDocument())

    const applyBtn = screen.getByTestId('apply-preset-btn') as HTMLButtonElement
    expect(applyBtn.disabled).toBe(true)

    await userEvent.click(screen.getByText('Use Cached'))
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
      reload_hint: true,
      used_cscli: true,
      cache_key: 'cache-123',
      slug: 'bot-mitigation-essentials',
    })

    renderWithProviders(<CrowdSecConfig />)

    const applyBtn = await screen.findByTestId('apply-preset-btn')
    await userEvent.click(applyBtn)

    await waitFor(() => expect(screen.getByTestId('preset-apply-info')).toHaveTextContent('/tmp/crowdsec-backup'))
    expect(screen.getByTestId('preset-apply-info')).toHaveTextContent('Status: applied')
    expect(screen.getByTestId('preset-apply-info')).toHaveTextContent('Method: cscli')
    // reloadHint is a boolean and renders as empty/true - just verify the info section exists
  })

  it('shows improved error message when preset is not cached', async () => {
    const axiosError = {
      isAxiosError: true,
      response: {
        status: 500,
        data: {
          error: 'CrowdSec preset not cached. Pull the preset first by clicking \'Pull Preview\', then try applying again.',
        },
      },
      message: 'Request failed',
    } as AxiosError

    vi.mocked(presetsApi.applyCrowdsecPreset).mockRejectedValueOnce(axiosError)

    renderWithProviders(<CrowdSecConfig />)

    const applyBtn = await screen.findByTestId('apply-preset-btn')
    await userEvent.click(applyBtn)

    await waitFor(() => expect(screen.getByTestId('preset-validation-error')).toBeInTheDocument())
    expect(screen.getByTestId('preset-validation-error')).toHaveTextContent('Preset must be pulled before applying')
  })
})
