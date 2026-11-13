import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, it, expect, vi, beforeEach } from 'vitest'

import { mockRemoteServers } from '../../test/mockData'
import ProxyHostForm from '../ProxyHostForm'

import type { ProxyHost } from '../../api/proxyHosts'

const roleState = vi.hoisted(() => ({ role: 'admin' as 'admin' | 'user' }))
const detectSpy = vi.hoisted(() => vi.fn())

vi.mock('../../hooks/useAuth', () => ({
  useAuth: () => ({ user: { user_id: 1, role: roleState.role } }),
}))

vi.mock('../../hooks/useRemoteServers', () => ({
  useRemoteServers: vi.fn(() => ({ servers: mockRemoteServers, isLoading: false, error: null })),
}))

vi.mock('../../hooks/useDocker', () => ({
  useDocker: vi.fn(() => ({ containers: [], isLoading: false, error: null, refetch: vi.fn() })),
}))

vi.mock('../../hooks/useDomains', () => ({
  useDomains: vi.fn(() => ({
    domains: [],
    createDomain: vi.fn().mockResolvedValue({ uuid: 'domain-1', name: 'example.com' }),
    isLoading: false,
    error: null,
  })),
}))

vi.mock('../../hooks/useCertificates', () => ({
  useCertificates: vi.fn(() => ({ certificates: [], isLoading: false, error: null })),
}))

vi.mock('../../hooks/useSecurity', () => ({
  useAuthPolicies: vi.fn(() => ({ policies: [], isLoading: false, error: null })),
}))

vi.mock('../../hooks/useSecurityHeaders', () => ({
  useSecurityHeaderProfiles: vi.fn(() => ({ data: [], isLoading: false, error: null })),
}))

vi.mock('../../hooks/useAccessLists', () => ({
  useAccessLists: vi.fn(() => ({ data: [], isLoading: false, error: null })),
}))

vi.mock('../../hooks/useProxyGroups', () => ({
  useProxyGroups: vi.fn(() => ({ data: [], isLoading: false, error: null })),
}))

vi.mock('../../hooks/useDNSProviders', () => ({
  useDNSProviders: vi.fn(() => ({
    data: [
      {
        uuid: 'dns-uuid-1',
        name: 'Cloudflare',
        provider_type: 'cloudflare',
        enabled: true,
        is_default: true,
        has_credentials: true,
        propagation_timeout: 120,
        polling_interval: 2,
        success_count: 0,
        failure_count: 0,
        created_at: '2025-01-01T00:00:00Z',
        updated_at: '2025-01-01T00:00:00Z',
      },
    ],
    isLoading: false,
    isError: false,
  })),
}))

vi.mock('../../hooks/useDNSDetection', () => ({
  useDetectDNSProvider: vi.fn(() => ({
    mutateAsync: detectSpy,
    isPending: false,
    data: undefined,
    reset: vi.fn(),
  })),
}))

vi.mock('../../api/proxyHosts', () => ({
  testProxyHostConnection: vi.fn(),
}))

vi.mock('react-hot-toast', () => ({
  toast: { success: vi.fn(), error: vi.fn() },
}))

const syncSpy = vi.hoisted(() => vi.fn())

vi.mock('../../api/uptime', () => ({
  syncMonitors: syncSpy,
}))

const mockFetch = vi.fn()
vi.stubGlobal('fetch', mockFetch)

const renderForm = (ui: React.ReactElement) => {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(<QueryClientProvider client={queryClient}>{ui}</QueryClientProvider>)
}

// The server redacts advanced_config for standard users and omits it, so the edited host carries has_advanced_config only.
const storedHost: ProxyHost = {
  uuid: 'host-1',
  name: 'Media',
  domain_names: 'media.example.com',
  forward_scheme: 'http',
  forward_host: '10.0.0.5',
  forward_port: 8096,
  ssl_forced: true,
  http2_support: true,
  hsts_enabled: true,
  hsts_subdomains: true,
  block_exploits: true,
  websocket_support: true,
  application: 'none',
  locations: [],
  enabled: true,
  dns_provider_id: 'dns-uuid-1',
  created_at: '2025-01-01T00:00:00Z',
  updated_at: '2025-01-01T00:00:00Z',
}

const adminHost: ProxyHost = { ...storedHost, advanced_config: '{"handler":"headers"}', has_advanced_config: true }
const redactedHost: ProxyHost = { ...storedHost, has_advanced_config: true }

const WILDCARD_NOTE = 'Only administrators can change the DNS provider and DNS challenge.'

