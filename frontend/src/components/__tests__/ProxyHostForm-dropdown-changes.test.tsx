import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import userEvent from '@testing-library/user-event'
import ProxyHostForm from '../ProxyHostForm'
import type { ProxyHost } from '../../api/proxyHosts'
import type { AccessList } from '../../api/accessLists'
import type { SecurityHeaderProfile } from '../../api/securityHeaders'
import { useAccessLists } from '../../hooks/useAccessLists'
import { useSecurityHeaderProfiles } from '../../hooks/useSecurityHeaders'

// Mock all required hooks
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
    domains: [{ uuid: 'domain-1', name: 'test.com' }], // Add test.com so modal doesn't appear
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

vi.mock('../../hooks/useDNSDetection', () => ({
  useDetectDNSProvider: vi.fn(() => ({
    mutateAsync: vi.fn(),
    isPending: false,
    data: undefined,
    reset: vi.fn(),
  })),
}))

const mockAccessLists: AccessList[] = [
  {
    id: 1,
    uuid: 'acl-uuid-1',
    name: 'Office Network',
    description: 'Office IP range',
    type: 'whitelist',
    ip_rules: JSON.stringify([]),
    country_codes: '',
    local_network_only: false,
    enabled: true,
    created_at: '2024-01-01',
    updated_at: '2024-01-01',
  },
  {
    id: 2,
    uuid: 'acl-uuid-2',
    name: 'VPN Users',
    description: 'VPN IP range',
    type: 'whitelist',
    ip_rules: JSON.stringify([]),
    country_codes: '',
    local_network_only: false,
    enabled: true,
    created_at: '2024-01-01',
    updated_at: '2024-01-01',
  },
]

const mockSecurityProfiles: SecurityHeaderProfile[] = [
  {
    id: 1,
    uuid: 'profile-uuid-1',
    name: 'Basic Security',
    description: 'Basic security headers',
    is_preset: true,
    preset_type: 'basic',
    security_score: 60,
    created_at: '2024-01-01',
    updated_at: '2024-01-01',
    hsts_enabled: false,
    hsts_max_age: 0,
    hsts_include_subdomains: false,
    hsts_preload: false,
    csp_enabled: false,
    csp_directives: '',
    csp_report_only: false,
    csp_report_uri: '',
    x_frame_options: '',
    x_content_type_options: false,
    referrer_policy: '',
    permissions_policy: '',
    cross_origin_opener_policy: '',
    cross_origin_resource_policy: '',
    cross_origin_embedder_policy: '',
    xss_protection: false,
    cache_control_no_store: false,
  },
  {
    id: 2,
    uuid: 'profile-uuid-2',
    name: 'Strict Security',
    description: 'Strict security headers',
    is_preset: true,
    preset_type: 'strict',
    security_score: 90,
    created_at: '2024-01-01',
    updated_at: '2024-01-01',
    hsts_enabled: true,
    hsts_max_age: 31536000,
    hsts_include_subdomains: true,
    hsts_preload: true,
    csp_enabled: true,
    csp_directives: "default-src 'self'",
    csp_report_only: false,
    csp_report_uri: '',
    x_frame_options: 'DENY',
    x_content_type_options: true,
    referrer_policy: 'no-referrer',
    permissions_policy: '',
    cross_origin_opener_policy: 'same-origin',
    cross_origin_resource_policy: 'same-origin',
    cross_origin_embedder_policy: 'require-corp',
    xss_protection: true,
    cache_control_no_store: false,
  },
]

vi.mock('../../hooks/useAccessLists', () => ({
  useAccessLists: vi.fn(() => ({
    data: mockAccessLists,
    isLoading: false,
    error: null,
  })),
}))

vi.mock('../../hooks/useSecurityHeaders', () => ({
  useSecurityHeaderProfiles: vi.fn(() => ({
    data: mockSecurityProfiles,
    isLoading: false,
    error: null,
  })),
}))

