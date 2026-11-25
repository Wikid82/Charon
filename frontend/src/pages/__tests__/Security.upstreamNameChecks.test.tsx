import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen } from '@testing-library/react'
import { BrowserRouter } from 'react-router'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import * as crowdsecApi from '../../api/crowdsec'
import * as securityApi from '../../api/security'
import * as authHook from '../../hooks/useAuth'
import Security from '../Security'

import type { SecurityStatus } from '../../api/security'
import type { AuthContextType } from '../../context/AuthContextValue'

vi.mock('../../api/security')
vi.mock('../../api/crowdsec')
vi.mock('../../api/settings')
vi.mock('../../hooks/useAuth')

const baseStatus: SecurityStatus = {
  cerberus: { enabled: true },
  crowdsec: { mode: 'disabled', api_url: '', enabled: false },
  waf: { mode: 'disabled', enabled: false },
  rate_limit: { enabled: false },
  acl: { enabled: false },
}

const NOTE_TEXT = 'Upstream name checks are not running. Forward target names are checked when a host is saved.'

const renderPage = () => {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  vi.spyOn(authHook, 'useAuth').mockReturnValue({ user: { user_id: 1, role: 'admin' } } as unknown as AuthContextType)
  render(
    <QueryClientProvider client={queryClient}>
      <BrowserRouter>
        <Security />
      </BrowserRouter>
    </QueryClientProvider>
  )
}

describe('Security page upstream name checks line', () => {
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

  it('shows the muted line when the resolver is not available', async () => {
    vi.mocked(securityApi.getSecurityStatus).mockResolvedValue({
      ...baseStatus,
      upstream_name_checks: { available: false },
    })
    renderPage()

    const note = await screen.findByTestId('upstream-name-checks-note')
    expect(note).toHaveTextContent(NOTE_TEXT)
    expect(note).toHaveClass('text-content-muted')
  })

  it('shows nothing when the resolver is available', async () => {
    vi.mocked(securityApi.getSecurityStatus).mockResolvedValue({
      ...baseStatus,
      upstream_name_checks: { available: true },
    })
    renderPage()

    await screen.findByText('Cerberus Dashboard')
    expect(screen.queryByTestId('upstream-name-checks-note')).not.toBeInTheDocument()
    expect(screen.queryByText(NOTE_TEXT)).not.toBeInTheDocument()
  })

  it('shows nothing when the backend omits the field', async () => {
    vi.mocked(securityApi.getSecurityStatus).mockResolvedValue({ ...baseStatus })
    renderPage()

    await screen.findByText('Cerberus Dashboard')
    expect(screen.queryByTestId('upstream-name-checks-note')).not.toBeInTheDocument()
    expect(screen.queryByText(NOTE_TEXT)).not.toBeInTheDocument()
  })
})
