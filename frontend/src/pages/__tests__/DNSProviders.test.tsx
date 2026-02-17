import { describe, it, expect, vi, beforeEach } from 'vitest'
import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import type { ReactNode } from 'react'
import DNSProviders from '../DNSProviders'
import { renderWithQueryClient } from '../../test-utils/renderWithQueryClient'
import { useDNSProviders, useDNSProviderMutations, type DNSProvider } from '../../hooks/useDNSProviders'
import { getChallenge } from '../../api/manualChallenge'
import { toast } from '../../utils/toast'

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string) => key,
  }),
}))

vi.mock('../../hooks/useDNSProviders', () => ({
  useDNSProviders: vi.fn(),
  useDNSProviderMutations: vi.fn(),
}))

vi.mock('../../api/manualChallenge', () => ({
  getChallenge: vi.fn(),
}))

vi.mock('../../utils/toast', () => ({
  toast: {
    success: vi.fn(),
    error: vi.fn(),
    warning: vi.fn(),
    info: vi.fn(),
  },
}))

vi.mock('../../components/ui', () => ({
  Button: ({ children, onClick, variant }: { children: ReactNode; onClick?: () => void; variant?: string }) => (
    <button type="button" data-variant={variant} onClick={onClick}>
      {children}
    </button>
  ),
  Alert: ({ children }: { children: ReactNode }) => <div role="alert">{children}</div>,
  EmptyState: ({ title }: { title: string }) => <div>{title}</div>,
  Skeleton: () => <div data-testid="skeleton" />,
}))

vi.mock('../../components/DNSProviderCard', () => ({
  default: ({ provider }: { provider: DNSProvider }) => (
    <article data-testid="provider-card">
      <h3>{provider.name}</h3>
    </article>
  ),
}))

vi.mock('../../components/DNSProviderForm', () => ({
  default: () => null,
}))

vi.mock('../../components/dns-providers', () => ({
  ManualDNSChallenge: ({
    challenge,
    onComplete,
    onCancel,
  }: {
    challenge: { fqdn: string }
    onComplete: () => void
    onCancel: () => void
  }) => (
    <section data-testid="manual-dns-challenge">
      <div>{challenge.fqdn}</div>
      <button type="button" onClick={onComplete}>complete-manual</button>
      <button type="button" onClick={onCancel}>cancel-manual</button>
    </section>
  ),
}))

const buildProvider = (overrides: Partial<DNSProvider> = {}): DNSProvider => ({
  id: 7,
  uuid: 'provider-uuid',
  name: 'Seeded Provider',
  provider_type: 'manual',
  enabled: true,
  is_default: false,
  has_credentials: true,
  propagation_timeout: 60,
  polling_interval: 5,
  success_count: 0,
  failure_count: 0,
  created_at: '2026-02-15T00:00:00Z',
  updated_at: '2026-02-15T00:00:00Z',
  ...overrides,
})

describe('DNSProviders page state behavior', () => {
  beforeEach(() => {
    vi.clearAllMocks()

    vi.mocked(useDNSProviderMutations).mockReturnValue({
      deleteMutation: { mutateAsync: vi.fn() },
      testMutation: { mutateAsync: vi.fn() },
      createMutation: { mutateAsync: vi.fn() },
      updateMutation: { mutateAsync: vi.fn() },
      testCredentialsMutation: { mutateAsync: vi.fn() },
    } as unknown as ReturnType<typeof useDNSProviderMutations>)

    vi.mocked(useDNSProviders).mockReturnValue({
      data: [buildProvider()],
      isLoading: false,
      refetch: vi.fn(),
    } as unknown as ReturnType<typeof useDNSProviders>)
  })

  it('keeps provider cards visible by default without challenge fetch side effects', async () => {
    renderWithQueryClient(<DNSProviders />)

    expect(await screen.findByRole('heading', { name: 'Seeded Provider' })).toBeInTheDocument()
    expect(getChallenge).not.toHaveBeenCalled()
    expect(screen.queryByTestId('manual-dns-challenge')).not.toBeInTheDocument()
  })

  it('does not hide provider cards when manual challenge fetch fails', async () => {
    vi.mocked(getChallenge).mockRejectedValue(new Error('not found'))

    const user = userEvent.setup()
    renderWithQueryClient(<DNSProviders />)

    await user.click(screen.getByRole('button', { name: 'dnsProvider.manual.title' }))

    await waitFor(() => {
      expect(getChallenge).toHaveBeenCalledWith(7, 'active')
    })

    expect(screen.queryByTestId('manual-dns-challenge')).not.toBeInTheDocument()
    expect(screen.getByRole('heading', { name: 'Seeded Provider' })).toBeInTheDocument()
  })

  it('opens manual challenge panel only after explicit action when fetch succeeds', async () => {
    vi.mocked(getChallenge).mockResolvedValue({
      id: 'active',
      status: 'pending',
      fqdn: '_acme-challenge.example.com',
      value: 'token',
      ttl: 300,
      created_at: '2026-02-15T00:00:00Z',
      expires_at: '2026-02-15T00:10:00Z',
      dns_propagated: false,
    })

    const user = userEvent.setup()
    renderWithQueryClient(<DNSProviders />)

    expect(screen.queryByTestId('manual-dns-challenge')).not.toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: 'dnsProvider.manual.title' }))

    expect(await screen.findByTestId('manual-dns-challenge')).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: 'complete-manual' }))
    await waitFor(() => {
      expect(getChallenge).toHaveBeenCalledTimes(2)
    })

    await user.click(screen.getByRole('button', { name: 'cancel-manual' }))
    await waitFor(() => {
      expect(screen.queryByTestId('manual-dns-challenge')).not.toBeInTheDocument()
    })
  })

  it('re-evaluates manual challenge visibility after completion refresh', async () => {
    vi.mocked(getChallenge)
      .mockResolvedValueOnce({
        id: 'active',
        status: 'pending',
        fqdn: '_acme-challenge.example.com',
        value: 'token',
        ttl: 300,
        created_at: '2026-02-15T00:00:00Z',
        expires_at: '2026-02-15T00:10:00Z',
        dns_propagated: false,
      })
      .mockRejectedValueOnce(new Error('challenge missing after refresh'))

    const user = userEvent.setup()
    renderWithQueryClient(<DNSProviders />)

    await user.click(screen.getByRole('button', { name: 'dnsProvider.manual.title' }))
    expect(await screen.findByTestId('manual-dns-challenge')).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: 'complete-manual' }))

    await waitFor(() => {
      expect(getChallenge).toHaveBeenCalledTimes(2)
      expect(screen.queryByTestId('manual-dns-challenge')).not.toBeInTheDocument()
    })
  })

  it('shows no provider toast when manual challenge is requested without providers', async () => {
    vi.mocked(useDNSProviders).mockReturnValue({
      data: [],
      isLoading: false,
      refetch: vi.fn(),
    } as unknown as ReturnType<typeof useDNSProviders>)

    const user = userEvent.setup()
    renderWithQueryClient(<DNSProviders />)

    await user.click(screen.getByRole('button', { name: 'dnsProvider.manual.title' }))

    expect(toast.error).toHaveBeenCalledWith('dnsProviders.noProviders')
    expect(getChallenge).not.toHaveBeenCalled()
  })
})
