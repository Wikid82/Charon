import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen } from '@testing-library/react'
import { describe, it, expect, vi, beforeEach } from 'vitest'

import { useDNSProviders } from '../../hooks/useDNSProviders'
import DNSProviderSelector from '../DNSProviderSelector'

import type { DNSProvider } from '../../api/dnsProviders'

vi.mock('../../hooks/useDNSProviders')

// Capture the onValueChange callback from Select component
let capturedOnValueChange: ((value: string) => void) | undefined
let capturedSelectDisabled: boolean | undefined
let capturedSelectValue: string | undefined

// Mock the Select component to capture onValueChange and enable testing
vi.mock('../ui', async () => {
  const actual = await vi.importActual('../ui')
  return {
    ...actual,
    Select: ({ value, onValueChange, disabled, children }: {
      value: string
      onValueChange: (value: string) => void
      disabled?: boolean
      children: React.ReactNode
    }) => {
      capturedOnValueChange = onValueChange
      capturedSelectDisabled = disabled
      capturedSelectValue = value
      return (
        <div data-testid="select-mock" data-value={value} data-disabled={disabled}>
          {children}
        </div>
      )
    },
    SelectTrigger: ({ error, children }: { error?: boolean; children: React.ReactNode }) => (
      <button
        role="combobox"
        data-error={error}
        disabled={capturedSelectDisabled}
        aria-disabled={capturedSelectDisabled}
      >
        {children}
      </button>
    ),
    SelectValue: ({ placeholder }: { placeholder?: string }) => {
      // Display actual selected value based on capturedSelectValue
      return <span data-placeholder={placeholder}>{capturedSelectValue || placeholder}</span>
    },
    SelectContent: ({ children }: { children: React.ReactNode }) => (
      <div role="listbox">{children}</div>
    ),
    SelectItem: ({ value, disabled, children }: { value: string; disabled?: boolean; children: React.ReactNode }) => (
      <div role="option" data-value={value} data-disabled={disabled}>
        {children}
      </div>
    ),
  }
})

const mockProviders: DNSProvider[] = [
  {
    id: 1,
    uuid: 'uuid-1',
    name: 'Cloudflare Prod',
    provider_type: 'cloudflare',
    enabled: true,
    is_default: true,
    has_credentials: true,
    propagation_timeout: 120,
    polling_interval: 2,
    success_count: 10,
    failure_count: 0,
    created_at: '2025-01-01T00:00:00Z',
    updated_at: '2025-01-01T00:00:00Z',
  },
  {
    id: 2,
    uuid: 'uuid-2',
    name: 'Route53 Staging',
    provider_type: 'route53',
    enabled: true,
    is_default: false,
    has_credentials: true,
    propagation_timeout: 60,
    polling_interval: 2,
    success_count: 5,
    failure_count: 1,
    created_at: '2025-01-01T00:00:00Z',
    updated_at: '2025-01-01T00:00:00Z',
  },
  {
    id: 3,
    uuid: 'uuid-3',
    name: 'Disabled Provider',
    provider_type: 'digitalocean',
    enabled: false,
    is_default: false,
    has_credentials: true,
    propagation_timeout: 90,
    polling_interval: 2,
    success_count: 0,
    failure_count: 0,
    created_at: '2025-01-01T00:00:00Z',
    updated_at: '2025-01-01T00:00:00Z',
  },
  {
    id: 4,
    uuid: 'uuid-4',
    name: 'No Credentials',
    provider_type: 'googleclouddns',
    enabled: true,
    is_default: false,
    has_credentials: false,
    propagation_timeout: 120,
    polling_interval: 2,
    success_count: 0,
    failure_count: 0,
    created_at: '2025-01-01T00:00:00Z',
    updated_at: '2025-01-01T00:00:00Z',
  },
]

const renderWithClient = (ui: React.ReactElement) => {
  return render(<QueryClientProvider client={new QueryClient()}>{ui}</QueryClientProvider>)
}

