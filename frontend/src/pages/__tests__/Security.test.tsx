import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { BrowserRouter } from 'react-router-dom'
import Security from '../Security'
import * as securityApi from '../../api/security'
import * as crowdsecApi from '../../api/crowdsec'
import * as settingsApi from '../../api/settings'
import { toast } from '../../utils/toast'

vi.mock('../../api/security')
vi.mock('../../api/crowdsec')
vi.mock('../../api/settings')
vi.mock('../../utils/toast', () => ({
  toast: {
    success: vi.fn(),
    error: vi.fn(),
  },
}))
vi.mock('../../hooks/useSecurity', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../../hooks/useSecurity')>()
  return {
    ...actual,
    useSecurityConfig: vi.fn(() => ({ data: { config: { admin_whitelist: '10.0.0.0/8' } } })),
    useUpdateSecurityConfig: vi.fn(() => ({ mutate: vi.fn(), isPending: false })),
    useGenerateBreakGlassToken: vi.fn(() => ({ mutate: vi.fn(), isPending: false })),
    useRuleSets: vi.fn(() => ({
      data: {
        rulesets: [
          { id: 1, uuid: 'abc', name: 'OWASP CRS', source_url: 'https://example.com', mode: 'blocking', last_updated: '2025-12-04', content: 'rules' }
        ]
      }
    })),
  }
})

