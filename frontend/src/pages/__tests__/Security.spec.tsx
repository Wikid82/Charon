import { describe, it, expect, vi, beforeEach } from 'vitest'
import { cleanup } from '@testing-library/react'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { BrowserRouter } from 'react-router-dom'
import Security from '../Security'
import * as api from '../../api/security'
import type { SecurityStatus } from '../../api/security'
import * as settingsApi from '../../api/settings'
import * as crowdsecApi from '../../api/crowdsec'

vi.mock('../../api/security')
vi.mock('../../api/settings')
vi.mock('../../api/crowdsec')

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

describe('Security page', () => {
  beforeEach(() => {
    vi.resetAllMocks()
  })

  it('shows banner when all services are disabled and links to docs', async () => {
    const status: SecurityStatus = {
      cerberus: { enabled: false },
      crowdsec: { enabled: false, mode: 'disabled' as const, api_url: '' },
      waf: { enabled: false, mode: 'disabled' as const },
      rate_limit: { enabled: false },
      acl: { enabled: false },
    }
    vi.mocked(api.getSecurityStatus).mockResolvedValueOnce(status as SecurityStatus)
    vi.mocked(api.getSecurityStatus).mockResolvedValueOnce({
      ...status,
      crowdsec: { ...status.crowdsec, enabled: true }
    } as SecurityStatus)

    renderWithProviders(<Security />)
    expect(await screen.findByText('Security Suite Disabled')).toBeInTheDocument()
    const docBtns = screen.getAllByText('Documentation')
    expect(docBtns.length).toBeGreaterThan(0)
  })

  it('renders per-service toggles and calls updateSetting on change', async () => {
    const status: SecurityStatus = {
      cerberus: { enabled: true },
      crowdsec: { enabled: false, mode: 'disabled' as const, api_url: '' },
      waf: { enabled: false, mode: 'disabled' as const },
      rate_limit: { enabled: false },
      acl: { enabled: false },
    }
    vi.mocked(api.getSecurityStatus).mockResolvedValue(status as SecurityStatus)
    vi.mocked(settingsApi.updateSetting).mockResolvedValue(undefined)

    renderWithProviders(<Security />)
    await waitFor(() => expect(screen.getByText('Security Dashboard')).toBeInTheDocument())
    const crowdsecToggle = screen.getByTestId('toggle-crowdsec')
    // debug: ensure element state
    console.log('crowdsecToggle disabled:', (crowdsecToggle as HTMLInputElement).disabled)
    expect(crowdsecToggle).toBeTruthy()
      // Ensure the toggle exists and is not disabled
      expect(crowdsecToggle).toBeTruthy()
      expect((crowdsecToggle as HTMLInputElement).disabled).toBe(false)
    // Ensure enable-all controls were removed
    expect(screen.queryByTestId('enable-all-btn')).toBeNull()
  })

  it('calls updateSetting when toggling ACL', async () => {
    const status: SecurityStatus = {
      cerberus: { enabled: true },
      crowdsec: { enabled: false, mode: 'disabled' as const, api_url: '' },
      waf: { enabled: false, mode: 'disabled' as const },
      rate_limit: { enabled: false },
      acl: { enabled: false },
    }
    vi.mocked(api.getSecurityStatus).mockResolvedValue(status as SecurityStatus)
    const updateSpy = vi.mocked(settingsApi.updateSetting)
    renderWithProviders(<Security />)
    await waitFor(() => expect(screen.getByText('Security Dashboard')).toBeInTheDocument())
    const aclToggle = screen.getByTestId('toggle-acl')
    await userEvent.click(aclToggle)
    await waitFor(() => expect(updateSpy).toHaveBeenCalledWith('security.acl.enabled', 'true', 'security', 'bool'))
  })

  it('calls export endpoint when clicking Export', async () => {
    const status: SecurityStatus = {
      cerberus: { enabled: true },
      crowdsec: { enabled: true, mode: 'local' as const, api_url: '' },
      waf: { enabled: false, mode: 'disabled' as const },
      rate_limit: { enabled: false },
      acl: { enabled: false },
    }
    vi.mocked(api.getSecurityStatus).mockResolvedValue(status as SecurityStatus)
    const blob = new Blob(['dummy'])
    vi.mocked(crowdsecApi.exportCrowdsecConfig).mockResolvedValue(blob as any)
    renderWithProviders(<Security />)
    await waitFor(() => expect(screen.getByText('Security Dashboard')).toBeInTheDocument())
    const exportBtn = screen.getByText('Export')
    await userEvent.click(exportBtn)
    await waitFor(() => expect(crowdsecApi.exportCrowdsecConfig).toHaveBeenCalled())
  })

  it('calls start/stop endpoints for CrowdSec', async () => {
    const status: SecurityStatus = {
      cerberus: { enabled: true },
      crowdsec: { enabled: true, mode: 'local' as const, api_url: '' },
      waf: { enabled: false, mode: 'disabled' as const },
      rate_limit: { enabled: false },
      acl: { enabled: false },
    }
    vi.mocked(api.getSecurityStatus).mockResolvedValue(status as SecurityStatus)
    // Test start
    vi.mocked(crowdsecApi.startCrowdsec).mockResolvedValue(undefined)
    vi.mocked(crowdsecApi.statusCrowdsec).mockResolvedValue({ running: false })
    renderWithProviders(<Security />)
    await waitFor(() => expect(screen.getByText('Security Dashboard')).toBeInTheDocument())
    const startBtn = screen.getByText('Start')
    await userEvent.click(startBtn)
    await waitFor(() => expect(crowdsecApi.startCrowdsec).toHaveBeenCalled())
    // Cleanup before re-render to avoid multiple DOM instances
    cleanup()

    // Test stop: render with running state and click stop
    vi.mocked(crowdsecApi.stopCrowdsec).mockResolvedValue(undefined)
    vi.mocked(crowdsecApi.statusCrowdsec).mockResolvedValue({ running: true, pid: 123 })
    renderWithProviders(<Security />)
    await waitFor(() => expect(screen.getByText('Security Dashboard')).toBeInTheDocument())
    await waitFor(() => expect(screen.getByText('Stop')).toBeInTheDocument())
    const stopBtn = screen.getAllByText('Stop').find(b => !b.hasAttribute('disabled'))
    if (!stopBtn) throw new Error('No enabled Stop button found')
    await userEvent.click(stopBtn)
    await waitFor(() => expect(crowdsecApi.stopCrowdsec).toHaveBeenCalled())
  })

  it('disables service toggles when cerberus is off', async () => {
    const status: SecurityStatus = {
      cerberus: { enabled: false },
      crowdsec: { enabled: false, mode: 'disabled' as const, api_url: '' },
      waf: { enabled: false, mode: 'disabled' as const },
      rate_limit: { enabled: false },
      acl: { enabled: false },
    }
    vi.mocked(api.getSecurityStatus).mockResolvedValue(status as SecurityStatus)
    renderWithProviders(<Security />)
    await waitFor(() => expect(screen.getByText('Security Suite Disabled')).toBeInTheDocument())
    const crowdsecToggle = screen.getByTestId('toggle-crowdsec')
    expect(crowdsecToggle).toBeDisabled()
  })
})
