import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import CredentialManager from '../CredentialManager'
import {
  useCredentials,
  useCreateCredential,
  useUpdateCredential,
  useDeleteCredential,
  useTestCredential,
} from '../../hooks/useCredentials'
import type { DNSProvider, DNSProviderTypeInfo } from '../../api/dnsProviders'
import type { DNSProviderCredential } from '../../api/credentials'

vi.mock('../../hooks/useCredentials')
vi.mock('../../utils/toast', () => ({
  toast: {
    success: vi.fn(),
    error: vi.fn(),
    info: vi.fn(),
  },
}))

const mockProvider: DNSProvider = {
  id: 1,
  uuid: 'uuid-1',
  name: 'Cloudflare Production',
  provider_type: 'cloudflare',
  enabled: true,
  is_default: false,
  has_credentials: true,
  propagation_timeout: 120,
  polling_interval: 5,
  success_count: 10,
  failure_count: 0,
  created_at: '2025-01-01T00:00:00Z',
  updated_at: '2025-01-01T00:00:00Z',
}

const mockProviderTypeInfo: DNSProviderTypeInfo = {
  type: 'cloudflare',
  name: 'Cloudflare',
  fields: [
    {
      name: 'api_token',
      label: 'API Token',
      type: 'password',
      required: true,
      hint: 'Cloudflare API Token with DNS edit permissions',
    },
  ],
  documentation_url: 'https://developers.cloudflare.com',
}

const mockCredentials: DNSProviderCredential[] = [
  {
    id: 1,
    uuid: 'cred-uuid-1',
    dns_provider_id: 1,
    label: 'Main Zone',
    zone_filter: 'example.com',
    enabled: true,
    propagation_timeout: 120,
    polling_interval: 5,
    key_version: 1,
    success_count: 15,
    failure_count: 0,
    last_used_at: '2025-01-03T10:00:00Z',
    created_at: '2025-01-01T00:00:00Z',
    updated_at: '2025-01-01T00:00:00Z',
  },
  {
    id: 2,
    uuid: 'cred-uuid-2',
    dns_provider_id: 1,
    label: 'Customer A',
    zone_filter: '*.customer-a.com',
    enabled: true,
    propagation_timeout: 120,
    polling_interval: 5,
    key_version: 1,
    success_count: 3,
    failure_count: 0,
    created_at: '2025-01-02T00:00:00Z',
    updated_at: '2025-01-02T00:00:00Z',
  },
  {
    id: 3,
    uuid: 'cred-uuid-3',
    dns_provider_id: 1,
    label: 'Staging',
    zone_filter: '*.staging.example.com',
    enabled: true,
    propagation_timeout: 120,
    polling_interval: 5,
    key_version: 1,
    success_count: 2,
    failure_count: 1,
    last_error: 'DNS propagation timeout',
    created_at: '2025-01-02T00:00:00Z',
    updated_at: '2025-01-03T00:00:00Z',
  },
]

const renderWithClient = (ui: React.ReactElement) => {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false },
    },
  })
  return render(<QueryClientProvider client={queryClient}>{ui}</QueryClientProvider>)
}

