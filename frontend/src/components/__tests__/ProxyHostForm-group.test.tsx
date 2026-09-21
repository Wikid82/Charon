import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, it, expect, vi, beforeEach } from 'vitest'

import { useProxyGroups } from '../../hooks/useProxyGroups'
import ProxyHostForm from '../ProxyHostForm'

import type { ProxyGroup } from '../../api/proxyGroups'
import type { ProxyHost } from '../../api/proxyHosts'

// Mock all hooks ProxyHostForm depends on so only the group-selector behavior
// under test is exercised.
vi.mock('../../hooks/useRemoteServers', () => ({
  useRemoteServers: vi.fn(() => ({
    servers: [],
    isLoading: false,
    error: null,
  })),
}))

vi.mock('../../hooks/useDocker', () => ({
  useDocker: vi.fn(() => ({
    containers: [],
    isLoading: false,
    error: null,
    refetch: vi.fn(),
  })),
}))

vi.mock('../../hooks/useDomains', () => ({
  useDomains: vi.fn(() => ({
    domains: [{ uuid: 'domain-1', name: 'test.com' }],
    createDomain: vi.fn().mockResolvedValue({}),
    isLoading: false,
    error: null,
  })),
}))

vi.mock('../../hooks/useCertificates', () => ({
  useCertificates: vi.fn(() => ({
    certificates: [],
    isLoading: false,
    error: null,
  })),
}))

vi.mock('../../hooks/useSecurityHeaders', () => ({
  useSecurityHeaderProfiles: vi.fn(() => ({
    data: [],
    isLoading: false,
    error: null,
  })),
}))

vi.mock('../../hooks/useAccessLists', () => ({
  useAccessLists: vi.fn(() => ({
    data: [],
    isLoading: false,
    error: null,
  })),
}))

vi.mock('../../hooks/useDNSDetection', () => ({
  useDetectDNSProvider: vi.fn(() => ({
    mutateAsync: vi.fn(),
    isPending: false,
    data: undefined,
    reset: vi.fn(),
  })),
}))

vi.mock('../../hooks/useProxyGroups')

// Mock fetch for the internal-IP health endpoint
vi.stubGlobal('fetch', vi.fn(() => Promise.resolve({
  json: () => Promise.resolve({ internal_ip: '127.0.0.1' }),
})))

const mockGroups: ProxyGroup[] = [
  {
    uuid: 'group-uuid-1',
    name: 'Media',
    description: 'Media services',
    color: '#6366f1',
    host_count: 3,
    created_at: '2024-01-01',
    updated_at: '2024-01-01',
  },
  {
    uuid: 'group-uuid-2',
    name: 'Internal',
    description: 'Internal tools',
    color: '#22c55e',
    host_count: 1,
    created_at: '2024-01-01',
    updated_at: '2024-01-01',
  },
]

const baseHost: ProxyHost = {
  uuid: 'host-uuid-1',
  name: 'Existing Service',
  domain_names: 'existing.com',
  forward_scheme: 'https',
  forward_host: '192.168.1.50',
  forward_port: 443,
  ssl_forced: true,
  http2_support: true,
  hsts_enabled: true,
  hsts_subdomains: true,
  block_exploits: true,
  websocket_support: false,
  enable_standard_headers: true,
  application: 'none',
  advanced_config: '',
  enabled: true,
  locations: [],
  certificate_id: null,
  access_list_id: null,
  security_header_profile_id: null,
  dns_provider_id: null,
  created_at: '2025-01-01T00:00:00Z',
  updated_at: '2025-01-01T00:00:00Z',
}

const createWrapper = () => {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false },
    },
  })
  return ({ children }: { children: React.ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  )
}

async function fillRequiredFields(user: ReturnType<typeof userEvent.setup>) {
  await user.type(screen.getByLabelText(/^Name/), 'Test Service')
  await user.type(screen.getByLabelText(/Domain Names/), 'test.com')
  await user.type(screen.getByLabelText(/^Host$/), 'localhost')
  await user.clear(screen.getByLabelText(/^Port$/))
  await user.type(screen.getByLabelText(/^Port$/), '8080')
}