describe('Security', () => {
  let queryClient: QueryClient

  beforeEach(() => {
    queryClient = new QueryClient({
      defaultOptions: {
        queries: { retry: false },
        mutations: { retry: false },
      },
    })
    vi.clearAllMocks()
  })

  const wrapper = ({ children }: { children: React.ReactNode }) => (
    <QueryClientProvider client={queryClient}>
      <BrowserRouter>{children}</BrowserRouter>
    </QueryClientProvider>
  )

  const mockSecurityStatus = {
    cerberus: { enabled: true },
    crowdsec: { mode: 'local' as const, api_url: 'http://localhost', enabled: true },
    waf: { mode: 'enabled' as const, enabled: true },
    rate_limit: { enabled: true },
    acl: { enabled: true }
  }

  describe('Rendering', () => {
    it('should show loading state initially', () => {
      vi.mocked(securityApi.getSecurityStatus).mockReturnValue(new Promise(() => {}))
      render(<Security />, { wrapper })
      expect(screen.getByText(/Loading security status/i)).toBeInTheDocument()
    })

    it('should show error if security status fails to load', async () => {
      vi.mocked(securityApi.getSecurityStatus).mockRejectedValue(new Error('Failed'))
      render(<Security />, { wrapper })
      await waitFor(() => expect(screen.getByText(/Failed to load security status/i)).toBeInTheDocument())
    })

    it('should render Security Dashboard when status loads', async () => {
      vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(mockSecurityStatus)
      render(<Security />, { wrapper })
      await waitFor(() => expect(screen.getByText(/Security Dashboard/i)).toBeInTheDocument())
    })

    it('should show banner when Cerberus is disabled', async () => {
      vi.mocked(securityApi.getSecurityStatus).mockResolvedValue({ ...mockSecurityStatus, cerberus: { enabled: false } })
      render(<Security />, { wrapper })
      await waitFor(() => expect(screen.getByText(/Security Suite Disabled/i)).toBeInTheDocument())
    })
  })

  describe('Cerberus Toggle', () => {
    it('should toggle Cerberus on', async () => {
      const user = userEvent.setup()
      vi.mocked(securityApi.getSecurityStatus).mockResolvedValue({ ...mockSecurityStatus, cerberus: { enabled: false } })
      vi.mocked(settingsApi.updateSetting).mockResolvedValue()

      render(<Security />, { wrapper })

      await waitFor(() => screen.getByTestId('toggle-cerberus'))
      const toggle = screen.getByTestId('toggle-cerberus')
      await user.click(toggle)

      await waitFor(() => expect(settingsApi.updateSetting).toHaveBeenCalledWith('security.cerberus.enabled', 'true', 'security', 'bool'))
    })

    it('should toggle Cerberus off', async () => {
      const user = userEvent.setup()
      vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(mockSecurityStatus)
      vi.mocked(settingsApi.updateSetting).mockResolvedValue()

      render(<Security />, { wrapper })

      await waitFor(() => screen.getByTestId('toggle-cerberus'))
      const toggle = screen.getByTestId('toggle-cerberus')
      await user.click(toggle)

      await waitFor(() => expect(settingsApi.updateSetting).toHaveBeenCalledWith('security.cerberus.enabled', 'false', 'security', 'bool'))
    })
  })

  describe('Service Toggles', () => {
    it('should toggle CrowdSec on', async () => {
      const user = userEvent.setup()
      vi.mocked(securityApi.getSecurityStatus).mockResolvedValue({ ...mockSecurityStatus, crowdsec: { mode: 'local', api_url: 'http://localhost', enabled: false } })
      vi.mocked(settingsApi.updateSetting).mockResolvedValue()

      render(<Security />, { wrapper })

      await waitFor(() => screen.getByTestId('toggle-crowdsec'))
      const toggle = screen.getByTestId('toggle-crowdsec')
      await user.click(toggle)

      await waitFor(() => expect(settingsApi.updateSetting).toHaveBeenCalledWith('security.crowdsec.enabled', 'true', 'security', 'bool'))
    })

    it('should toggle WAF on', async () => {
      const user = userEvent.setup()
      vi.mocked(securityApi.getSecurityStatus).mockResolvedValue({ ...mockSecurityStatus, waf: { mode: 'enabled', enabled: false } })
      vi.mocked(settingsApi.updateSetting).mockResolvedValue()

      render(<Security />, { wrapper })

      await waitFor(() => screen.getByTestId('toggle-waf'))
      const toggle = screen.getByTestId('toggle-waf')
      await user.click(toggle)

      await waitFor(() => expect(settingsApi.updateSetting).toHaveBeenCalledWith('security.waf.enabled', 'true', 'security', 'bool'))
    })

    it('should toggle ACL on', async () => {
      const user = userEvent.setup()
      vi.mocked(securityApi.getSecurityStatus).mockResolvedValue({ ...mockSecurityStatus, acl: { enabled: false } })
      vi.mocked(settingsApi.updateSetting).mockResolvedValue()

      render(<Security />, { wrapper })

      await waitFor(() => screen.getByTestId('toggle-acl'))
      const toggle = screen.getByTestId('toggle-acl')
      await user.click(toggle)

      await waitFor(() => expect(settingsApi.updateSetting).toHaveBeenCalledWith('security.acl.enabled', 'true', 'security', 'bool'))
    })

    it('should toggle Rate Limiting on', async () => {
      const user = userEvent.setup()
      vi.mocked(securityApi.getSecurityStatus).mockResolvedValue({ ...mockSecurityStatus, rate_limit: { enabled: false } })
      vi.mocked(settingsApi.updateSetting).mockResolvedValue()

      render(<Security />, { wrapper })

      await waitFor(() => screen.getByTestId('toggle-rate-limit'))
      const toggle = screen.getByTestId('toggle-rate-limit')
      await user.click(toggle)

      await waitFor(() => expect(settingsApi.updateSetting).toHaveBeenCalledWith('security.rate_limit.enabled', 'true', 'security', 'bool'))
    })
  })

  describe('Admin Whitelist', () => {
    it('should load admin whitelist from config', async () => {
      vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(mockSecurityStatus)
      render(<Security />, { wrapper })

      await waitFor(() => screen.getByDisplayValue('10.0.0.0/8'))
      expect(screen.getByDisplayValue('10.0.0.0/8')).toBeInTheDocument()
    })

    it('should update admin whitelist on save', async () => {
      const user = userEvent.setup()
      const mockMutate = vi.fn()
      const { useUpdateSecurityConfig } = await import('../../hooks/useSecurity')
      vi.mocked(useUpdateSecurityConfig).mockReturnValue({ mutate: mockMutate, isPending: false } as any)
      vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(mockSecurityStatus)

      render(<Security />, { wrapper })

      await waitFor(() => screen.getByDisplayValue('10.0.0.0/8'))

      const saveButton = screen.getByRole('button', { name: /Save/i })
      await user.click(saveButton)

      await waitFor(() => {
        expect(mockMutate).toHaveBeenCalledWith({ name: 'default', admin_whitelist: '10.0.0.0/8' })
      })
    })
  })

  describe('CrowdSec Controls', () => {
    it('should start CrowdSec', async () => {
      const user = userEvent.setup()
      vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(mockSecurityStatus)
      vi.mocked(crowdsecApi.statusCrowdsec).mockResolvedValue({ running: false })
      vi.mocked(crowdsecApi.startCrowdsec).mockResolvedValue({ success: true })

      render(<Security />, { wrapper })

      await waitFor(() => screen.getByTestId('crowdsec-start'))
      const startButton = screen.getByTestId('crowdsec-start')
      await user.click(startButton)

      await waitFor(() => expect(crowdsecApi.startCrowdsec).toHaveBeenCalled())
    })

    it('should stop CrowdSec', async () => {
      const user = userEvent.setup()
      vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(mockSecurityStatus)
      vi.mocked(crowdsecApi.statusCrowdsec).mockResolvedValue({ running: true, pid: 1234 })
      vi.mocked(crowdsecApi.stopCrowdsec).mockResolvedValue({ success: true })

      render(<Security />, { wrapper })

      await waitFor(() => screen.getByTestId('crowdsec-stop'))
      const stopButton = screen.getByTestId('crowdsec-stop')
      await user.click(stopButton)

      await waitFor(() => expect(crowdsecApi.stopCrowdsec).toHaveBeenCalled())
    })

    it('should export CrowdSec config', async () => {
      const user = userEvent.setup()
      vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(mockSecurityStatus)
      vi.mocked(crowdsecApi.exportCrowdsecConfig).mockResolvedValue('config data' as any)
      window.URL.createObjectURL = vi.fn(() => 'blob:url')
      window.URL.revokeObjectURL = vi.fn()

      render(<Security />, { wrapper })

      await waitFor(() => screen.getByRole('button', { name: /Export/i }))
      const exportButton = screen.getByRole('button', { name: /Export/i })
      await user.click(exportButton)

      await waitFor(() => {
        expect(crowdsecApi.exportCrowdsecConfig).toHaveBeenCalled()
        expect(toast.success).toHaveBeenCalledWith('CrowdSec configuration exported')
      })
    })
  })

  describe('WAF Controls', () => {
    it('should change WAF mode', async () => {
      const user = userEvent.setup()
      const { useUpdateSecurityConfig } = await import('../../hooks/useSecurity')
      const mockMutate = vi.fn()
      vi.mocked(useUpdateSecurityConfig).mockReturnValue({ mutate: mockMutate, isPending: false } as any)
      vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(mockSecurityStatus)

      render(<Security />, { wrapper })

      await waitFor(() => screen.getByTestId('waf-mode-select'))
      const select = screen.getByTestId('waf-mode-select')
      await user.selectOptions(select, 'monitor')

      await waitFor(() => expect(mockMutate).toHaveBeenCalledWith({ name: 'default', waf_mode: 'monitor' }))
    })

    it('should change WAF ruleset', async () => {
      const user = userEvent.setup()
      const { useUpdateSecurityConfig } = await import('../../hooks/useSecurity')
      const mockMutate = vi.fn()
      vi.mocked(useUpdateSecurityConfig).mockReturnValue({ mutate: mockMutate, isPending: false } as any)
      vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(mockSecurityStatus)

      render(<Security />, { wrapper })

      await waitFor(() => screen.getByTestId('waf-ruleset-select'))
      const select = screen.getByTestId('waf-ruleset-select')
      await user.selectOptions(select, 'OWASP CRS')

      await waitFor(() => expect(mockMutate).toHaveBeenCalledWith({ name: 'default', waf_rules_source: 'OWASP CRS' }))
    })
  })

  describe('Loading Overlay', () => {
    it('should show Cerberus overlay when Cerberus is toggling', async () => {
      const user = userEvent.setup()
      vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(mockSecurityStatus)
      vi.mocked(settingsApi.updateSetting).mockImplementation(() => new Promise(() => {}))

      render(<Security />, { wrapper })

      await waitFor(() => screen.getByTestId('toggle-cerberus'))
      const toggle = screen.getByTestId('toggle-cerberus')
      await user.click(toggle)

      await waitFor(() => expect(screen.getByText(/Cerberus awakens/i)).toBeInTheDocument())
    })

    it('should show overlay when service is toggling', async () => {
      const user = userEvent.setup()
      vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(mockSecurityStatus)
      vi.mocked(settingsApi.updateSetting).mockImplementation(() => new Promise(() => {}))

      render(<Security />, { wrapper })

      await waitFor(() => screen.getByTestId('toggle-waf'))
      const toggle = screen.getByTestId('toggle-waf')
      await user.click(toggle)

      await waitFor(() => expect(screen.getByText(/Three heads turn/i)).toBeInTheDocument())
    })

    it('should show overlay when starting CrowdSec', async () => {
      const user = userEvent.setup()
      vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(mockSecurityStatus)
      vi.mocked(crowdsecApi.statusCrowdsec).mockResolvedValue({ running: false })
      vi.mocked(crowdsecApi.startCrowdsec).mockImplementation(() => new Promise(() => {}))

      render(<Security />, { wrapper })

      await waitFor(() => screen.getByTestId('crowdsec-start'))
      const startButton = screen.getByTestId('crowdsec-start')
      await user.click(startButton)

      await waitFor(() => expect(screen.getByText(/Summoning the guardian/i)).toBeInTheDocument())
    })

    it('should show overlay when stopping CrowdSec', async () => {
      const user = userEvent.setup()
      vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(mockSecurityStatus)
      vi.mocked(crowdsecApi.statusCrowdsec).mockResolvedValue({ running: true, pid: 1234 })
      vi.mocked(crowdsecApi.stopCrowdsec).mockImplementation(() => new Promise(() => {}))

      render(<Security />, { wrapper })

      await waitFor(() => screen.getByTestId('crowdsec-stop'))
      const stopButton = screen.getByTestId('crowdsec-stop')
      await user.click(stopButton)

      await waitFor(() => expect(screen.getByText(/Guardian rests/i)).toBeInTheDocument())
    })
  })
})