describe('ProxyHostForm - role-aware fields', () => {
  const onSubmit = vi.fn((_data: Partial<ProxyHost>) => Promise.resolve())
  const onCancel = vi.fn()

  beforeEach(() => {
    vi.clearAllMocks()
    roleState.role = 'admin'
    detectSpy.mockResolvedValue({ domain: 'example.com', detected: false, nameservers: [], confidence: 'none' })
    mockFetch.mockResolvedValue({ json: () => Promise.resolve({ internal_ip: '192.168.1.50' }) })
  })

  describe('as a standard user', () => {
    beforeEach(() => {
      roleState.role = 'user'
    })

    it('disables the DNS provider selector and explains why for a wildcard host', async () => {
      renderForm(<ProxyHostForm host={redactedHost} onSubmit={onSubmit} onCancel={onCancel} />)

      await userEvent.clear(screen.getByPlaceholderText('example.com, www.example.com'))
      await userEvent.type(screen.getByPlaceholderText('example.com, www.example.com'), '*.example.com')

      const section = await screen.findByTestId('dns-provider-section')
      expect(within(section).getByRole('combobox')).toBeDisabled()
      expect(screen.getByText(WILDCARD_NOTE)).toBeInTheDocument()
    })

    it('does not start DNS provider detection', async () => {
      renderForm(<ProxyHostForm host={redactedHost} onSubmit={onSubmit} onCancel={onCancel} />)

      await userEvent.clear(screen.getByPlaceholderText('example.com, www.example.com'))
      await userEvent.type(screen.getByPlaceholderText('example.com, www.example.com'), '*.example.com')
      await screen.findByTestId('dns-provider-section')

      await new Promise((resolve) => setTimeout(resolve, 600))
      expect(detectSpy).not.toHaveBeenCalled()
    })

    it('replaces the Advanced Caddy Config editor with a hidden-for-role note', () => {
      renderForm(<ProxyHostForm host={redactedHost} onSubmit={onSubmit} onCancel={onCancel} />)

      expect(screen.queryByRole('textbox', { name: /advanced caddy config/i })).not.toBeInTheDocument()
      expect(screen.getByTestId('advanced-config-hidden')).toHaveTextContent('Hidden for your role.')
    })

    it('leaves the DNS provider and advanced config out of an edit that changes other fields', async () => {
      renderForm(<ProxyHostForm host={redactedHost} onSubmit={onSubmit} onCancel={onCancel} />)

      const nameInput = screen.getByDisplayValue('Media')
      await userEvent.clear(nameInput)
      await userEvent.type(nameInput, 'Media Renamed')
      await userEvent.click(screen.getByTestId('proxy-host-save'))

      await waitFor(() => expect(onSubmit).toHaveBeenCalledTimes(1))
      const payload = { ...onSubmit.mock.calls[0][0] }
      expect(payload.name).toBe('Media Renamed')
      expect(payload).not.toHaveProperty('dns_provider_id')
      expect(payload).not.toHaveProperty('advanced_config')
    })

    it('applies a preset as application and WebSocket flag only, never as advanced config', async () => {
      renderForm(<ProxyHostForm host={{ ...redactedHost, websocket_support: false }} onSubmit={onSubmit} onCancel={onCancel} />)

      await userEvent.click(screen.getByRole('combobox', { name: /Application Preset/i }))
      await userEvent.click(await screen.findByRole('option', { name: 'Plex - Media server with remote access' }))
      await userEvent.click(screen.getByTestId('proxy-host-save'))

      await waitFor(() => expect(onSubmit).toHaveBeenCalledTimes(1))
      const payload = { ...onSubmit.mock.calls[0][0] }
      expect(payload.application).toBe('plex')
      expect(payload.websocket_support).toBe(true)
      expect(payload).not.toHaveProperty('advanced_config')
    })
  })

  describe('as an administrator', () => {
    beforeEach(() => {
      roleState.role = 'admin'
    })

    it('keeps the DNS provider selector enabled and shows the advanced config editor', async () => {
      renderForm(<ProxyHostForm host={adminHost} onSubmit={onSubmit} onCancel={onCancel} />)

      expect(screen.getByRole('textbox', { name: /advanced caddy config/i })).toBeInTheDocument()
      expect(screen.queryByTestId('advanced-config-hidden')).not.toBeInTheDocument()

      await userEvent.clear(screen.getByPlaceholderText('example.com, www.example.com'))
      await userEvent.type(screen.getByPlaceholderText('example.com, www.example.com'), '*.example.com')
      const section = await screen.findByTestId('dns-provider-section')
      expect(within(section).getByRole('combobox')).toBeEnabled()
      expect(screen.queryByText(WILDCARD_NOTE)).not.toBeInTheDocument()
    })

    it('submits the DNS provider and advanced config as stored', async () => {
      renderForm(<ProxyHostForm host={adminHost} onSubmit={onSubmit} onCancel={onCancel} />)

      await userEvent.click(screen.getByTestId('proxy-host-save'))

      await waitFor(() => expect(onSubmit).toHaveBeenCalledTimes(1))
      const payload = { ...onSubmit.mock.calls[0][0] }
      expect(payload.dns_provider_id).toBe('dns-uuid-1')
      expect(payload.advanced_config).toBe('{"handler":"headers"}')
    })
  })

  describe('uptime monitoring option', () => {
    const UPTIME_LABEL = 'Add Uptime monitoring for this host'

    beforeEach(() => {
      syncSpy.mockResolvedValue({ message: 'synced' })
    })

    it('hides the uptime option from a standard user and never requests a monitor sync on save', async () => {
      roleState.role = 'user'
      renderForm(<ProxyHostForm host={redactedHost} onSubmit={onSubmit} onCancel={onCancel} />)

      expect(screen.queryByLabelText(UPTIME_LABEL)).not.toBeInTheDocument()

      await userEvent.click(screen.getByTestId('proxy-host-save'))

      await waitFor(() => expect(onSubmit).toHaveBeenCalledTimes(1))
      expect(syncSpy).not.toHaveBeenCalled()
    })

    it('shows the uptime option to an administrator and requests a monitor sync when it is checked', async () => {
      roleState.role = 'admin'
      renderForm(<ProxyHostForm host={adminHost} onSubmit={onSubmit} onCancel={onCancel} />)

      await userEvent.click(screen.getByLabelText(UPTIME_LABEL))
      await userEvent.click(screen.getByTestId('proxy-host-save'))

      await waitFor(() => expect(syncSpy).toHaveBeenCalledTimes(1))
      expect(syncSpy).toHaveBeenCalledWith({ interval: 60, max_retries: 3 })
    })
  })
})
