import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen } from '@testing-library/react'
import { BrowserRouter } from 'react-router'
import { describe, it, expect, vi, beforeEach } from 'vitest'

import * as crowdsecApi from '../../api/crowdsec'
import * as securityApi from '../../api/security'
import { useAuth } from '../../hooks/useAuth'
import * as useSecurity from '../../hooks/useSecurity'
import Security from '../Security'

import type { SecurityConfigRecord } from '../../api/security'

vi.mock('../../hooks/useAuth', () => ({ useAuth: vi.fn() }))
vi.mock('../../api/security')
vi.mock('../../api/crowdsec')
vi.mock('../../api/settings')
vi.mock('../../hooks/useSecurity', async (importOriginal) => {
  const actual = await importOriginal<typeof useSecurity>()
  return {
    ...actual,
    useSecurityConfig: vi.fn(),
    useUpdateSecurityConfig: vi.fn(() => ({ mutate: vi.fn(), isPending: false })),
    useGenerateBreakGlassToken: vi.fn(() => ({ mutate: vi.fn(), isPending: false })),
    useRuleSets: vi.fn(() => ({ data: { rulesets: [] } })),
  }
})

const ALLOW_LIST_PLACEHOLDER = '192.168.1.0/24, 10.0.0.1'

const securityStatus = {
  cerberus: { enabled: true },
  crowdsec: { mode: 'local' as const, api_url: 'http://localhost', enabled: true },
  waf: { mode: 'enabled' as const, enabled: true },
  rate_limit: { enabled: true },
  acl: { enabled: true },
}

const setRole = (role: 'admin' | 'user') => {
  vi.mocked(useAuth).mockReturnValue({ user: { role } } as unknown as ReturnType<typeof useAuth>)
}

const setConfig = (config: SecurityConfigRecord) => {
  vi.mocked(useSecurity.useSecurityConfig).mockReturnValue({ data: { config } } as unknown as ReturnType<typeof useSecurity.useSecurityConfig>)
}

const renderPage = () => {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } })
  return render(
    <QueryClientProvider client={queryClient}>
      <BrowserRouter>
        <Security />
      </BrowserRouter>
    </QueryClientProvider>,
  )
}

describe('Security page allow-list for standard users', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(securityStatus)
    vi.mocked(crowdsecApi.statusCrowdsec).mockResolvedValue({ running: false, pid: 0, lapi_ready: false })
  })

  it('shows a read-only hidden note in the allow-list when the server holds one', async () => {
    setRole('user')
    setConfig({ has_admin_whitelist: true })

    renderPage()

    const input = await screen.findByLabelText('Comma-separated CIDR/IPs')
    expect(input).toHaveAttribute('readonly')
    expect(input).toHaveAttribute('placeholder', 'Hidden for your role.')
    expect(input).toHaveValue('')
    expect(screen.getByRole('button', { name: 'Save' })).toBeDisabled()
  })

  it('keeps the example placeholder and an editable field when no allow-list is stored', async () => {
    setRole('user')
    setConfig({ has_admin_whitelist: false })

    renderPage()

    const input = await screen.findByLabelText('Comma-separated CIDR/IPs')
    expect(input).not.toHaveAttribute('readonly')
    expect(input).toHaveAttribute('placeholder', ALLOW_LIST_PLACEHOLDER)
    expect(screen.getByRole('button', { name: 'Save' })).toBeEnabled()
  })

  it('lets an administrator edit the allow-list even when the flag is set', async () => {
    setRole('admin')
    setConfig({ admin_whitelist: '10.0.0.0/8', has_admin_whitelist: true })

    renderPage()

    const input = await screen.findByLabelText('Comma-separated CIDR/IPs')
    expect(input).toHaveValue('10.0.0.0/8')
    expect(input).not.toHaveAttribute('readonly')
    expect(input).toHaveAttribute('placeholder', ALLOW_LIST_PLACEHOLDER)
    expect(screen.getByRole('button', { name: 'Save' })).toBeEnabled()
  })
})
