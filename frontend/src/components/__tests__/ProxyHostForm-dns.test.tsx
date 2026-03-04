import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import ProxyHostForm from '../ProxyHostForm'
import type { ProxyHost } from '../../api/proxyHosts'
import { mockRemoteServers } from '../../test/mockData'
import { toast } from 'react-hot-toast'

// Mock the hooks
vi.mock('../../hooks/useRemoteServers', () => ({
  useRemoteServers: vi.fn(() => ({
    servers: mockRemoteServers,
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
    domains: [{ uuid: 'domain-1', name: 'example.com' }],
    createDomain: vi.fn().mockResolvedValue({ uuid: 'domain-1', name: 'example.com' }),
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

vi.mock('../../hooks/useSecurity', () => ({
  useAuthPolicies: vi.fn(() => ({
    policies: [],
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

vi.mock('../../hooks/useDNSProviders', () => ({
  useDNSProviders: vi.fn(() => ({
    data: [
      {
        id: 1,
        uuid: 'dns-uuid-1',
        name: 'Cloudflare',
        provider_type: 'cloudflare',
        enabled: true,
        is_default: true,
        has_credentials: true,
        propagation_timeout: 120,
        polling_interval: 2,
        success_count: 5,
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
    mutateAsync: vi.fn().mockResolvedValue({
      domain: 'example.com',
      detected: false,
      nameservers: [],
      confidence: 'none',
    }),
    isPending: false,
    data: undefined,
    reset: vi.fn(),
  })),
}))

vi.mock('../DNSDetectionResult', () => ({
  DNSDetectionResult: ({ result, onUseSuggested, onSelectManually }: {
    result?: { suggested_provider?: { id: number; name: string } }
    isLoading: boolean
    onUseSuggested: (provider: { id: number; name: string }) => void
    onSelectManually: () => void
  }) => (
    <div>
      <button
        type="button"
        onClick={() => {
          if (result?.suggested_provider) {
            onUseSuggested(result.suggested_provider)
          }
        }}
      >
        Use Suggested DNS
      </button>
      <button type="button" onClick={onSelectManually}>Select Manually DNS</button>
    </div>
  ),
}))

vi.mock('react-hot-toast', () => ({
  toast: {
    success: vi.fn(),
    error: vi.fn(),
  },
}))

vi.mock('../../api/dnsDetection', () => ({
  detectDNSProvider: vi.fn().mockResolvedValue({
    domain: 'example.com',
    detected: false,
    nameservers: [],
    confidence: 'none',
  }),
  getDetectionPatterns: vi.fn().mockResolvedValue([]),
}))

vi.mock('../../api/proxyHosts', () => ({
  testProxyHostConnection: vi.fn(),
}))

const mockFetch = vi.fn()
vi.stubGlobal('fetch', mockFetch)

const renderWithClient = (ui: React.ReactElement) => {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: {
        retry: false,
      },
    },
  })
  return render(<QueryClientProvider client={queryClient}>{ui}</QueryClientProvider>)
}

describe('ProxyHostForm - DNS Provider Integration', () => {
  const mockOnSubmit = vi.fn(() => Promise.resolve())
  const mockOnCancel = vi.fn()

  beforeEach(() => {
    vi.clearAllMocks()
    mockFetch.mockResolvedValue({
      json: () => Promise.resolve({ internal_ip: '192.168.1.50' }),
    })
  })

  describe('Wildcard Domain Detection', () => {
    it('detects *.example.com as wildcard', async () => {
      renderWithClient(<ProxyHostForm onSubmit={mockOnSubmit} onCancel={mockOnCancel} />)

      const domainInput = screen.getByPlaceholderText('example.com, www.example.com')
      await userEvent.type(domainInput, '*.example.com')

      await waitFor(() => {
        expect(screen.getByText('Wildcard Certificate Required')).toBeInTheDocument()
      })
    })

    it('does not detect sub.example.com as wildcard', async () => {
      renderWithClient(<ProxyHostForm onSubmit={mockOnSubmit} onCancel={mockOnCancel} />)

      const domainInput = screen.getByPlaceholderText('example.com, www.example.com')
      await userEvent.type(domainInput, 'sub.example.com')

      expect(screen.queryByText('Wildcard Certificate Required')).not.toBeInTheDocument()
    })

    it('detects multiple wildcards in comma-separated list', async () => {
      renderWithClient(<ProxyHostForm onSubmit={mockOnSubmit} onCancel={mockOnCancel} />)

      const domainInput = screen.getByPlaceholderText('example.com, www.example.com')
      await userEvent.type(domainInput, 'app.test.com, *.wildcard.com, api.test.com')

      await waitFor(() => {
        expect(screen.getByText('Wildcard Certificate Required')).toBeInTheDocument()
      })
    })

    it('detects wildcard at start of comma-separated list', async () => {
      renderWithClient(<ProxyHostForm onSubmit={mockOnSubmit} onCancel={mockOnCancel} />)

      const domainInput = screen.getByPlaceholderText('example.com, www.example.com')
      await userEvent.type(domainInput, '*.example.com, app.example.com')

      await waitFor(() => {
        expect(screen.getByText('Wildcard Certificate Required')).toBeInTheDocument()
      })
    })
  })

  describe('DNS Provider Requirement for Wildcards', () => {
    it('shows DNS provider selector when wildcard domain entered', async () => {
      renderWithClient(<ProxyHostForm onSubmit={mockOnSubmit} onCancel={mockOnCancel} />)

      const domainInput = screen.getByPlaceholderText('example.com, www.example.com')
      await userEvent.type(domainInput, '*.example.com')

      await waitFor(() => {
        expect(screen.getByText('Wildcard Certificate Required')).toBeInTheDocument()
        // Verify the selector combobox is rendered (even without opening it)
        const selectors = screen.getAllByRole('combobox')
        expect(selectors.length).toBeGreaterThan(3) // More than the base form selectors
      })
    })

    it('shows info alert explaining DNS-01 requirement', async () => {
      renderWithClient(<ProxyHostForm onSubmit={mockOnSubmit} onCancel={mockOnCancel} />)

      const domainInput = screen.getByPlaceholderText('example.com, www.example.com')
      await userEvent.type(domainInput, '*.example.com')

      await waitFor(() => {
        expect(screen.getByText('Wildcard Certificate Required')).toBeInTheDocument()
        expect(
          screen.getByText(/Wildcard certificates.*require DNS-01 challenge/i)
        ).toBeInTheDocument()
      })
    })

    it('shows validation error on submit if wildcard without provider', async () => {
      renderWithClient(<ProxyHostForm onSubmit={mockOnSubmit} onCancel={mockOnCancel} />)

      // Fill required fields
      await userEvent.type(screen.getByPlaceholderText('My Service'), 'Test Service')
      await userEvent.type(
        screen.getByPlaceholderText('example.com, www.example.com'),
        '*.example.com'
      )
      await userEvent.type(screen.getByLabelText(/^Host$/), '192.168.1.100')
      await userEvent.clear(screen.getByLabelText(/^Port$/))
      await userEvent.type(screen.getByLabelText(/^Port$/), '8080')

      // Submit without selecting DNS provider
      await userEvent.click(screen.getByText('Save'))

      // Should not call onSubmit
      await waitFor(() => {
        expect(mockOnSubmit).not.toHaveBeenCalled()
      })
    })

    it('does not show DNS provider selector without wildcard', async () => {
      renderWithClient(<ProxyHostForm onSubmit={mockOnSubmit} onCancel={mockOnCancel} />)

      const domainInput = screen.getByPlaceholderText('example.com, www.example.com')
      await userEvent.type(domainInput, 'app.example.com')

      // DNS Provider section should not appear
      expect(screen.queryByText('Wildcard Certificate Required')).not.toBeInTheDocument()
    })
  })

  describe('DNS Provider Selection', () => {
    it('DNS provider selector is present for wildcard domains', async () => {
      renderWithClient(<ProxyHostForm onSubmit={mockOnSubmit} onCancel={mockOnCancel} />)

      // Enter wildcard domain to show DNS selector
      const domainInput = screen.getByPlaceholderText('example.com, www.example.com')
      await userEvent.type(domainInput, '*.example.com')

      await waitFor(() => {
        expect(screen.getByText('Wildcard Certificate Required')).toBeInTheDocument()
      })

      // DNS provider selector should be rendered (it's a combobox without explicit name)
      const comboboxes = screen.getAllByRole('combobox')
      // There should be extra combobox(es) now for DNS provider
      expect(comboboxes.length).toBeGreaterThan(5) // Base form has ~5 comboboxes
    })

    it('clears DNS provider when switching to non-wildcard', async () => {
      renderWithClient(<ProxyHostForm onSubmit={mockOnSubmit} onCancel={mockOnCancel} />)

      // Enter wildcard
      const domainInput = screen.getByPlaceholderText('example.com, www.example.com')
      await userEvent.type(domainInput, '*.example.com')

      await waitFor(() => {
        expect(screen.getByText('Wildcard Certificate Required')).toBeInTheDocument()
      })

      // Change to non-wildcard domain
      await userEvent.clear(domainInput)
      await userEvent.type(domainInput, 'app.example.com')

      // DNS provider selector should disappear
      await waitFor(() => {
        expect(screen.queryByText('Wildcard Certificate Required')).not.toBeInTheDocument()
      })
    })

    it('preserves form state during wildcard domain edits', async () => {
      renderWithClient(<ProxyHostForm onSubmit={mockOnSubmit} onCancel={mockOnCancel} />)

      // Fill name field
      await userEvent.type(screen.getByPlaceholderText('My Service'), 'Test Service')

      // Enter wildcard
      const domainInput = screen.getByPlaceholderText('example.com, www.example.com')
      await userEvent.type(domainInput, '*.example.com')

      await waitFor(() => {
        expect(screen.getByText('Wildcard Certificate Required')).toBeInTheDocument()
      })

      // Edit other fields
      await userEvent.type(screen.getByLabelText(/^Host$/), '10.0.0.5')

      // Name should still be present
      expect(screen.getByPlaceholderText('My Service')).toHaveValue('Test Service')
    })
  })

  describe('Form Submission with DNS Provider', () => {
    it('includes dns_provider_id null for non-wildcard domains', async () => {
      renderWithClient(<ProxyHostForm onSubmit={mockOnSubmit} onCancel={mockOnCancel} />)

      // Fill required fields without wildcard
      await userEvent.type(screen.getByPlaceholderText('My Service'), 'Regular Service')
      await userEvent.type(
        screen.getByPlaceholderText('example.com, www.example.com'),
        'app.example.com'
      )
      await userEvent.type(screen.getByLabelText(/^Host$/), '192.168.1.100')
      await userEvent.clear(screen.getByLabelText(/^Port$/))
      await userEvent.type(screen.getByLabelText(/^Port$/), '8080')

      // Submit form
      await userEvent.click(screen.getByText('Save'))

      await waitFor(() => {
        expect(mockOnSubmit).toHaveBeenCalledWith(
          expect.objectContaining({
            dns_provider_id: null,
          })
        )
      })
    })

    it('prevents submission when wildcard present without DNS provider', async () => {
      renderWithClient(<ProxyHostForm onSubmit={mockOnSubmit} onCancel={mockOnCancel} />)

      // Fill required fields with wildcard
      await userEvent.type(screen.getByPlaceholderText('My Service'), 'Test Service')
      await userEvent.type(
        screen.getByPlaceholderText('example.com, www.example.com'),
        '*.example.com'
      )
      await userEvent.type(screen.getByLabelText(/^Host$/), '192.168.1.100')
      await userEvent.clear(screen.getByLabelText(/^Port$/))
      await userEvent.type(screen.getByLabelText(/^Port$/), '8080')

      // Submit without selecting DNS provider
      await userEvent.click(screen.getByText('Save'))

      // Should not call onSubmit due to validation
      await waitFor(() => {
        expect(mockOnSubmit).not.toHaveBeenCalled()
      })
    })

    it('loads existing host with DNS provider correctly', async () => {
      const existingHost: ProxyHost = {
        uuid: 'test-uuid',
        name: 'Existing Wildcard',
        domain_names: '*.example.com',
        forward_scheme: 'http',
        forward_host: '192.168.1.100',
        forward_port: 8080,
        ssl_forced: true,
        http2_support: true,
        hsts_enabled: true,
        hsts_subdomains: false,
        block_exploits: true,
        websocket_support: false,
        application: 'none',
        locations: [],
        enabled: true,
        dns_provider_id: 1,
        created_at: '2025-01-01T00:00:00Z',
        updated_at: '2025-01-01T00:00:00Z',
      }

      renderWithClient(
        <ProxyHostForm host={existingHost} onSubmit={mockOnSubmit} onCancel={mockOnCancel} />
      )

      // DNS provider section should be visible due to wildcard
      await waitFor(() => {
        expect(screen.getByText('Wildcard Certificate Required')).toBeInTheDocument()
      })

      // The form should have wildcard domain loaded
      expect(screen.getByPlaceholderText('example.com, www.example.com')).toHaveValue(
        '*.example.com'
      )
    })

    it('submits with dns_provider_id when editing existing wildcard host', async () => {
      const existingHost: ProxyHost = {
        uuid: 'test-uuid',
        name: 'Existing Wildcard',
        domain_names: '*.example.com',
        forward_scheme: 'http',
        forward_host: '192.168.1.100',
        forward_port: 8080,
        ssl_forced: true,
        http2_support: true,
        hsts_enabled: true,
        hsts_subdomains: false,
        block_exploits: true,
        websocket_support: false,
        application: 'none',
        locations: [],
        enabled: true,
        dns_provider_id: 1,
        created_at: '2025-01-01T00:00:00Z',
        updated_at: '2025-01-01T00:00:00Z',
      }

      renderWithClient(
        <ProxyHostForm host={existingHost} onSubmit={mockOnSubmit} onCancel={mockOnCancel} />
      )

      await waitFor(() => {
        expect(screen.getByText('Wildcard Certificate Required')).toBeInTheDocument()
      })

      // Submit without changes
      await userEvent.click(screen.getByText('Save'))

      await waitFor(() => {
        expect(mockOnSubmit).toHaveBeenCalledWith(
          expect.objectContaining({
            dns_provider_id: 1,
          })
        )
      })
    })
  })

  describe('DNS Detection Branches', () => {
    it('skips detection call when wildcard has provider set and no suggestion', async () => {
      vi.useFakeTimers()
      const { useDetectDNSProvider } = await import('../../hooks/useDNSDetection')
      const detectSpy = vi.fn().mockResolvedValue({
        domain: 'example.com',
        detected: false,
        nameservers: [],
        confidence: 'none',
      })

      vi.mocked(useDetectDNSProvider).mockReturnValue({
        mutateAsync: detectSpy,
        isPending: false,
        data: undefined,
        reset: vi.fn(),
      } as unknown as ReturnType<typeof useDetectDNSProvider>)

      const existingHost: ProxyHost = {
        uuid: 'test-uuid-skip-detect',
        name: 'Existing Wildcard Provider',
        domain_names: '*.example.com',
        forward_scheme: 'http',
        forward_host: '192.168.1.100',
        forward_port: 8080,
        ssl_forced: true,
        http2_support: true,
        hsts_enabled: true,
        hsts_subdomains: false,
        block_exploits: true,
        websocket_support: false,
        application: 'none',
        locations: [],
        enabled: true,
        dns_provider_id: 1,
        created_at: '2025-01-01T00:00:00Z',
        updated_at: '2025-01-01T00:00:00Z',
      }

      renderWithClient(
        <ProxyHostForm host={existingHost} onSubmit={mockOnSubmit} onCancel={mockOnCancel} />
      )

      await vi.advanceTimersByTimeAsync(600)

      expect(detectSpy).not.toHaveBeenCalled()
      vi.useRealTimers()
    })

    it('logs detection errors when detectProvider rejects', async () => {
      const { useDetectDNSProvider } = await import('../../hooks/useDNSDetection')
      const detectSpy = vi.fn().mockRejectedValue(new Error('detect failed'))
      const errorSpy = vi.spyOn(console, 'error').mockImplementation(() => {})

      vi.mocked(useDetectDNSProvider).mockReturnValue({
        mutateAsync: detectSpy,
        isPending: false,
        data: undefined,
        reset: vi.fn(),
      } as unknown as ReturnType<typeof useDetectDNSProvider>)

      renderWithClient(<ProxyHostForm onSubmit={mockOnSubmit} onCancel={mockOnCancel} />)

      const domainInput = screen.getByPlaceholderText('example.com, www.example.com')
      await userEvent.type(domainInput, '*.example.com')

      await new Promise((resolve) => setTimeout(resolve, 700))

      await waitFor(() => {
        expect(errorSpy).toHaveBeenCalledWith('DNS detection failed:', expect.any(Error))
      })

      errorSpy.mockRestore()
    })

    it('auto-selects high confidence suggestion and emits success toast', async () => {
      const { useDetectDNSProvider } = await import('../../hooks/useDNSDetection')
      vi.mocked(useDetectDNSProvider).mockReturnValue({
        mutateAsync: vi.fn().mockResolvedValue({}),
        isPending: false,
        data: {
          domain: 'example.com',
          detected: true,
          nameservers: ['ns1.cloudflare.com'],
          confidence: 'high',
          suggested_provider: { id: 1, name: 'Cloudflare' },
        },
        reset: vi.fn(),
      } as unknown as ReturnType<typeof useDetectDNSProvider>)

      renderWithClient(<ProxyHostForm onSubmit={mockOnSubmit} onCancel={mockOnCancel} />)

      await userEvent.type(screen.getByPlaceholderText('My Service'), 'Auto Select')
      await userEvent.type(screen.getByPlaceholderText('example.com, www.example.com'), '*.example.com')
      await userEvent.type(screen.getByLabelText(/^Host$/), '192.168.1.100')
      await userEvent.clear(screen.getByLabelText(/^Port$/))
      await userEvent.type(screen.getByLabelText(/^Port$/), '8080')
      await userEvent.click(screen.getByText('Save'))

      await waitFor(() => {
        expect(toast.success).toHaveBeenCalledWith('Auto-selected: Cloudflare')
        expect(mockOnSubmit).toHaveBeenCalledWith(expect.objectContaining({ dns_provider_id: 1 }))
      })
    })

    it('handles suggested and manual selection callbacks from detection result card', async () => {
      const { useDetectDNSProvider } = await import('../../hooks/useDNSDetection')
      vi.mocked(useDetectDNSProvider).mockReturnValue({
        mutateAsync: vi.fn().mockResolvedValue({}),
        isPending: false,
        data: {
          domain: 'example.com',
          detected: true,
          nameservers: ['ns1.cloudflare.com'],
          confidence: 'medium',
          suggested_provider: { id: 1, name: 'Cloudflare' },
        },
        reset: vi.fn(),
      } as unknown as ReturnType<typeof useDetectDNSProvider>)

      renderWithClient(<ProxyHostForm onSubmit={mockOnSubmit} onCancel={mockOnCancel} />)

      await userEvent.type(screen.getByPlaceholderText('example.com, www.example.com'), '*.example.com')

      await waitFor(() => {
        expect(screen.getByRole('button', { name: 'Use Suggested DNS' })).toBeInTheDocument()
      })

      await userEvent.click(screen.getByRole('button', { name: 'Use Suggested DNS' }))
      expect(toast.success).toHaveBeenCalledWith('Selected: Cloudflare')

      await userEvent.click(screen.getByRole('button', { name: 'Select Manually DNS' }))
    })
  })
})