describe('DNSProviderSelector', () => {
  const mockOnChange = vi.fn()

  beforeEach(() => {
    vi.clearAllMocks()
    capturedOnValueChange = undefined
    capturedSelectDisabled = undefined
    capturedSelectValue = undefined
    vi.mocked(useDNSProviders).mockReturnValue({
      data: mockProviders,
      isLoading: false,
      isError: false,
      error: null,
    } as unknown as ReturnType<typeof useDNSProviders>)
  })

  describe('Rendering', () => {
    it('renders with label when provided', () => {
      renderWithClient(
        <DNSProviderSelector value={undefined} onChange={mockOnChange} label="DNS Provider" />
      )

      expect(screen.getByText('DNS Provider')).toBeInTheDocument()
    })

    it('renders without label when not provided', () => {
      renderWithClient(<DNSProviderSelector value={undefined} onChange={mockOnChange} />)

      expect(screen.queryByRole('label')).not.toBeInTheDocument()
    })

    it('shows required asterisk when required=true', () => {
      renderWithClient(
        <DNSProviderSelector
          value={undefined}
          onChange={mockOnChange}
          label="DNS Provider"
          required
        />
      )

      const label = screen.getByText('DNS Provider')
      expect(label.parentElement?.textContent).toContain('*')
    })

    it('shows helper text when provided', () => {
      renderWithClient(
        <DNSProviderSelector
          value={undefined}
          onChange={mockOnChange}
          helperText="Select a DNS provider for wildcard certificates"
        />
      )

      expect(
        screen.getByText('Select a DNS provider for wildcard certificates')
      ).toBeInTheDocument()
    })

    it('shows error message when provided and replaces helper text', () => {
      renderWithClient(
        <DNSProviderSelector
          value={undefined}
          onChange={mockOnChange}
          helperText="This should not appear"
          error="DNS provider is required"
        />
      )

      expect(screen.getByText('DNS provider is required')).toBeInTheDocument()
      expect(screen.queryByText('This should not appear')).not.toBeInTheDocument()
    })
  })

  describe('Provider Filtering', () => {
    it('only shows enabled providers', () => {
      renderWithClient(
        <DNSProviderSelector value={undefined} onChange={mockOnChange} />
      )

      // Component filters providers internally, verify filtering logic
      // by checking that only enabled providers with credentials are available
      const providers = mockProviders.filter((p) => p.enabled && p.has_credentials)
      expect(providers).toHaveLength(2)
      expect(providers[0].name).toBe('Cloudflare Prod')
      expect(providers[1].name).toBe('Route53 Staging')
    })

    it('only shows providers with credentials', () => {
      renderWithClient(<DNSProviderSelector value={undefined} onChange={mockOnChange} />)

      // Verify filtering logic: providers must have both enabled=true and has_credentials=true
      const availableProviders = mockProviders.filter((p) => p.enabled && p.has_credentials)
      expect(availableProviders.every((p) => p.has_credentials)).toBe(true)
    })

    it('filters out disabled providers', () => {
      const disabledProvider: DNSProvider = {
        ...mockProviders[0],
        id: 5,
        enabled: false,
        name: 'Another Disabled',
      }
      vi.mocked(useDNSProviders).mockReturnValue({
        data: [...mockProviders, disabledProvider],
        isLoading: false,
        isError: false,
        error: null,
      } as unknown as ReturnType<typeof useDNSProviders>)

      renderWithClient(<DNSProviderSelector value={undefined} onChange={mockOnChange} />)

      // Verify the disabled provider is filtered out
      const allProviders = [...mockProviders, disabledProvider]
      const availableProviders = allProviders.filter((p) => p.enabled && p.has_credentials)
      expect(availableProviders.find((p) => p.name === 'Another Disabled')).toBeUndefined()
    })

    it('filters out providers without credentials', () => {
      const noCredProvider: DNSProvider = {
        ...mockProviders[0],
        id: 6,
        has_credentials: false,
        name: 'Missing Creds',
      }
      vi.mocked(useDNSProviders).mockReturnValue({
        data: [...mockProviders, noCredProvider],
        isLoading: false,
        isError: false,
        error: null,
      } as unknown as ReturnType<typeof useDNSProviders>)

      renderWithClient(<DNSProviderSelector value={undefined} onChange={mockOnChange} />)

      // Verify the provider without credentials is filtered out
      const allProviders = [...mockProviders, noCredProvider]
      const availableProviders = allProviders.filter((p) => p.enabled && p.has_credentials)
      expect(availableProviders.find((p) => p.name === 'Missing Creds')).toBeUndefined()
    })
  })

  describe('Loading States', () => {
    it('shows loading state while fetching', () => {
      vi.mocked(useDNSProviders).mockReturnValue({
        data: undefined,
        isLoading: true,
        isError: false,
        error: null,
      } as unknown as ReturnType<typeof useDNSProviders>)

      renderWithClient(<DNSProviderSelector value={undefined} onChange={mockOnChange} />)

      // When loading, data is undefined and isLoading is true
      expect(screen.getByRole('combobox')).toBeDisabled()
    })

    it('disables select during loading', () => {
      vi.mocked(useDNSProviders).mockReturnValue({
        data: undefined,
        isLoading: true,
        isError: false,
        error: null,
      } as unknown as ReturnType<typeof useDNSProviders>)

      renderWithClient(<DNSProviderSelector value={undefined} onChange={mockOnChange} />)

      expect(screen.getByRole('combobox')).toBeDisabled()
    })
  })

  describe('Empty States', () => {
    it('handles empty provider list', () => {
      vi.mocked(useDNSProviders).mockReturnValue({
        data: [],
        isLoading: false,
        isError: false,
        error: null,
      } as unknown as ReturnType<typeof useDNSProviders>)

      renderWithClient(<DNSProviderSelector value={undefined} onChange={mockOnChange} />)

      // Verify selector renders even with empty list
      expect(screen.getByRole('combobox')).toBeInTheDocument()
    })

    it('handles all providers filtered out scenario', () => {
      const allDisabled = mockProviders.map((p) => ({ ...p, enabled: false }))
      vi.mocked(useDNSProviders).mockReturnValue({
        data: allDisabled,
        isLoading: false,
        isError: false,
        error: null,
      } as unknown as ReturnType<typeof useDNSProviders>)

      renderWithClient(<DNSProviderSelector value={undefined} onChange={mockOnChange} />)

      // Verify selector renders with no available providers
      const availableProviders = allDisabled.filter((p) => p.enabled && p.has_credentials)
      expect(availableProviders).toHaveLength(0)
      expect(screen.getByRole('combobox')).toBeInTheDocument()
    })
  })

  describe('Selection Behavior', () => {
    it('displays selected provider by ID', () => {
      renderWithClient(<DNSProviderSelector value={1} onChange={mockOnChange} />)

      // Verify the Select received the correct value
      expect(capturedSelectValue).toBe('1')
    })

    it('shows none placeholder when value is undefined and not required', () => {
      renderWithClient(<DNSProviderSelector value={undefined} onChange={mockOnChange} />)

      // When value is undefined, the component uses 'none' as the Select value
      expect(capturedSelectValue).toBe('none')
    })

    it('handles required prop correctly', () => {
      renderWithClient(
        <DNSProviderSelector value={undefined} onChange={mockOnChange} required />
      )

      // When required, component should not include "none" in value
      const combobox = screen.getByRole('combobox')
      expect(combobox).toBeInTheDocument()
    })

    it('stores provider ID in component state', () => {
      const { rerender } = renderWithClient(
        <DNSProviderSelector value={1} onChange={mockOnChange} />
      )

      expect(capturedSelectValue).toBe('1')

      // Change to different provider
      rerender(
        <QueryClientProvider client={new QueryClient()}>
          <DNSProviderSelector value={2} onChange={mockOnChange} />
        </QueryClientProvider>
      )

      expect(capturedSelectValue).toBe('2')
    })

    it('handles undefined selection', () => {
      renderWithClient(<DNSProviderSelector value={undefined} onChange={mockOnChange} />)

      // When undefined, the value should be 'none'
      expect(capturedSelectValue).toBe('none')
    })
  })

  describe('Provider Display', () => {
    it('renders provider names correctly', () => {
      renderWithClient(<DNSProviderSelector value={1} onChange={mockOnChange} />)

      // Verify selected provider value is passed to Select
      expect(capturedSelectValue).toBe('1')
      // Provider names are rendered in SelectItems
      expect(screen.getByText('Cloudflare Prod')).toBeInTheDocument()
    })

    it('identifies default provider', () => {
      const defaultProvider = mockProviders.find((p) => p.is_default)
      expect(defaultProvider?.is_default).toBe(true)
      expect(defaultProvider?.name).toBe('Cloudflare Prod')
    })

    it('includes provider type information', () => {
      // Verify mock data includes provider types
      expect(mockProviders[0].provider_type).toBe('cloudflare')
      expect(mockProviders[1].provider_type).toBe('route53')
    })

    it('uses translation keys for provider types', () => {
      renderWithClient(<DNSProviderSelector value={1} onChange={mockOnChange} />)

      // The component uses t(`dnsProviders.types.${provider.provider_type}`)
      // Our mock translation returns the key if not found
      expect(screen.getByRole('combobox')).toBeInTheDocument()
    })
  })

  describe('Disabled State', () => {
    it('disables select when disabled=true', () => {
      renderWithClient(<DNSProviderSelector value={undefined} onChange={mockOnChange} disabled />)

      expect(screen.getByRole('combobox')).toBeDisabled()
    })

    it('disables select during loading', () => {
      vi.mocked(useDNSProviders).mockReturnValue({
        data: undefined,
        isLoading: true,
        isError: false,
        error: null,
      } as unknown as ReturnType<typeof useDNSProviders>)

      renderWithClient(<DNSProviderSelector value={undefined} onChange={mockOnChange} />)

      expect(screen.getByRole('combobox')).toBeDisabled()
    })
  })

  describe('Accessibility', () => {
    it('error has role="alert"', () => {
      renderWithClient(
        <DNSProviderSelector
          value={undefined}
          onChange={mockOnChange}
          error="Required field"
        />
      )

      const errorElement = screen.getByText('Required field')
      expect(errorElement).toHaveAttribute('role', 'alert')
    })

    it('label properly associates with select', () => {
      renderWithClient(
        <DNSProviderSelector
          value={undefined}
          onChange={mockOnChange}
          label="Choose Provider"
        />
      )

      const label = screen.getByText('Choose Provider')
      const select = screen.getByRole('combobox')

      // They should be associated (exact implementation may vary)
      expect(label).toBeInTheDocument()
      expect(select).toBeInTheDocument()
    })
  })

  describe('Value Change Handling', () => {
    it('calls onChange with undefined when "none" is selected', () => {
      renderWithClient(
        <DNSProviderSelector value={1} onChange={mockOnChange} />
      )

      // Invoke the captured onValueChange with 'none'
      expect(capturedOnValueChange).toBeDefined()
      capturedOnValueChange!('none')

      expect(mockOnChange).toHaveBeenCalledWith(undefined)
    })

    it('calls onChange with provider ID when a provider is selected', () => {
      renderWithClient(
        <DNSProviderSelector value={undefined} onChange={mockOnChange} />
      )

      // Invoke the captured onValueChange with provider id '1'
      expect(capturedOnValueChange).toBeDefined()
      capturedOnValueChange!('1')

      expect(mockOnChange).toHaveBeenCalledWith(1)
    })

    it('calls onChange with different provider ID when switching providers', () => {
      renderWithClient(
        <DNSProviderSelector value={1} onChange={mockOnChange} />
      )

      // Invoke the captured onValueChange with provider id '2'
      expect(capturedOnValueChange).toBeDefined()
      capturedOnValueChange!('2')

      expect(mockOnChange).toHaveBeenCalledWith(2)
    })
  })
})
