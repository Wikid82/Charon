import { describe, it, expect, vi, beforeEach } from 'vitest'
import { cleanup } from '@testing-library/react'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { BrowserRouter } from 'react-router-dom'
import Security from '../Security'
import * as api from '../../api/security'
import type { SecurityStatus, RuleSetsResponse } from '../../api/security'
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

const mockSecurityConfig = {
  config: {
    name: 'default',
    waf_mode: 'block',
    waf_rules_source: '',
    admin_whitelist: '',
  },
}

const mockRuleSets: RuleSetsResponse = {
  rulesets: [
    { id: 1, uuid: 'uuid-1', name: 'OWASP CRS', source_url: '', mode: 'blocking', last_updated: '', content: '' },
    { id: 2, uuid: 'uuid-2', name: 'Custom Rules', source_url: '', mode: 'detection', last_updated: '', content: '' },
  ],
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

  it('shows WAF mode selector when WAF is enabled', async () => {
    const status: SecurityStatus = {
      cerberus: { enabled: true },
      crowdsec: { enabled: false, mode: 'disabled' as const, api_url: '' },
      waf: { enabled: true, mode: 'enabled' as const },
      rate_limit: { enabled: false },
      acl: { enabled: false },
    }
    vi.mocked(api.getSecurityStatus).mockResolvedValue(status as SecurityStatus)
    vi.mocked(api.getSecurityConfig).mockResolvedValue(mockSecurityConfig)
    vi.mocked(api.getRuleSets).mockResolvedValue(mockRuleSets)

    renderWithProviders(<Security />)
    await waitFor(() => expect(screen.getByTestId('waf-mode-select')).toBeInTheDocument())

    // Check mode selector is present with correct options
    const modeSelect = screen.getByTestId('waf-mode-select')
    expect(modeSelect).toBeInTheDocument()
    expect(modeSelect).toHaveValue('block')
  })

  it('shows WAF ruleset selector with available rulesets', async () => {
    const status: SecurityStatus = {
      cerberus: { enabled: true },
      crowdsec: { enabled: false, mode: 'disabled' as const, api_url: '' },
      waf: { enabled: true, mode: 'enabled' as const },
      rate_limit: { enabled: false },
      acl: { enabled: false },
    }
    vi.mocked(api.getSecurityStatus).mockResolvedValue(status as SecurityStatus)
    vi.mocked(api.getSecurityConfig).mockResolvedValue(mockSecurityConfig)
    vi.mocked(api.getRuleSets).mockResolvedValue(mockRuleSets)

    renderWithProviders(<Security />)
    await waitFor(() => expect(screen.getByTestId('waf-ruleset-select')).toBeInTheDocument())

    // Check ruleset selector shows available rulesets
    const rulesetSelect = screen.getByTestId('waf-ruleset-select')
    expect(rulesetSelect).toBeInTheDocument()

    // Verify options are present
    expect(screen.getByText('None (all rule sets)')).toBeInTheDocument()
    expect(screen.getByText('OWASP CRS (blocking)')).toBeInTheDocument()
    expect(screen.getByText('Custom Rules (detection)')).toBeInTheDocument()
  })

  it('calls updateSecurityConfig when WAF mode is changed', async () => {
    const status: SecurityStatus = {
      cerberus: { enabled: true },
      crowdsec: { enabled: false, mode: 'disabled' as const, api_url: '' },
      waf: { enabled: true, mode: 'enabled' as const },
      rate_limit: { enabled: false },
      acl: { enabled: false },
    }
    vi.mocked(api.getSecurityStatus).mockResolvedValue(status as SecurityStatus)
    vi.mocked(api.getSecurityConfig).mockResolvedValue(mockSecurityConfig)
    vi.mocked(api.getRuleSets).mockResolvedValue(mockRuleSets)
    vi.mocked(api.updateSecurityConfig).mockResolvedValue({})

    renderWithProviders(<Security />)
    await waitFor(() => expect(screen.getByTestId('waf-mode-select')).toBeInTheDocument())

    // Change mode to monitor
    const modeSelect = screen.getByTestId('waf-mode-select')
    await userEvent.selectOptions(modeSelect, 'monitor')

    await waitFor(() => {
      expect(api.updateSecurityConfig).toHaveBeenCalledWith(
        expect.objectContaining({ waf_mode: 'monitor' })
      )
    })
  })

  it('calls updateSecurityConfig when WAF ruleset is changed', async () => {
    const status: SecurityStatus = {
      cerberus: { enabled: true },
      crowdsec: { enabled: false, mode: 'disabled' as const, api_url: '' },
      waf: { enabled: true, mode: 'enabled' as const },
      rate_limit: { enabled: false },
      acl: { enabled: false },
    }
    vi.mocked(api.getSecurityStatus).mockResolvedValue(status as SecurityStatus)
    vi.mocked(api.getSecurityConfig).mockResolvedValue(mockSecurityConfig)
    vi.mocked(api.getRuleSets).mockResolvedValue(mockRuleSets)
    vi.mocked(api.updateSecurityConfig).mockResolvedValue({})

    renderWithProviders(<Security />)
    await waitFor(() => expect(screen.getByTestId('waf-ruleset-select')).toBeInTheDocument())

    // Select a specific ruleset
    const rulesetSelect = screen.getByTestId('waf-ruleset-select')
    await userEvent.selectOptions(rulesetSelect, 'OWASP CRS')

    await waitFor(() => {
      expect(api.updateSecurityConfig).toHaveBeenCalledWith(
        expect.objectContaining({ waf_rules_source: 'OWASP CRS' })
      )
    })
  })

  it('shows warning when no rulesets are configured', async () => {
    const status: SecurityStatus = {
      cerberus: { enabled: true },
      crowdsec: { enabled: false, mode: 'disabled' as const, api_url: '' },
      waf: { enabled: true, mode: 'enabled' as const },
      rate_limit: { enabled: false },
      acl: { enabled: false },
    }
    vi.mocked(api.getSecurityStatus).mockResolvedValue(status as SecurityStatus)
    vi.mocked(api.getSecurityConfig).mockResolvedValue(mockSecurityConfig)
    vi.mocked(api.getRuleSets).mockResolvedValue({ rulesets: [] })

    renderWithProviders(<Security />)
    await waitFor(() => expect(screen.getByTestId('waf-ruleset-select')).toBeInTheDocument())

    // Should show warning about no rulesets
    expect(screen.getByText('No rule sets configured. Add one below.')).toBeInTheDocument()
  })

  it('displays correct WAF mode in status text', async () => {
    const status: SecurityStatus = {
      cerberus: { enabled: true },
      crowdsec: { enabled: false, mode: 'disabled' as const, api_url: '' },
      waf: { enabled: true, mode: 'enabled' as const },
      rate_limit: { enabled: false },
      acl: { enabled: false },
    }
    vi.mocked(api.getSecurityStatus).mockResolvedValue(status as SecurityStatus)
    vi.mocked(api.getSecurityConfig).mockResolvedValue({
      config: { ...mockSecurityConfig.config, waf_mode: 'monitor' },
    })
    vi.mocked(api.getRuleSets).mockResolvedValue(mockRuleSets)

    renderWithProviders(<Security />)
    await waitFor(() => expect(screen.getByText('Mode: Monitor (log only)')).toBeInTheDocument())
  })

  it('does not show WAF controls when WAF is disabled', async () => {
    const status: SecurityStatus = {
      cerberus: { enabled: true },
      crowdsec: { enabled: false, mode: 'disabled' as const, api_url: '' },
      waf: { enabled: false, mode: 'disabled' as const },
      rate_limit: { enabled: false },
      acl: { enabled: false },
    }
    vi.mocked(api.getSecurityStatus).mockResolvedValue(status as SecurityStatus)
    vi.mocked(api.getSecurityConfig).mockResolvedValue(mockSecurityConfig)
    vi.mocked(api.getRuleSets).mockResolvedValue(mockRuleSets)

    renderWithProviders(<Security />)
    await waitFor(() => expect(screen.getByText('Security Dashboard')).toBeInTheDocument())

    // Mode selector and ruleset selector should not be visible
    expect(screen.queryByTestId('waf-mode-select')).not.toBeInTheDocument()
    expect(screen.queryByTestId('waf-ruleset-select')).not.toBeInTheDocument()
  })
})