// Mock fetch for health endpoint
vi.stubGlobal('fetch', vi.fn(() => Promise.resolve({
  json: () => Promise.resolve({ internal_ip: '127.0.0.1' }),
})))

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

describe('ProxyHostForm Dropdown Change Bug Fix', () => {
  let mockOnSubmit: (data: Partial<ProxyHost>) => Promise<void>
  let mockOnCancel: () => void

  beforeEach(() => {
    mockOnSubmit = vi.fn<(data: Partial<ProxyHost>) => Promise<void>>()
    mockOnCancel = vi.fn<() => void>()

    vi.mocked(useAccessLists).mockReturnValue({
      data: mockAccessLists,
      isLoading: false,
      error: null,
    } as unknown as ReturnType<typeof useAccessLists>)

    vi.mocked(useSecurityHeaderProfiles).mockReturnValue({
      data: mockSecurityProfiles,
      isLoading: false,
      error: null,
    } as unknown as ReturnType<typeof useSecurityHeaderProfiles>)
  })

  it('allows changing ACL selection after initial selection', async () => {
    const user = userEvent.setup()
    const Wrapper = createWrapper()

    render(
      <Wrapper>
        <ProxyHostForm onSubmit={mockOnSubmit} onCancel={mockOnCancel} />
      </Wrapper>
    )

    // Fill required fields
    await user.type(screen.getByLabelText(/^Name/), 'Test Service')
    await user.type(screen.getByLabelText(/Domain Names/), 'test.com')
    await user.type(screen.getByLabelText(/^Host$/), 'localhost')
    await user.clear(screen.getByLabelText(/^Port$/))
    await user.type(screen.getByLabelText(/^Port$/), '8080')

    // Select first ACL
    await user.click(screen.getByRole('combobox', { name: /Access Control List/i }))
    await user.click(await screen.findByRole('option', { name: /Office Network/i }))

    // Verify first ACL is selected
    expect(screen.getByText('Office Network')).toBeInTheDocument()

    // Change to second ACL
    await waitFor(() => {
      expect(screen.queryByRole('option', { name: /Office Network/i })).not.toBeInTheDocument()
    })
    await user.click(screen.getByRole('combobox', { name: /Access Control List/i }))
    await user.click(await screen.findByRole('option', { name: /VPN Users/i }))

    // Verify second ACL is now selected
    expect(screen.getByText('VPN Users')).toBeInTheDocument()

    // Submit and verify the correct ACL ID is in the payload
    await user.click(screen.getByRole('button', { name: /Save/i }))

    await waitFor(() => {
      expect(mockOnSubmit).toHaveBeenCalledWith(
        expect.objectContaining({
          access_list_id: 2, // Should be second ACL
        })
      )
    })
  })

  it('allows removing ACL selection', async () => {
    const user = userEvent.setup()
    const Wrapper = createWrapper()

    render(
      <Wrapper>
        <ProxyHostForm onSubmit={mockOnSubmit} onCancel={mockOnCancel} />
      </Wrapper>
    )

    // Fill required fields
    await user.type(screen.getByLabelText(/^Name/), 'Test Service')
    await user.type(screen.getByLabelText(/Domain Names/), 'test.com')
    await user.type(screen.getByLabelText(/^Host$/), 'localhost')
    await user.clear(screen.getByLabelText(/^Port$/))
    await user.type(screen.getByLabelText(/^Port$/), '8080')

    // Select an ACL
    await user.click(screen.getByRole('combobox', { name: /Access Control List/i }))
    await user.click(await screen.findByRole('option', { name: /Office Network/i }))

    // Verify ACL is selected
    expect(screen.getByText('Office Network')).toBeInTheDocument()

    // Remove ACL by selecting "No Access Control"
    await waitFor(() => {
      expect(screen.queryByRole('option', { name: /Office Network/i })).not.toBeInTheDocument()
    })
    await user.click(screen.getByRole('combobox', { name: /Access Control List/i }))
    await user.click(await screen.findByRole('option', { name: /No Access Control/i }))

    // Submit and verify ACL is null
    await user.click(screen.getByRole('button', { name: /Save/i }))

    await waitFor(() => {
      expect(mockOnSubmit).toHaveBeenCalledWith(
        expect.objectContaining({
          access_list_id: null,
        })
      )
    })
  })

  it('allows changing Security Headers selection after initial selection', async () => {
    const user = userEvent.setup()
    const Wrapper = createWrapper()

    render(
      <Wrapper>
        <ProxyHostForm onSubmit={mockOnSubmit} onCancel={mockOnCancel} />
      </Wrapper>
    )

    // Fill required fields
    await user.type(screen.getByLabelText(/^Name/), 'Test Service')
    await user.type(screen.getByLabelText(/Domain Names/), 'test.com')
    await user.type(screen.getByLabelText(/^Host$/), 'localhost')
    await user.clear(screen.getByLabelText(/^Port$/))
    await user.type(screen.getByLabelText(/^Port$/), '8080')

    // Select first security profile
    const headersTrigger = screen.getByRole('combobox', { name: /Security Headers/i })
    await user.click(headersTrigger)
    await user.click(await screen.findByRole('option', { name: /Basic Security/i }))

    // Change to second profile
    await user.click(headersTrigger)
    await user.click(await screen.findByRole('option', { name: /Strict Security/i }))

    // Submit and verify the correct profile ID is in the payload
    await user.click(screen.getByRole('button', { name: /Save/i }))

    await waitFor(() => {
      expect(mockOnSubmit).toHaveBeenCalledWith(
        expect.objectContaining({
          security_header_profile_id: 2, // Should be second profile
        })
      )
    })
  })

  it('allows removing Security Headers selection', async () => {
    const user = userEvent.setup()
    const Wrapper = createWrapper()

    render(
      <Wrapper>
        <ProxyHostForm onSubmit={mockOnSubmit} onCancel={mockOnCancel} />
      </Wrapper>
    )

    // Fill required fields
    await user.type(screen.getByLabelText(/^Name/), 'Test Service')
    await user.type(screen.getByLabelText(/Domain Names/), 'test.com')
    await user.type(screen.getByLabelText(/^Host$/), 'localhost')
    await user.clear(screen.getByLabelText(/^Port$/))
    await user.type(screen.getByLabelText(/^Port$/), '8080')

    // Select a security profile
    const headersTrigger = screen.getByRole('combobox', { name: /Security Headers/i })
    await user.click(headersTrigger)
    await user.click(await screen.findByRole('option', { name: /Basic Security/i }))

    // Remove security headers by selecting "None"
    await user.click(headersTrigger)
    await user.click(await screen.findByRole('option', { name: /None \(No Security Headers\)/i }))

    // Submit and verify security_header_profile_id is null
    await user.click(screen.getByRole('button', { name: /Save/i }))

    await waitFor(() => {
      expect(mockOnSubmit).toHaveBeenCalledWith(
        expect.objectContaining({
          security_header_profile_id: null,
        })
      )
    })
  })

  it('allows editing existing host with ACL and changing it', async () => {
    const user = userEvent.setup()
    const Wrapper = createWrapper()

    const existingHost: ProxyHost = {
      uuid: 'host-uuid-1',
      name: 'Existing Service',
      domain_names: 'existing.com',
      forward_scheme: 'http',
      forward_host: 'localhost',
      forward_port: 8080,
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
      access_list_id: 1, // Initially has first ACL
      security_header_profile_id: 1, // Initially has first profile
      dns_provider_id: null,
      created_at: '2024-01-01',
      updated_at: '2024-01-01',
    }

    render(
      <Wrapper>
        <ProxyHostForm host={existingHost} onSubmit={mockOnSubmit} onCancel={mockOnCancel} />
      </Wrapper>
    )

    // Verify initial ACL is shown
    expect(screen.getByText('Office Network')).toBeInTheDocument()

    // Change ACL
    const aclTrigger = screen.getByRole('combobox', { name: /Access Control List/i })
    await user.click(aclTrigger)
    await user.click(await screen.findByRole('option', { name: /VPN Users/i }))

    // Verify new ACL is shown
    expect(screen.getByText('VPN Users')).toBeInTheDocument()

    // Change security headers
    const headersTrigger = screen.getByRole('combobox', { name: /Security Headers/i })
    await user.click(headersTrigger)
    await user.click(await screen.findByRole('option', { name: /Strict Security/i }))

    // Submit and verify changes
    await user.click(screen.getByRole('button', { name: /Save/i }))

    await waitFor(() => {
      expect(mockOnSubmit).toHaveBeenCalledWith(
        expect.objectContaining({
          access_list_id: 2,
          security_header_profile_id: 2,
        })
      )
    })
  })

  it('persists null to value transitions for ACL and security headers in edit flow', async () => {
    const user = userEvent.setup()
    const Wrapper = createWrapper()

    const existingHostWithNulls: ProxyHost = {
      uuid: 'host-uuid-null-fields',
      name: 'Existing Null Fields',
      domain_names: 'existing-null.com',
      forward_scheme: 'http',
      forward_host: 'localhost',
      forward_port: 8080,
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
      created_at: '2024-01-01',
      updated_at: '2024-01-01',
    }

    render(
      <Wrapper>
        <ProxyHostForm host={existingHostWithNulls} onSubmit={mockOnSubmit} onCancel={mockOnCancel} />
      </Wrapper>
    )

    const aclTrigger = screen.getByRole('combobox', { name: /Access Control List/i })
    await user.click(aclTrigger)
    await user.click(await screen.findByRole('option', { name: /Office Network/i }))

    const headersTrigger = screen.getByRole('combobox', { name: /Security Headers/i })
    await user.click(headersTrigger)
    await user.click(await screen.findByRole('option', { name: /Strict Security/i }))

    await user.click(screen.getByRole('button', { name: /Save/i }))

    await waitFor(() => {
      expect(mockOnSubmit).toHaveBeenCalledWith(
        expect.objectContaining({
          access_list_id: 1,
          security_header_profile_id: 2,
        })
      )
    })
  })

  it('resets ACL/security header form state when editing target host changes', async () => {
    const user = userEvent.setup()
    const Wrapper = createWrapper()

    const firstHost: ProxyHost = {
      uuid: 'host-uuid-first',
      name: 'First Host',
      domain_names: 'first.example.com',
      forward_scheme: 'http',
      forward_host: 'localhost',
      forward_port: 8080,
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
      access_list_id: 1,
      security_header_profile_id: 1,
      dns_provider_id: null,
      created_at: '2024-01-01',
      updated_at: '2024-01-01',
    }

    const secondHost: ProxyHost = {
      ...firstHost,
      uuid: 'host-uuid-second',
      name: 'Second Host',
      domain_names: 'second.example.com',
      access_list_id: null,
      security_header_profile_id: null,
    }

    const { rerender } = render(
      <Wrapper>
        <ProxyHostForm host={firstHost} onSubmit={mockOnSubmit} onCancel={mockOnCancel} />
      </Wrapper>
    )

    // Mutate first host state in the form before switching targets.
    await user.click(screen.getByRole('combobox', { name: /Access Control List/i }))
    await user.click(await screen.findByRole('option', { name: /VPN Users/i }))

    await user.click(screen.getByRole('combobox', { name: /Security Headers/i }))
    await user.click(await screen.findByRole('option', { name: /Strict Security/i }))

    rerender(
      <Wrapper>
        <ProxyHostForm host={secondHost} onSubmit={mockOnSubmit} onCancel={mockOnCancel} />
      </Wrapper>
    )

    await user.click(screen.getByRole('button', { name: /Save/i }))

    await waitFor(() => {
      expect(mockOnSubmit).toHaveBeenCalledWith(
        expect.objectContaining({
          access_list_id: null,
          security_header_profile_id: null,
        })
      )
    })
  })

  it('persists ACL and security header selections with UUID-only option payloads', async () => {
    const user = userEvent.setup()
    const Wrapper = createWrapper()

    const uuidOnlyAccessLists = [
      {
        ...mockAccessLists[0],
        id: undefined,
        uuid: 'acl-uuid-only',
        name: 'UUID Office Network',
      },
    ]

    const uuidOnlySecurityProfiles = [
      {
        ...mockSecurityProfiles[0],
        id: undefined,
        uuid: 'profile-uuid-only',
        name: 'UUID Basic Security',
      },
    ]

    vi.mocked(useAccessLists).mockReturnValue({
      data: uuidOnlyAccessLists as unknown as AccessList[],
      isLoading: false,
      error: null,
    } as unknown as ReturnType<typeof useAccessLists>)

    vi.mocked(useSecurityHeaderProfiles).mockReturnValue({
      data: uuidOnlySecurityProfiles as unknown as SecurityHeaderProfile[],
      isLoading: false,
      error: null,
    } as unknown as ReturnType<typeof useSecurityHeaderProfiles>)

    render(
      <Wrapper>
        <ProxyHostForm onSubmit={mockOnSubmit} onCancel={mockOnCancel} />
      </Wrapper>
    )

    await user.type(screen.getByLabelText(/^Name/), 'UUID Test Service')
    await user.type(screen.getByLabelText(/Domain Names/), 'test.com')
    await user.type(screen.getByLabelText(/^Host$/), 'localhost')
    await user.clear(screen.getByLabelText(/^Port$/))
    await user.type(screen.getByLabelText(/^Port$/), '8080')

    const aclTrigger = screen.getByRole('combobox', { name: /Access Control List/i })
    await user.click(aclTrigger)
    await user.click(await screen.findByRole('option', { name: /UUID Office Network/i }))

    const headersTrigger = screen.getByRole('combobox', { name: /Security Headers/i })
    await user.click(headersTrigger)
    await user.click(await screen.findByRole('option', { name: /UUID Basic Security/i }))

    expect(screen.getByRole('combobox', { name: /Access Control List/i })).toHaveTextContent('UUID Office Network')
    expect(screen.getByRole('combobox', { name: /Security Headers/i })).toHaveTextContent('UUID Basic Security')

    await user.click(screen.getByRole('button', { name: /Save/i }))

    await waitFor(() => {
      expect(mockOnSubmit).toHaveBeenCalled()
    })
  })

  it('submits numeric ACL value when ACL option id is a numeric string', async () => {
    const user = userEvent.setup()
    const Wrapper = createWrapper()

    const stringIdAccessLists = [
      {
        ...mockAccessLists[0],
        id: '2',
        uuid: 'acl-string-id-2',
        name: 'String ID ACL',
      },
    ]

    vi.mocked(useAccessLists).mockReturnValue({
      data: stringIdAccessLists as unknown as AccessList[],
      isLoading: false,
      error: null,
    } as unknown as ReturnType<typeof useAccessLists>)

    render(
      <Wrapper>
        <ProxyHostForm onSubmit={mockOnSubmit} onCancel={mockOnCancel} />
      </Wrapper>
    )

    await user.type(screen.getByLabelText(/^Name/), 'String ID ACL Host')
    await user.type(screen.getByLabelText(/Domain Names/), 'test.com')
    await user.type(screen.getByLabelText(/^Host$/), 'localhost')
    await user.clear(screen.getByLabelText(/^Port$/))
    await user.type(screen.getByLabelText(/^Port$/), '8080')

    await user.click(screen.getByRole('combobox', { name: /Access Control List/i }))
    await user.click(await screen.findByRole('option', { name: /String ID ACL/i }))

    await user.click(screen.getByRole('combobox', { name: /Security Headers/i }))
    await user.click(await screen.findByRole('option', { name: /Basic Security/i }))

    await user.click(screen.getByRole('button', { name: /Save/i }))

    await waitFor(() => {
      expect(mockOnSubmit).toHaveBeenCalledWith(
        expect.objectContaining({
          access_list_id: 2,
          security_header_profile_id: 1,
        })
      )
    })
  })
})
