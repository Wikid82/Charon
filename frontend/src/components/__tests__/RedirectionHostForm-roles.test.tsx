import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, it, expect, vi, beforeEach } from 'vitest'

import RedirectionHostForm from '../RedirectionHostForm'

import type { RedirectionHost } from '../../api/redirectionHosts'

const roleState = vi.hoisted(() => ({ role: 'admin' as 'admin' | 'user' }))

vi.mock('../../hooks/useAuth', () => ({
  useAuth: () => ({ user: { user_id: 1, role: roleState.role } }),
}))

vi.mock('../../hooks/useCertificates', () => ({
  useCertificates: vi.fn(() => ({ certificates: [], isLoading: false, error: null })),
}))

vi.mock('../../hooks/useDNSProviders', () => ({
  useDNSProviders: vi.fn(() => ({
    data: [{ uuid: 'dns-1', name: 'Cloudflare', provider_type: 'cloudflare', enabled: true, has_credentials: true, is_default: true }],
    isLoading: false,
  })),
}))

const renderForm = (ui: React.ReactElement) => {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } })
  return render(<QueryClientProvider client={queryClient}>{ui}</QueryClientProvider>)
}

const DNS_NOTE = 'Only administrators can change the DNS provider and DNS challenge.'

const dnsHost: RedirectionHost = {
  uuid: 'rh-1',
  name: 'Old blog redirect',
  domain_names: '*.old-blog.example.com',
  target_url: 'https://newblog.example.com',
  status_code: 301,
  preserve_path: true,
  ssl_forced: true,
  http2_support: true,
  hsts_enabled: false,
  hsts_subdomains: false,
  enabled: true,
  certificate_id: null,
  certificate: null,
  dns_provider_id: 'dns-1',
  dns_provider: null,
  use_dns_challenge: true,
  created_at: '2026-01-01T00:00:00Z',
  updated_at: '2026-01-01T00:00:00Z',
}

describe('RedirectionHostForm - role-aware DNS fields', () => {
  const onSubmit = vi.fn((_data: Partial<RedirectionHost>) => Promise.resolve())
  const onCancel = vi.fn()

  beforeEach(() => {
    vi.clearAllMocks()
    roleState.role = 'admin'
  })

  it('disables the DNS challenge switch and provider selector for a standard user and explains why', () => {
    roleState.role = 'user'
    renderForm(<RedirectionHostForm host={dnsHost} onSubmit={onSubmit} onCancel={onCancel} />)

    expect(screen.getByLabelText(/use dns challenge/i)).toBeDisabled()
    expect(screen.getByRole('combobox', { name: /DNS Provider/ })).toBeDisabled()
    expect(screen.getByTestId('redirection-dns-admin-only')).toHaveTextContent(DNS_NOTE)
  })

  it('leaves the DNS fields out of a standard user save so the stored binding is kept', async () => {
    roleState.role = 'user'
    renderForm(<RedirectionHostForm host={dnsHost} onSubmit={onSubmit} onCancel={onCancel} />)

    await userEvent.click(screen.getByRole('button', { name: 'Save' }))

    await waitFor(() => expect(onSubmit).toHaveBeenCalledTimes(1))
    const payload = { ...onSubmit.mock.calls[0][0] }
    expect(payload.name).toBe('Old blog redirect')
    expect(payload).not.toHaveProperty('dns_provider_id')
    expect(payload).not.toHaveProperty('use_dns_challenge')
  })

  it('keeps the DNS challenge switch enabled and submits the DNS fields for an administrator', async () => {
    renderForm(<RedirectionHostForm host={dnsHost} onSubmit={onSubmit} onCancel={onCancel} />)

    expect(screen.getByLabelText(/use dns challenge/i)).not.toBeDisabled()
    expect(screen.queryByTestId('redirection-dns-admin-only')).not.toBeInTheDocument()

    await userEvent.click(screen.getByRole('button', { name: 'Save' }))

    await waitFor(() => expect(onSubmit).toHaveBeenCalledTimes(1))
    expect(onSubmit.mock.calls[0][0]).toEqual(
      expect.objectContaining({ dns_provider_id: 'dns-1', use_dns_challenge: true }),
    )
  })
})
