import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router-dom'
import { describe, it, expect, vi, beforeEach } from 'vitest'

import * as backupsApi from '../../api/backups'
import * as crowdsecApi from '../../api/crowdsec'
import * as featureFlagsApi from '../../api/featureFlags'
import * as presetsApi from '../../api/presets'
import * as securityApi from '../../api/security'
import { toast } from '../../utils/toast'
import CrowdSecConfig from '../CrowdSecConfig'

vi.mock('../../api/security')
vi.mock('../../api/crowdsec')
vi.mock('../../api/backups')
vi.mock('../../api/presets')
vi.mock('../../api/featureFlags')
vi.mock('../../components/CrowdSecBouncerKeyDisplay', () => ({
  CrowdSecBouncerKeyDisplay: () => null,
}))
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
vi.mock('../../utils/toast', () => ({
  toast: {
    success: vi.fn(),
    error: vi.fn(),
    info: vi.fn(),
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

  const renderComponent = () => {
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

    // Default mocks
    vi.mocked(securityApi.getSecurityStatus).mockResolvedValue({
      cerberus: { enabled: true },
      crowdsec: { mode: 'local', api_url: 'http://localhost', enabled: true },
      waf: { mode: 'enabled', enabled: true },
      rate_limit: { enabled: true },
      acl: { enabled: true },
    })

    vi.mocked(featureFlagsApi.getFeatureFlags).mockResolvedValue({
        'feature.crowdsec.console_enrollment': true
    })

    vi.mocked(crowdsecApi.listCrowdsecFiles).mockResolvedValue({ files: ['config.yaml', 'profiles.yaml'] })
    vi.mocked(crowdsecApi.readCrowdsecFile).mockResolvedValue({ content: 'yaml content' })
    vi.mocked(crowdsecApi.writeCrowdsecFile).mockResolvedValue({})
    vi.mocked(crowdsecApi.listCrowdsecDecisions).mockResolvedValue({
        decisions: [
            { id: '1', ip: '1.2.3.4', reason: 'ssh-bf', duration: '23h', created_at: '2023-01-01', source: 'local' }
        ]
    })
    vi.mocked(crowdsecApi.exportCrowdsecConfig).mockResolvedValue(new Blob(['data']))
    vi.mocked(crowdsecApi.importCrowdsecConfig).mockResolvedValue({})
    vi.mocked(crowdsecApi.banIP).mockResolvedValue()
    vi.mocked(crowdsecApi.unbanIP).mockResolvedValue()
    vi.mocked(crowdsecApi.statusCrowdsec).mockResolvedValue({
      running: true,
      pid: 123,
      lapi_ready: true,
    })

    vi.mocked(backupsApi.createBackup).mockResolvedValue({ filename: 'backup.tar.gz' })
    vi.mocked(presetsApi.listCrowdsecPresets).mockResolvedValue({ presets: [] })
    vi.mocked(presetsApi.pullCrowdsecPreset).mockResolvedValue({
      status: 'pulled',
      slug: 'bot-mitigation-essentials',
      preview: 'configs: {}',
      cache_key: 'cache-123',
    })

    // Window Prompt Mock
    vi.spyOn(window, 'prompt').mockReturnValue('crowdsec-export.tar.gz')
    window.URL.createObjectURL = vi.fn(() => 'blob:url')
    window.URL.revokeObjectURL = vi.fn()
  })

  // 1. Rendering basic elements
  it('renders page configuration elements', async () => {
    renderComponent()

    await waitFor(() => {
        expect(screen.getByText('CrowdSec Configuration')).toBeInTheDocument()
        expect(screen.getByText('Configuration Packages')).toBeInTheDocument()
        // Updated text to match translation file
        expect(screen.getByText('Edit Configuration Files')).toBeInTheDocument()
        expect(screen.getByText('Banned IPs')).toBeInTheDocument()
    })
  })

  // 2. File Editor
  it('allows reading and saving config files', async () => {
    const user = userEvent.setup()
    renderComponent()

    await waitFor(() => screen.getByTestId('crowdsec-file-select'))

    // Select file
    const select = screen.getByTestId('crowdsec-file-select')
    await user.selectOptions(select, 'config.yaml')

    await waitFor(() => {
        expect(crowdsecApi.readCrowdsecFile).toHaveBeenCalledWith('config.yaml')
        expect(screen.getByDisplayValue('yaml content')).toBeInTheDocument()
    })

    // Edit content
    const textarea = screen.getByDisplayValue('yaml content')
    await user.clear(textarea)
    await user.type(textarea, 'new content')

    // Save
    await user.click(screen.getByRole('button', { name: 'Save' }))

    await waitFor(() => {
        expect(crowdsecApi.writeCrowdsecFile).toHaveBeenCalledWith('config.yaml', 'new content')
        expect(backupsApi.createBackup).toHaveBeenCalled() // Should backup first
    })
  })

  // 3. Banned IPs Table
  it('renders banned IPs table', async () => {
    renderComponent()

    await waitFor(() => {
        expect(screen.getByText('1.2.3.4')).toBeInTheDocument()
        expect(screen.getByText('ssh-bf')).toBeInTheDocument()
    })
  })

  // 4. Ban IP Action
  it('allows banning an IP', async () => {
    const user = userEvent.setup()
    renderComponent()

    await waitFor(() => screen.getByText('Ban IP'))

    // Click Ban IP trigger (using ID we added)
    await user.click(screen.getByTestId('ban-ip-trigger'))

    // Modal opens
    const dialog = await screen.findByRole('dialog', { name: 'Ban IP Address' })

    // Fill form
    await user.type(within(dialog).getByLabelText(/IP Address/i), '5.6.7.8')
    await user.type(within(dialog).getByPlaceholderText(/Reason for ban/i), 'manual ban')

    await user.click(within(dialog).getByRole('button', { name: 'Ban IP' }))

    await waitFor(() => {
        expect(crowdsecApi.banIP).toHaveBeenCalledWith('5.6.7.8', '24h', 'manual ban')
        expect(toast.success).toHaveBeenCalled()
    })
  })

  // 5. Unban IP Action
  it('allows unbanning an IP', async () => {
    const user = userEvent.setup()
    renderComponent()

    await waitFor(() => screen.getByText('1.2.3.4'))

    const unbanBtns = screen.getAllByRole('button', { name: 'Unban' })
    expect(unbanBtns.length).toBeGreaterThan(0)

    // Click the unban button in the table (first one)
    await user.click(unbanBtns[0])

    // Confirm modal
    await waitFor(() => screen.getByText('Confirm Unban'))

    // Click confirm in modal. Use getAllByRole to get the modal one (last one)
    const modalButtons = screen.getAllByRole('button', { name: 'Unban' })
    const confirmBtn = modalButtons[modalButtons.length - 1]
    await user.click(confirmBtn)

    await waitFor(() => {
        expect(crowdsecApi.unbanIP).toHaveBeenCalledWith('1.2.3.4')
        expect(toast.success).toHaveBeenCalled()
    })
  })

  // 6. Console Enrollment fields (if enabled)
  it('handles console enrollment form', async () => {
    // const user = userEvent.setup()
    renderComponent()

    await waitFor(() => screen.getByText('Console Enrollment'))

    // Check inputs exist
    expect(screen.getByTestId('console-enrollment-token')).toBeInTheDocument()
    expect(screen.getByTestId('console-agent-name')).toBeInTheDocument()
  })

  // 7. Presets logic
  it('handles preset searching', async () => {
    const user = userEvent.setup()

    // Mock presets with data
    vi.mocked(presetsApi.listCrowdsecPresets).mockResolvedValue({
      presets: [
        {
          slug: 'ssh-bf',
          title: 'SSH Bruteforce',
          summary: 'Block SSH attacks',
          source: 'crowdsec',
          tags: ['linux'],
          requires_hub: false,
          available: true,
          cached: false,
        },
      ]
    })

    renderComponent()

    await waitFor(() => {
        expect(presetsApi.listCrowdsecPresets).toHaveBeenCalled()
    })

    const searchInput = screen.getByPlaceholderText('Search presets...')
    expect(searchInput).toBeInTheDocument()

    await user.type(searchInput, 'SSH')
    expect(searchInput).toHaveValue('SSH')
  })
})
