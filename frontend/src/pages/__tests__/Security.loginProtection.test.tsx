import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { act, render, screen } from '@testing-library/react'
import { BrowserRouter } from 'react-router'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import * as crowdsecApi from '../../api/crowdsec'
import * as securityApi from '../../api/security'
import * as authHook from '../../hooks/useAuth'
import Security from '../Security'

import type { AuthContextType } from '../../context/AuthContextValue'
import type * as useSecurity from '../../hooks/useSecurity'

vi.mock('../../api/security')
vi.mock('../../api/crowdsec')
vi.mock('../../api/settings')
vi.mock('../../hooks/useAuth')
vi.mock('../../hooks/useSecurity', async (importOriginal) => {
  const actual = await importOriginal<typeof useSecurity>()
  return {
    ...actual,
    useSecurityConfig: vi.fn(() => ({ data: { config: { admin_whitelist: '' } } })),
    useUpdateSecurityConfig: vi.fn(() => ({ mutate: vi.fn(), isPending: false })),
    useGenerateBreakGlassToken: vi.fn(() => ({ mutate: vi.fn(), isPending: false })),
  }
})

const loginProtection: securityApi.LoginProtectionStatus = {
  enabled: true,
  login: { requests: 10, window_seconds: 600 },
  session: { requests: 60, window_seconds: 60 },
  trusted_proxy_count: 0,
  caller_client_key: '203.0.113.7',
  caller_client_scope: 'public',
  untrusted_forwarded_headers: {
    local: { count: 0, last_seen: null, last_peer: '', last_peer_scope: '' },
    public: { count: 0, last_seen: null, last_peer: '', last_peer_scope: '' },
  },
}

const renderPage = async (role: 'admin' | 'user' | null) => {
  vi.spyOn(authHook, 'useAuth').mockReturnValue({
    user: role ? { user_id: 1, role } : null,
  } as unknown as AuthContextType)
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  await act(async () => {
    render(
      <QueryClientProvider client={queryClient}>
        <BrowserRouter>
          <Security />
        </BrowserRouter>
      </QueryClientProvider>
    )
  })
}

describe('Security page login protection card', () => {
  beforeEach(() => {
    vi.mocked(securityApi.getSecurityStatus).mockResolvedValue({
      // Cerberus off: the card must still appear because login protection is always on
      cerberus: { enabled: false },
      crowdsec: { mode: 'disabled', api_url: '', enabled: false },
      waf: { mode: 'disabled', enabled: false },
      rate_limit: { enabled: false },
      acl: { enabled: false },
    })
    vi.mocked(securityApi.getLoginProtectionStatus).mockResolvedValue(loginProtection)
    vi.mocked(crowdsecApi.statusCrowdsec).mockResolvedValue({ running: false, pid: 0, lapi_ready: false })
  })

  it('shows the card to admins even when Cerberus is disabled', async () => {
    await renderPage('admin')
    expect(await screen.findByRole('region', { name: /login protection/i })).toBeInTheDocument()
    expect(securityApi.getLoginProtectionStatus).toHaveBeenCalled()
  })

  it.each(['user', null] as const)('hides the card and skips the request for %s', async (role) => {
    await renderPage(role)
    await screen.findByText('Cerberus Dashboard')
    expect(screen.queryByRole('region', { name: /login protection/i })).not.toBeInTheDocument()
    expect(securityApi.getLoginProtectionStatus).not.toHaveBeenCalled()
  })
})