describe('ProxyHostForm Proxy Group Selector', () => {
  let mockOnSubmit: (data: Partial<ProxyHost>) => Promise<void>
  let mockOnCancel: () => void

  beforeEach(() => {
    mockOnSubmit = vi.fn<(data: Partial<ProxyHost>) => Promise<void>>(() => Promise.resolve())
    mockOnCancel = vi.fn<() => void>()

    vi.mocked(useProxyGroups).mockReturnValue({
      data: mockGroups,
      isLoading: false,
      error: null,
    } as unknown as ReturnType<typeof useProxyGroups>)
  })

  it('renders the group selector with no group preselected for a new host', () => {
    const Wrapper = createWrapper()

    render(
      <Wrapper>
        <ProxyHostForm onSubmit={mockOnSubmit} onCancel={mockOnCancel} />
      </Wrapper>
    )

    const trigger = screen.getByRole('combobox', { name: /Proxy Group/i })
    expect(trigger).toBeInTheDocument()
    expect(trigger).toHaveTextContent('No Group')
  })

  it('preselects the current group when editing a host with a proxy_group', () => {
    const Wrapper = createWrapper()
    const hostWithGroup: ProxyHost = {
      ...baseHost,
      proxy_group: { uuid: 'group-uuid-2', name: 'Internal', color: '#22c55e' },
    }

    render(
      <Wrapper>
        <ProxyHostForm host={hostWithGroup} onSubmit={mockOnSubmit} onCancel={mockOnCancel} />
      </Wrapper>
    )

    expect(screen.getByRole('combobox', { name: /Proxy Group/i })).toHaveTextContent('Internal')
  })

  it('submits proxy_group_id for the selected group on a new host', async () => {
    const user = userEvent.setup()
    const Wrapper = createWrapper()

    render(
      <Wrapper>
        <ProxyHostForm onSubmit={mockOnSubmit} onCancel={mockOnCancel} />
      </Wrapper>
    )

    await fillRequiredFields(user)

    await user.click(screen.getByRole('combobox', { name: /Proxy Group/i }))
    await user.click(await screen.findByRole('option', { name: 'Media' }))

    expect(screen.getByRole('combobox', { name: /Proxy Group/i })).toHaveTextContent('Media')

    await user.click(screen.getByRole('button', { name: /Save/i }))

    await waitFor(() => {
      expect(mockOnSubmit).toHaveBeenCalledWith(
        expect.objectContaining({ proxy_group_id: 'group-uuid-1' })
      )
    })
  })

  it('clears the group (sentinel) and submits proxy_group_id null', async () => {
    const user = userEvent.setup()
    const Wrapper = createWrapper()
    const hostWithGroup: ProxyHost = {
      ...baseHost,
      proxy_group: { uuid: 'group-uuid-1', name: 'Media', color: '#6366f1' },
    }

    render(
      <Wrapper>
        <ProxyHostForm host={hostWithGroup} onSubmit={mockOnSubmit} onCancel={mockOnCancel} />
      </Wrapper>
    )

    expect(screen.getByRole('combobox', { name: /Proxy Group/i })).toHaveTextContent('Media')

    await user.click(screen.getByRole('combobox', { name: /Proxy Group/i }))
    await user.click(await screen.findByRole('option', { name: /No Group/i }))

    expect(screen.getByRole('combobox', { name: /Proxy Group/i })).toHaveTextContent('No Group')

    await user.click(screen.getByRole('button', { name: /Save/i }))

    await waitFor(() => {
      expect(mockOnSubmit).toHaveBeenCalledWith(
        expect.objectContaining({ proxy_group_id: null })
      )
    })
  })

  it('does not regress other fields when the group is left untouched on submit', async () => {
    const user = userEvent.setup()
    const Wrapper = createWrapper()

    render(
      <Wrapper>
        <ProxyHostForm onSubmit={mockOnSubmit} onCancel={mockOnCancel} />
      </Wrapper>
    )

    await fillRequiredFields(user)

    await user.click(screen.getByRole('button', { name: /Save/i }))

    await waitFor(() => {
      expect(mockOnSubmit).toHaveBeenCalledWith(
        expect.objectContaining({
          name: 'Test Service',
          domain_names: 'test.com',
          forward_host: 'localhost',
          forward_port: 8080,
          proxy_group_id: null,
        })
      )
    })
  })
})
