import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, within } from '@testing-library/react'
import { describe, expect, it, vi, beforeEach } from 'vitest'

import * as securityApi from '../../api/security'
import { LoginProtectionCard } from '../LoginProtectionCard'

import type { ForwardedHeaderObservation, LoginProtectionStatus } from '../../api/security'

vi.mock('../../api/security')

const empty: ForwardedHeaderObservation = { count: 0, last_seen: null, last_peer: '', last_peer_scope: '' }
const minutesAgo = (minutes: number) => new Date(Date.now() - minutes * 60_000).toISOString()

const status = (overrides: Partial<LoginProtectionStatus> = {}): LoginProtectionStatus => ({
  enabled: true,
  login: { requests: 10, window_seconds: 600 },
  session: { requests: 60, window_seconds: 60 },
  trusted_proxy_count: 1,
  caller_client_key: '203.0.113.7',
  caller_client_scope: 'public',
  untrusted_forwarded_headers: { local: empty, public: empty },
  ...overrides,
})

const withHeaders = (
  local: Partial<ForwardedHeaderObservation> = {},
  publicObs: Partial<ForwardedHeaderObservation> = {}
): LoginProtectionStatus =>
  status({
    untrusted_forwarded_headers: {
      local: { ...empty, ...local },
      public: { ...empty, ...publicObs },
    },
  })

const renderCard = async () => {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  render(
    <QueryClientProvider client={queryClient}>
      <LoginProtectionCard />
    </QueryClientProvider>
  )
  return screen.findByRole('region', { name: /login protection/i })
}

