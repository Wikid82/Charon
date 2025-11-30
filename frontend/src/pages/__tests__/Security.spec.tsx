import { describe, it, expect, vi, beforeEach } from 'vitest'
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
    vi.clearAllMocks()
  })

  it('shows banner when all services are disabled and links to docs', async () => {
    const status: SecurityStatus = {
      cerberus: { enabled: false },
      crowdsec: { enabled: false, mode: 'disabled' as const, api_url: '' },
      waf: { enabled: false, mode: 'disabled' as const },
      rate_limit: { enabled: false },
      acl: { enabled: false },
    }
    vi.mocked(api.getSecurityStatus).mockResolvedValue(status as SecurityStatus)

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
    expect(crowdsecToggle).toBeTruthy()
    await userEvent.click(crowdsecToggle)
    await waitFor(() => expect(settingsApi.updateSetting).toHaveBeenCalledWith('security.crowdsec.enabled', 'true', 'security', 'bool'))
    // Test Enable All toggles also call updateSetting for multiple keys
    const enableAllBtn = screen.getByTestId('enable-all-btn')
    await userEvent.click(enableAllBtn)
    await waitFor(() => expect(settingsApi.updateSetting).toHaveBeenCalledWith('security.waf.enabled', 'true', 'security', 'bool'))
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