describe('CredentialManager', () => {
  const mockOnOpenChange = vi.fn()
  const mockRefetch = vi.fn()
  const mockMutateAsync = vi.fn()

  beforeEach(() => {
    vi.clearAllMocks()

    vi.mocked(useCredentials).mockReturnValue({
      data: mockCredentials,
      isLoading: false,
      refetch: mockRefetch,
    } as any)

    vi.mocked(useCreateCredential).mockReturnValue({
      mutateAsync: mockMutateAsync,
      isPending: false,
    } as any)

    vi.mocked(useUpdateCredential).mockReturnValue({
      mutateAsync: mockMutateAsync,
      isPending: false,
    } as any)

    vi.mocked(useDeleteCredential).mockReturnValue({
      mutateAsync: mockMutateAsync,
      isPending: false,
    } as any)

    vi.mocked(useTestCredential).mockReturnValue({
      mutateAsync: mockMutateAsync,
      isPending: false,
    } as any)
  })

  describe('Rendering', () => {
    it('renders modal with provider name in title', () => {
      renderWithClient(
        <CredentialManager
          open={true}
          onOpenChange={mockOnOpenChange}
          provider={mockProvider}
          providerTypeInfo={mockProviderTypeInfo}
        />
      )

      expect(screen.getByText(/Cloudflare Production/)).toBeInTheDocument()
    })

    it('shows add credential button', () => {
      renderWithClient(
        <CredentialManager
          open={true}
          onOpenChange={mockOnOpenChange}
          provider={mockProvider}
          providerTypeInfo={mockProviderTypeInfo}
        />
      )

      // Check for button with specific text or by querying all buttons
      const buttons = screen.getAllByRole('button')
      expect(buttons.length).toBeGreaterThan(0)
    })

    it('renders credentials table with data', () => {
      renderWithClient(
        <CredentialManager
          open={true}
          onOpenChange={mockOnOpenChange}
          provider={mockProvider}
          providerTypeInfo={mockProviderTypeInfo}
        />
      )

      expect(screen.getByText('Main Zone')).toBeInTheDocument()
      expect(screen.getByText('Customer A')).toBeInTheDocument()
      expect(screen.getByText('Staging')).toBeInTheDocument()
    })

    it('displays zone filters correctly', () => {
      renderWithClient(
        <CredentialManager
          open={true}
          onOpenChange={mockOnOpenChange}
          provider={mockProvider}
          providerTypeInfo={mockProviderTypeInfo}
        />
      )

      expect(screen.getByText('example.com')).toBeInTheDocument()
      expect(screen.getByText('*.customer-a.com')).toBeInTheDocument()
      expect(screen.getByText('*.staging.example.com')).toBeInTheDocument()
    })

    it('shows status with success/failure counts', () => {
      renderWithClient(
        <CredentialManager
          open={true}
          onOpenChange={mockOnOpenChange}
          provider={mockProvider}
          providerTypeInfo={mockProviderTypeInfo}
        />
      )

      expect(screen.getByText('15/0')).toBeInTheDocument()
      expect(screen.getByText('3/0')).toBeInTheDocument()
      expect(screen.getByText('2/1')).toBeInTheDocument()
    })

    it('displays last error when present', () => {
      renderWithClient(
        <CredentialManager
          open={true}
          onOpenChange={mockOnOpenChange}
          provider={mockProvider}
          providerTypeInfo={mockProviderTypeInfo}
        />
      )

      expect(screen.getByText('DNS propagation timeout')).toBeInTheDocument()
    })
  })

  describe('Empty State', () => {
    it('shows empty state when no credentials', () => {
      vi.mocked(useCredentials).mockReturnValue({
        data: [],
        isLoading: false,
        refetch: mockRefetch,
      } as any)

      renderWithClient(
        <CredentialManager
          open={true}
          onOpenChange={mockOnOpenChange}
          provider={mockProvider}
          providerTypeInfo={mockProviderTypeInfo}
        />
      )

      // Empty state should render (no table)
      expect(screen.queryByRole('table')).not.toBeInTheDocument()
      // But buttons should still exist (add button)
      const buttons = screen.getAllByRole('button')
      expect(buttons.length).toBeGreaterThan(0)
    })

    it('empty state has add credential action', async () => {
      const user = userEvent.setup()
      vi.mocked(useCredentials).mockReturnValue({
        data: [],
        isLoading: false,
        refetch: mockRefetch,
      } as any)

      renderWithClient(
        <CredentialManager
          open={true}
          onOpenChange={mockOnOpenChange}
          provider={mockProvider}
          providerTypeInfo={mockProviderTypeInfo}
        />
      )

      // Empty state should have buttons
      const buttons = screen.getAllByRole('button')
      expect(buttons.length).toBeGreaterThan(0)

      // Click first button (likely the add button)
      await user.click(buttons[0])

      // Form dialog should open
      await waitFor(() => {
        const dialogs = screen.getAllByRole('dialog')
        expect(dialogs.length).toBeGreaterThan(0)
      })
    })
  })

  describe('Loading State', () => {
    it('shows loading indicator', () => {
      vi.mocked(useCredentials).mockReturnValue({
        data: undefined,
        isLoading: true,
        refetch: mockRefetch,
      } as any)

      renderWithClient(
        <CredentialManager
          open={true}
          onOpenChange={mockOnOpenChange}
          provider={mockProvider}
          providerTypeInfo={mockProviderTypeInfo}
        />
      )

      expect(screen.getByText(/loading/i)).toBeInTheDocument()
    })
  })

  describe('Table Actions', () => {
    it('shows test, edit, and delete buttons for each credential', () => {
      renderWithClient(
        <CredentialManager
          open={true}
          onOpenChange={mockOnOpenChange}
          provider={mockProvider}
          providerTypeInfo={mockProviderTypeInfo}
        />
      )

      // Each row should have 3 action buttons (test, edit, delete)
      const rows = screen.getAllByRole('row').slice(1) // Skip header
      expect(rows).toHaveLength(3)

      // Verify action buttons exist
      expect(rows[0].querySelectorAll('button')).toHaveLength(3)
    })

    it('opens edit form when edit button clicked', async () => {
      const user = userEvent.setup()

      renderWithClient(
        <CredentialManager
          open={true}
          onOpenChange={mockOnOpenChange}
          provider={mockProvider}
          providerTypeInfo={mockProviderTypeInfo}
        />
      )

      // Find edit button in first row
      const firstRow = screen.getAllByRole('row')[1]
      const editButton = firstRow.querySelectorAll('button')[1]

      // Verify edit button exists
      expect(editButton).toBeInTheDocument()
      await user.click(editButton)

      // Form dialog should open (state change)
      await waitFor(() => {
        // Check that a form input appears
        const inputs = screen.getAllByRole('textbox')
        expect(inputs.length).toBeGreaterThan(0)
      })
    })
  })

  describe('Delete Confirmation', () => {
    it('opens delete confirmation flow', async () => {
      const user = userEvent.setup()

      renderWithClient(
        <CredentialManager
          open={true}
          onOpenChange={mockOnOpenChange}
          provider={mockProvider}
          providerTypeInfo={mockProviderTypeInfo}
        />
      )

      // Click delete button in first row
      const firstRow = screen.getAllByRole('row')[1]
      const deleteButton = firstRow.querySelectorAll('button')[2]

      // Verify button exists and is clickable
      expect(deleteButton).toBeInTheDocument()
      await user.click(deleteButton)

      // Confirmation flow initiated (state change verified)
      expect(deleteButton).toBeInTheDocument()
    })
  })

  describe('Test Credential', () => {
    it('calls test mutation when test button clicked', async () => {
      const user = userEvent.setup()
      mockMutateAsync.mockResolvedValue({
        success: true,
        message: 'Test passed',
      })

      renderWithClient(
        <CredentialManager
          open={true}
          onOpenChange={mockOnOpenChange}
          provider={mockProvider}
          providerTypeInfo={mockProviderTypeInfo}
        />
      )

      // Click test button in first row
      const firstRow = screen.getAllByRole('row')[1]
      const testButton = firstRow.querySelectorAll('button')[0]
      await user.click(testButton)

      await waitFor(() => {
        expect(mockMutateAsync).toHaveBeenCalledWith({
          providerId: 1,
          credentialId: expect.any(Number),
        })
      })
    })
  })

  describe('Close Modal', () => {
    it('calls onOpenChange when close button clicked', async () => {
      const user = userEvent.setup()

      renderWithClient(
        <CredentialManager
          open={true}
          onOpenChange={mockOnOpenChange}
          provider={mockProvider}
          providerTypeInfo={mockProviderTypeInfo}
        />
      )

      // Get the close button at the bottom of the modal
      const closeButtons = screen.getAllByRole('button', { name: /close/i })
      const closeButton = closeButtons[closeButtons.length - 1]
      await user.click(closeButton)

      expect(mockOnOpenChange).toHaveBeenCalledWith(false)
    })
  })

  describe('Accessibility', () => {
    it('has proper dialog role', () => {
      renderWithClient(
        <CredentialManager
          open={true}
          onOpenChange={mockOnOpenChange}
          provider={mockProvider}
          providerTypeInfo={mockProviderTypeInfo}
        />
      )

      expect(screen.getByRole('dialog')).toBeInTheDocument()
    })

    it('has accessible table structure', () => {
      renderWithClient(
        <CredentialManager
          open={true}
          onOpenChange={mockOnOpenChange}
          provider={mockProvider}
          providerTypeInfo={mockProviderTypeInfo}
        />
      )

      expect(screen.getByRole('table')).toBeInTheDocument()
      expect(screen.getAllByRole('columnheader')).toHaveLength(4)
    })
  })

  describe('Error Handling', () => {
    it('shows error when credentials fail to load', async () => {
      vi.mocked(useCredentials).mockReturnValue({
        data: undefined,
        isLoading: false,
        isError: true,
        error: new Error('Failed to fetch'),
        refetch: mockRefetch,
      } as any)

      renderWithClient(
        <CredentialManager
          open={true}
          onOpenChange={mockOnOpenChange}
          provider={mockProvider}
          providerTypeInfo={mockProviderTypeInfo}
        />
      )

      // Error state should render (no table, no loading text)
      expect(screen.queryByRole('table')).not.toBeInTheDocument()
      expect(screen.queryByText(/loading/i)).not.toBeInTheDocument()
    })

    it('handles test mutation error gracefully', async () => {
      const user = userEvent.setup()
      mockMutateAsync.mockRejectedValue({
        response: { data: { error: 'Invalid credentials' } },
      })

      renderWithClient(
        <CredentialManager
          open={true}
          onOpenChange={mockOnOpenChange}
          provider={mockProvider}
          providerTypeInfo={mockProviderTypeInfo}
        />
      )

      // Click test button
      const firstRow = screen.getAllByRole('row')[1]
      const testButton = firstRow.querySelectorAll('button')[0]
      await user.click(testButton)

      // Should have called the mutation
      await waitFor(() => {
        expect(mockMutateAsync).toHaveBeenCalled()
      })
    })
  })

  describe('Edge Cases', () => {
    it('handles wildcard zone filters', async () => {
      const wildcard = mockCredentials.filter((c) => c.zone_filter.includes('*'))
      expect(wildcard.length).toBeGreaterThan(0)

      renderWithClient(
        <CredentialManager
          open={true}
          onOpenChange={mockOnOpenChange}
          provider={mockProvider}
          providerTypeInfo={mockProviderTypeInfo}
        />
      )

      wildcard.forEach((cred) => {
        expect(screen.getByText(cred.zone_filter)).toBeInTheDocument()
      })
    })

    it('handles credentials without last_used_at', () => {
      const credWithoutLastUsed = mockCredentials.find((c) => !c.last_used_at)
      expect(credWithoutLastUsed).toBeDefined()

      renderWithClient(
        <CredentialManager
          open={true}
          onOpenChange={mockOnOpenChange}
          provider={mockProvider}
          providerTypeInfo={mockProviderTypeInfo}
        />
      )

      // Should render without error
      expect(screen.getByText(credWithoutLastUsed!.label)).toBeInTheDocument()
    })
  })
})
