import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { act, render, screen } from '@testing-library/react'
import { BrowserRouter } from 'react-router'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import * as crowdsecApi from '../../api/crowdsec'
import * as securityApi from '../../api/security'
import * as authHook from '../../hooks/useAuth'
import { SECURITY_STATUS_QUERY_KEY } from '../../hooks/useSecurity'
import Security from '../Security'

import type { SecurityStatus } from '../../api/security'
import type { AuthContextType } from '../../context/AuthContextValue'

vi.mock('../../api/security')
vi.mock('../../api/crowdsec')
vi.mock('../../api/settings')
vi.mock('../../hooks/useAuth')

const statusWith = (overrides: {
  waf?: Partial<SecurityStatus['waf']>
  rate_limit?: Partial<SecurityStatus['rate_limit']>
}): SecurityStatus => ({
  cerberus: { enabled: true },
  crowdsec: { mode: 'disabled', api_url: '', enabled: false },
  waf: { mode: 'enabled', enabled: true, ...overrides.waf },
  rate_limit: { enabled: true, ...overrides.rate_limit },
  acl: { enabled: false },
})

const renderPage = async (queryClient: QueryClient) => {
  vi.spyOn(authHook, 'useAuth').mockReturnValue({ user: { user_id: 1, role: 'admin' } } as unknown as AuthContextType)
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

const newClient = () => new QueryClient({ defaultOptions: { queries: { retry: false } } })

describe('Security page enforcement lines', () => {
  beforeEach(() => {
    vi.resetAllMocks()
    vi.mocked(securityApi.getSecurityConfig).mockResolvedValue({ config: { admin_whitelist: '' } })
    vi.mocked(securityApi.getLoginProtectionStatus).mockResolvedValue({
      enabled: false,
      login: { requests: 10, window_seconds: 600 },
      session: { requests: 60, window_seconds: 60 },
      trusted_proxy_count: 0,
      caller_client_key: '',
      caller_client_scope: 'public',
      untrusted_forwarded_headers: {
        local: { count: 0, last_seen: null, last_peer: '', last_peer_scope: '' },
        public: { count: 0, last_seen: null, last_peer: '', last_peer_scope: '' },
      },
    })
    vi.mocked(crowdsecApi.statusCrowdsec).mockResolvedValue({ running: false, pid: 0, lapi_ready: false })
  })

  it('shows the WAF line for a reason when the WAF is on but not applied', async () => {
    vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(
      statusWith({ waf: { enforced: false, reason: 'no_ruleset' } })
    )
    await renderPage(newClient())

    expect(await screen.findByTestId('waf-enforcement-note')).toHaveTextContent(
      'no rule set is available to inspect requests'
    )
  })

  it('shows the rate-limit line when no thresholds are set', async () => {
    vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(
      statusWith({ rate_limit: { enforced: false, reason: 'thresholds_unset' } })
    )
    await renderPage(newClient())

    expect(await screen.findByTestId('rate_limit-enforcement-note')).toHaveTextContent(
      'no limits are set yet'
    )
  })

  it('omits both lines when enforced is absent', async () => {
    vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(statusWith({}))
    await renderPage(newClient())

    await screen.findByText('Coraza WAF')
    expect(screen.queryByTestId('waf-enforcement-note')).not.toBeInTheDocument()
    expect(screen.queryByTestId('rate_limit-enforcement-note')).not.toBeInTheDocument()
  })

  it('omits both lines when the module is applied', async () => {
    vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(
      statusWith({ waf: { enforced: true }, rate_limit: { enforced: true } })
    )
    await renderPage(newClient())

    await screen.findByText('Coraza WAF')
    expect(screen.queryByTestId('waf-enforcement-note')).not.toBeInTheDocument()
    expect(screen.queryByTestId('rate_limit-enforcement-note')).not.toBeInTheDocument()
  })

  it('refetches the status when the shared security status key is invalidated', async () => {
    vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(
      statusWith({ waf: { enforced: true } })
    )
    const queryClient = newClient()
    await renderPage(queryClient)
    await screen.findByText('Coraza WAF')
    expect(securityApi.getSecurityStatus).toHaveBeenCalledTimes(1)

    // The WAF page invalidates the shared key after a rule-set source save.
    vi.mocked(securityApi.getSecurityStatus).mockResolvedValue(
      statusWith({ waf: { enforced: false, reason: 'ruleset_not_selected' } })
    )
    await act(async () => {
      await queryClient.invalidateQueries({ queryKey: SECURITY_STATUS_QUERY_KEY })
    })

    expect(await screen.findByTestId('waf-enforcement-note')).toBeInTheDocument()
    expect(securityApi.getSecurityStatus).toHaveBeenCalledTimes(2)
  })
})
