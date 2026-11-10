import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor } from '@testing-library/react'
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

const renderPage = (role: 'admin' | 'user' | null) => {
  vi.spyOn(authHook, 'useAuth').mockReturnValue({
    user: role ? { user_id: 1, role } : null,
  } as unknown as AuthContextType)
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  render(
    <QueryClientProvider client={queryClient}>
      <BrowserRouter>
        <Security />
      </BrowserRouter>
    </QueryClientProvider>
  )
}

describe('Security page GeoIP database card', () => {
  beforeEach(() => {
    vi.resetAllMocks()
    vi.mocked(securityApi.getSecurityStatus).mockResolvedValue({
      cerberus: { enabled: false },
      crowdsec: { mode: 'disabled', api_url: '', enabled: false },
      waf: { mode: 'disabled', enabled: false },
      rate_limit: { enabled: false },
      acl: { enabled: false },
    })
    vi.mocked(securityApi.getLoginProtectionStatus).mockResolvedValue({
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
    })
    vi.mocked(securityApi.getGeoIPStatus).mockResolvedValue({
      available: true,
      loaded: true,
      db_path: '/app/geoip/GeoLite2-Country.mmdb',
      message: 'GeoIP service available',
      source: 'bundled',
      database_type: 'GeoLite2-Country',
      build_date: null,
      size_bytes: null,
      loaded_at: null,
      included_build_date: null,
      data_copy_present: false,
      can_replace: true,
      replace_blocked_reason: '',
      unenforced_lists_total: 0,
    })
    vi.mocked(crowdsecApi.statusCrowdsec).mockResolvedValue({ running: false, pid: 0, lapi_ready: false })
  })

  it('shows the card to admins', async () => {
    renderPage('admin')
    expect(await screen.findByRole('region', { name: 'Country database (GeoIP)' })).toBeInTheDocument()
    await waitFor(() => expect(securityApi.getGeoIPStatus).toHaveBeenCalled())
  })

  it.each(['user', null] as const)('hides the card and skips the status request for %s', async (role) => {
    renderPage(role)
    await waitFor(() => expect(securityApi.getSecurityStatus).toHaveBeenCalled())
    expect(screen.queryByRole('region', { name: 'Country database (GeoIP)' })).not.toBeInTheDocument()
    expect(securityApi.getGeoIPStatus).not.toHaveBeenCalled()
  })
})