describe('<LoginProtectionCard />', () => {
  beforeEach(() => {
    vi.mocked(securityApi.getLoginProtectionStatus).mockReset()
  })

  it('shows a skeleton while loading', () => {
    vi.mocked(securityApi.getLoginProtectionStatus).mockReturnValue(new Promise(() => {}))
    const queryClient = new QueryClient()
    render(
      <QueryClientProvider client={queryClient}>
        <LoginProtectionCard />
      </QueryClientProvider>
    )
    expect(screen.getByRole('region', { name: /login protection/i })).toBeInTheDocument()
    expect(screen.getByTestId('login-protection-skeleton')).toBeInTheDocument()
  })

  it('shows the healthy state with budgets, caller address and no warning', async () => {
    vi.mocked(securityApi.getLoginProtectionStatus).mockResolvedValue(status())
    const card = await renderCard()

    expect(await within(card).findByText('On')).toBeInTheDocument()
    expect(card).toHaveTextContent('Sign-in attempts: 10 per 10 minutes')
    expect(card).toHaveTextContent('Session checks: 60 per 1 minute')
    expect(card).toHaveTextContent('Charon sees your browser as 203.0.113.7 (public address).')
    expect(card).toHaveTextContent("If that isn't your device's address")
    expect(card).not.toHaveTextContent('CHARON_TRUSTED_PROXIES')
    expect(within(card).queryByRole('alert')).not.toBeInTheDocument()
    const docs = within(card).getByRole('link', { name: /documentation/i })
    expect(docs).toHaveAttribute('href', 'https://wikid82.github.io/Charon/docs/configuration/trusted-proxies')
    expect(docs).toHaveAttribute('target', '_blank')
    expect(docs).toHaveAttribute('rel', 'noopener noreferrer')
  })

  it('formats sub-minute windows in seconds', async () => {
    vi.mocked(securityApi.getLoginProtectionStatus).mockResolvedValue(
      status({ login: { requests: 5, window_seconds: 45 } })
    )
    const card = await renderCard()
    expect(await within(card).findByText(/Sign-in attempts: 5 per 45 seconds/)).toBeInTheDocument()
  })

  it('warns and suggests a /32 for a recent private IPv4 peer', async () => {
    vi.mocked(securityApi.getLoginProtectionStatus).mockResolvedValue(
      withHeaders({ count: 3, last_seen: minutesAgo(5), last_peer: '172.18.0.5', last_peer_scope: 'private' })
    )
    const card = await renderCard()

    const alert = await within(card).findByRole('alert')
    expect(alert).toHaveTextContent('Forwarded client addresses from 172.18.0.5 are being ignored')
    expect(alert).toHaveTextContent('3 times')
    expect(alert).toHaveTextContent('5 minutes ago')
    expect(alert).toHaveTextContent('add 172.18.0.5/32 to CHARON_TRUSTED_PROXIES')
  })

  it('suggests a /128 for an IPv6 loopback peer', async () => {
    vi.mocked(securityApi.getLoginProtectionStatus).mockResolvedValue(
      withHeaders({ count: 1, last_seen: minutesAgo(120), last_peer: '::1', last_peer_scope: 'loopback' })
    )
    const card = await renderCard()

    const alert = await within(card).findByRole('alert')
    expect(alert).toHaveTextContent('add ::1/128 to CHARON_TRUSTED_PROXIES')
    expect(alert).toHaveTextContent('2 hours ago')
  })

  it('shows seconds for very recent observations', async () => {
    vi.mocked(securityApi.getLoginProtectionStatus).mockResolvedValue(
      withHeaders({ count: 1, last_seen: new Date().toISOString(), last_peer: '10.0.0.2', last_peer_scope: 'private' })
    )
    const card = await renderCard()
    expect(await within(card).findByRole('alert')).toHaveTextContent(/now|seconds? ago/)
  })

  it('shows an informational note without any trust suggestion for a public peer', async () => {
    vi.mocked(securityApi.getLoginProtectionStatus).mockResolvedValue(
      withHeaders({}, { count: 2, last_seen: minutesAgo(3), last_peer: '203.0.113.9', last_peer_scope: 'public' })
    )
    const card = await renderCard()

    const note = await within(card).findByRole('alert')
    expect(note).toHaveTextContent('203.0.113.9')
    expect(note).toHaveTextContent(/no action is needed/i)
    expect(card).not.toHaveTextContent('CHARON_TRUSTED_PROXIES')
    expect(card).not.toHaveTextContent('/32')
    expect(card).not.toHaveTextContent('/128')
  })

  it('keeps the private warning when later public noise arrives, and shows both', async () => {
    vi.mocked(securityApi.getLoginProtectionStatus).mockResolvedValue(
      withHeaders(
        { count: 4, last_seen: minutesAgo(600), last_peer: '172.18.0.5', last_peer_scope: 'private' },
        { count: 9, last_seen: minutesAgo(1), last_peer: '203.0.113.9', last_peer_scope: 'public' }
      )
    )
    const card = await renderCard()

    const alerts = await within(card).findAllByRole('alert')
    expect(alerts).toHaveLength(2)
    expect(alerts[0]).toHaveTextContent('add 172.18.0.5/32 to CHARON_TRUSTED_PROXIES')
    expect(alerts[1]).toHaveTextContent('203.0.113.9')
    expect(alerts[1]).not.toHaveTextContent('CHARON_TRUSTED_PROXIES')
  })

  it('ignores observations older than 24 hours', async () => {
    vi.mocked(securityApi.getLoginProtectionStatus).mockResolvedValue(
      withHeaders(
        { count: 4, last_seen: minutesAgo(25 * 60), last_peer: '172.18.0.5', last_peer_scope: 'private' },
        { count: 2, last_seen: minutesAgo(30 * 60), last_peer: '203.0.113.9', last_peer_scope: 'public' }
      )
    )
    const card = await renderCard()
    expect(await within(card).findByText('On')).toBeInTheDocument()
    expect(within(card).queryByRole('alert')).not.toBeInTheDocument()
  })

  it('ignores observations with an unparseable timestamp', async () => {
    vi.mocked(securityApi.getLoginProtectionStatus).mockResolvedValue(
      withHeaders({ count: 1, last_seen: 'not-a-date', last_peer: '10.0.0.2', last_peer_scope: 'private' })
    )
    const card = await renderCard()
    expect(await within(card).findByText('On')).toBeInTheDocument()
    expect(within(card).queryByRole('alert')).not.toBeInTheDocument()
  })

  it('shows the disabled state with a login-protection docs link', async () => {
    vi.mocked(securityApi.getLoginProtectionStatus).mockResolvedValue(status({ enabled: false }))
    const card = await renderCard()

    expect(await within(card).findByText('Off')).toBeInTheDocument()
    expect(card).toHaveTextContent('Login protection is turned off (CHARON_AUTH_RATELIMIT_ENABLED=false).')
    expect(card).not.toHaveTextContent('Sign-in attempts')
    expect(within(card).getByRole('link', { name: /documentation/i })).toHaveAttribute(
      'href',
      'https://wikid82.github.io/Charon/docs/features/login-protection'
    )
  })

  it('shows an error line when the status cannot be loaded', async () => {
    vi.mocked(securityApi.getLoginProtectionStatus).mockRejectedValue(new Error('403'))
    const card = await renderCard()
    expect(await within(card).findByText('Login protection status could not be loaded.')).toBeInTheDocument()
  })
})
