import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { QueryClient, QueryClientProvider, type UseMutationResult } from '@tanstack/react-query'
import CredentialManager from '../CredentialManager'
import {
  useCredentials,
  useCreateCredential,
  useUpdateCredential,
  useDeleteCredential,
  useTestCredential,
} from '../../hooks/useCredentials'

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string, defaultValue?: string) => defaultValue || key,
    i18n: {
      changeLanguage: () => new Promise(() => {}),
    },
  }),
}))
import type { DNSProvider, DNSProviderTypeInfo } from '../../api/dnsProviders'
import type { CredentialRequest, CredentialTestResult, DNSProviderCredential } from '../../api/credentials'

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
    {
        name: 'email',
        label: 'Email Address',
        type: 'text',
        required: false,
    }
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
]

const createCredentialsQueryResult = (
  overrides: Record<string, unknown> = {}
): ReturnType<typeof useCredentials> => ({
  data: mockCredentials,
  isLoading: false,
  refetch: vi.fn(),
  error: null,
  isError: false,
  isSuccess: true,
  ...overrides,
} as unknown as ReturnType<typeof useCredentials>)

const createMutationResult = <TData, TVariables>(
  mutateAsync: ReturnType<typeof vi.fn>,
  overrides: Partial<UseMutationResult<TData, Error, TVariables, unknown>> = {}
): UseMutationResult<TData, Error, TVariables, unknown> => ({
  mutate: vi.fn() as UseMutationResult<TData, Error, TVariables, unknown>['mutate'],
  mutateAsync: mutateAsync as UseMutationResult<TData, Error, TVariables, unknown>['mutateAsync'],
  isPending: false,
  ...overrides,
} as UseMutationResult<TData, Error, TVariables, unknown>)

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
  const mockCreateMutate = vi.fn()
  const mockUpdateMutate = vi.fn()
  const mockDeleteMutate = vi.fn()
  const mockTestMutate = vi.fn()

  beforeEach(() => {
    vi.clearAllMocks()

    vi.mocked(useCredentials).mockReturnValue(createCredentialsQueryResult())

    vi.mocked(useCreateCredential).mockReturnValue(
      createMutationResult<DNSProviderCredential, { providerId: number; data: CredentialRequest }>(
        mockCreateMutate
      )
    )

    vi.mocked(useUpdateCredential).mockReturnValue(
      createMutationResult<
        DNSProviderCredential,
        { providerId: number; credentialId: number; data: CredentialRequest }
      >(mockUpdateMutate)
    )

    vi.mocked(useDeleteCredential).mockReturnValue(
      createMutationResult<void, { providerId: number; credentialId: number }>(
        mockDeleteMutate
      )
    )

    vi.mocked(useTestCredential).mockReturnValue(
      createMutationResult<CredentialTestResult, { providerId: number; credentialId: number }>(
        mockTestMutate
      )
    )
  })

  // 1. Rendering Checks
  it('renders credentials properly', async () => {
    renderWithClient(
      <CredentialManager
        open={true}
        onOpenChange={mockOnOpenChange}
        provider={mockProvider}
        providerTypeInfo={mockProviderTypeInfo}
      />
    )

    expect(screen.getByText('Manage Credentials: Cloudflare Production')).toBeInTheDocument()
    expect(screen.getByText('Main Zone')).toBeInTheDocument()
    expect(screen.getByText('example.com')).toBeInTheDocument()
  })

  // 2. Add Operation
  it('allows adding a new credential', async () => {
    const user = userEvent.setup()
    renderWithClient(
      <CredentialManager
        open={true}
        onOpenChange={mockOnOpenChange}
        provider={mockProvider}
        providerTypeInfo={mockProviderTypeInfo}
      />
    )

    // Click Add Credential
    await user.click(screen.getByText('Add Credential'))

    // Verify Form opens
    expect(screen.getByRole('dialog', { name: 'Add Credential' })).toBeInTheDocument()

    // Fill Form
    // Label requires *
    await user.type(screen.getByLabelText(/Label/i), 'New Staging')

    // Zone Filter
    await user.type(screen.getByLabelText(/Zone Filter/i), '*.staging.com')

    // Credentials fields from type info
    // API Token (required)
    await user.type(screen.getByLabelText(/API Token/i), 'my-secret-token')

    // Click Save
    await user.click(screen.getByRole('button', { name: 'Save' }))

    // Expect Create Mutation
    await waitFor(() => {
        expect(mockCreateMutate).toHaveBeenCalledWith({
            providerId: 1,
            data: expect.objectContaining({
                label: 'New Staging',
                zone_filter: '*.staging.com',
                credentials: expect.objectContaining({
                    api_token: 'my-secret-token'
                }),
                enabled: true
            })
        })
    })
  })

  // 3. Edit Operation
  it('allows editing an existing credential', async () => {
    const user = userEvent.setup()
    renderWithClient(
      <CredentialManager
        open={true}
        onOpenChange={mockOnOpenChange}
        provider={mockProvider}
        providerTypeInfo={mockProviderTypeInfo}
      />
    )

    // Locate the edit button for the first credential.
    const rows = screen.getAllByRole('row')
    const credRow = rows.find(r => r.innerHTML.includes('Main Zone'))
    const editBtn = credRow?.querySelectorAll('button')[1] // 0=Test, 1=Edit, 2=Delete

    expect(editBtn).toBeDefined()
    await user.click(editBtn!)

    // Verify Form opens with pre-filled values
    expect(screen.getByRole('dialog', { name: 'Edit Credential' })).toBeInTheDocument()
    expect(screen.getByDisplayValue('Main Zone')).toBeInTheDocument()
    expect(screen.getByDisplayValue('example.com')).toBeInTheDocument()

    // Change label
    const labelInput = screen.getByLabelText(/Label/i)
    await user.clear(labelInput)
    await user.type(labelInput, 'Updated Label')

    // Click Save
    await user.click(screen.getByRole('button', { name: 'Save' }))

    // Expect Update Mutation
    await waitFor(() => {
        expect(mockUpdateMutate).toHaveBeenCalledWith({
            providerId: 1,
            credentialId: 1,
            data: expect.objectContaining({
                label: 'Updated Label',
                zone_filter: 'example.com'
            })
        })
    })
  })

  // 4. Delete Operation
  it('allows deleting a credential after confirmation', async () => {
    const user = userEvent.setup()
    renderWithClient(
      <CredentialManager
        open={true}
        onOpenChange={mockOnOpenChange}
        provider={mockProvider}
        providerTypeInfo={mockProviderTypeInfo}
      />
    )

    const rows = screen.getAllByRole('row')
    const credRow = rows.find(r => r.innerHTML.includes('Main Zone'))
    const deleteBtn = credRow?.querySelectorAll('button')[2] // 0=Test, 1=Edit, 2=Delete

    expect(deleteBtn).toBeDefined()
    await user.click(deleteBtn!)

    // Confirmation Dialog
    expect(screen.getByText('Delete Credential?')).toBeInTheDocument()

    // Confirm
    await user.click(screen.getByRole('button', { name: 'Delete' }))

    // Expect Delete Mutation
    await waitFor(() => {
        expect(mockDeleteMutate).toHaveBeenCalledWith({
            providerId: 1,
            credentialId: 1
        })
    })
  })

  it('opens delete confirmation dialog when delete action is clicked', async () => {
    const user = userEvent.setup()

    renderWithClient(
      <CredentialManager
        open={true}
        onOpenChange={mockOnOpenChange}
        provider={mockProvider}
        providerTypeInfo={mockProviderTypeInfo}
      />
    )

    const credentialRow = screen.getByText('Main Zone').closest('tr')
    expect(credentialRow).not.toBeNull()

    const actionButtons = credentialRow?.querySelectorAll('button')
    expect(actionButtons?.[2]).toBeDefined()

    await user.click(actionButtons![2])

    expect(await screen.findByRole('dialog', { name: 'Delete Credential?' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Delete' })).toBeInTheDocument()
  })

  it('closes delete confirmation dialog via dialog close button', async () => {
    const user = userEvent.setup()

    renderWithClient(
      <CredentialManager
        open={true}
        onOpenChange={mockOnOpenChange}
        provider={mockProvider}
        providerTypeInfo={mockProviderTypeInfo}
      />
    )

    const credentialRow = screen.getByText('Main Zone').closest('tr')
    expect(credentialRow).not.toBeNull()

    const actionButtons = credentialRow?.querySelectorAll('button')
    expect(actionButtons?.[2]).toBeDefined()

    await user.click(actionButtons![2])

    const deleteDialog = await screen.findByRole('dialog', { name: 'Delete Credential?' })
    await user.click(within(deleteDialog).getByRole('button', { name: 'Close' }))

    await waitFor(() => {
      expect(screen.queryByRole('dialog', { name: 'Delete Credential?' })).not.toBeInTheDocument()
    })
  })

  // 5. Validation - Required Fields
  it('validates required fields on add', async () => {
    const user = userEvent.setup()
    renderWithClient(
      <CredentialManager
        open={true}
        onOpenChange={mockOnOpenChange}
        provider={mockProvider}
        providerTypeInfo={mockProviderTypeInfo}
      />
    )

    await user.click(screen.getByText('Add Credential'))

    // Click Save without filling anything
    await user.click(screen.getByRole('button', { name: 'Save' }))

    // Mutation should NOT be called.
    expect(mockCreateMutate).not.toHaveBeenCalled()

    // Fill Label but not API Key (which is required by type info)
    await user.type(screen.getByLabelText(/Label/i), 'Incomplete')
    await user.click(screen.getByRole('button', { name: 'Save' }))

    // Still no mutation
    expect(mockCreateMutate).not.toHaveBeenCalled()
  })

  // 6. Validation - Zone Filter Format
  it('validates zone filter format', async () => {
    const user = userEvent.setup()
    renderWithClient(
      <CredentialManager
        open={true}
        onOpenChange={mockOnOpenChange}
        provider={mockProvider}
        providerTypeInfo={mockProviderTypeInfo}
      />
    )

    await user.click(screen.getByText('Add Credential'))

    await user.type(screen.getByLabelText(/Label/i), 'Bad Zone')
    await user.type(screen.getByLabelText(/API Token/i), 'token')

    // Invalid zone
    await user.type(screen.getByLabelText(/Zone Filter/i), 'invalid zone')

    await user.click(screen.getByRole('button', { name: 'Save' }))

    expect(mockCreateMutate).not.toHaveBeenCalled()

    // Fix zone
    const zoneInput = screen.getByLabelText(/Zone Filter/i)
    await user.clear(zoneInput)
    await user.type(zoneInput, 'valid.com')

    await user.click(screen.getByRole('button', { name: 'Save' }))

    await waitFor(() => {
        expect(mockCreateMutate).toHaveBeenCalled()
    })
  })

  // ===== BRANCH COVERAGE EXPANSION TESTS =====

  // 7. Empty Credential List Rendering
  it('renders empty state when no credentials exist', () => {
    vi.mocked(useCredentials).mockReturnValue(
      createCredentialsQueryResult({ data: [] })
    )

    renderWithClient(
      <CredentialManager
        open={true}
        onOpenChange={mockOnOpenChange}
        provider={mockProvider}
        providerTypeInfo={mockProviderTypeInfo}
      />
    )

    expect(screen.getByText(/No credentials configured/i)).toBeInTheDocument()
    expect(screen.getByText(/Add credentials to enable/i)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /Add First Credential/i })).toBeInTheDocument()
  })

  // 8. Loading State
  it('renders loading state while fetching credentials', () => {
    vi.mocked(useCredentials).mockReturnValue(
      createCredentialsQueryResult({
        data: [],
        isLoading: true,
        isSuccess: false,
        status: 'loading',
        fetchStatus: 'fetching',
      })
    )

    renderWithClient(
      <CredentialManager
        open={true}
        onOpenChange={mockOnOpenChange}
        provider={mockProvider}
        providerTypeInfo={mockProviderTypeInfo}
      />
    )

    expect(screen.getByText('Loading...')).toBeInTheDocument()
  })

  // 9. Delete Error Handling
  it('shows error toast when delete fails', async () => {
    const user = userEvent.setup()
    const { toast } = await import('../../utils/toast')

    mockDeleteMutate.mockRejectedValue({
      response: { data: { error: 'Credential in use' } }
    })

    renderWithClient(
      <CredentialManager
        open={true}
        onOpenChange={mockOnOpenChange}
        provider={mockProvider}
        providerTypeInfo={mockProviderTypeInfo}
      />
    )

    const rows = screen.getAllByRole('row')
    const credRow = rows.find(r => r.innerHTML.includes('Main Zone'))
    const deleteBtn = credRow?.querySelectorAll('button')[2]

    await user.click(deleteBtn!)
    await user.click(screen.getByRole('button', { name: 'Delete' }))

    await waitFor(() => {
      expect(toast.error).toHaveBeenCalled()
    })
  })

  // 10. Test Credential - Success
  it('tests credential and shows success', async () => {
    const user = userEvent.setup()
    const { toast } = await import('../../utils/toast')

    mockTestMutate.mockResolvedValue({ success: true, message: 'Test passed' })

    renderWithClient(
      <CredentialManager
        open={true}
        onOpenChange={mockOnOpenChange}
        provider={mockProvider}
        providerTypeInfo={mockProviderTypeInfo}
      />
    )

    const rows = screen.getAllByRole('row')
    const credRow = rows.find(r => r.innerHTML.includes('Main Zone'))
    const testBtn = credRow?.querySelectorAll('button')[0]

    await user.click(testBtn!)

    await waitFor(() => {
      expect(mockTestMutate).toHaveBeenCalledWith({
        providerId: 1,
        credentialId: 1,
      })
      expect(toast.success).toHaveBeenCalled()
    })
  })

  // 11. Test Credential - Failure
  it('tests credential and shows error on failure', async () => {
    const user = userEvent.setup()
    const { toast } = await import('../../utils/toast')

    mockTestMutate.mockResolvedValue({ success: false, error: 'Invalid token' })

    renderWithClient(
      <CredentialManager
        open={true}
        onOpenChange={mockOnOpenChange}
        provider={mockProvider}
        providerTypeInfo={mockProviderTypeInfo}
      />
    )

    const rows = screen.getAllByRole('row')
    const credRow = rows.find(r => r.innerHTML.includes('Main Zone'))
    const testBtn = credRow?.querySelectorAll('button')[0]

    await user.click(testBtn!)

    await waitFor(() => {
      expect(toast.error).toHaveBeenCalled()
    })
  })

  // 12. Test Credential - Exception
  it('handles test credential exception', async () => {
    const user = userEvent.setup()
    const { toast } = await import('../../utils/toast')

    mockTestMutate.mockRejectedValue({ message: 'Network error' })

    renderWithClient(
      <CredentialManager
        open={true}
        onOpenChange={mockOnOpenChange}
        provider={mockProvider}
        providerTypeInfo={mockProviderTypeInfo}
      />
    )

    const rows = screen.getAllByRole('row')
    const credRow = rows.find(r => r.innerHTML.includes('Main Zone'))
    const testBtn = credRow?.querySelectorAll('button')[0]

    await user.click(testBtn!)

    await waitFor(() => {
      expect(toast.error).toHaveBeenCalled()
    })
  })

  // 13. Multiple Credentials
  it('renders multiple credentials in table', () => {
    const multipleCreds = [
      ...mockCredentials,
      {
        id: 2,
        uuid: 'cred-uuid-2',
        dns_provider_id: 1,
        label: 'Staging Zone',
        zone_filter: '*.staging.com',
        enabled: true,
        propagation_timeout: 120,
        polling_interval: 5,
        key_version: 1,
        success_count: 5,
        failure_count: 2,
        last_used_at: undefined,
        last_error: undefined,
        created_at: '2025-01-02T00:00:00Z',
        updated_at: '2025-01-02T00:00:00Z',
      }
    ]

    vi.mocked(useCredentials).mockReturnValue(
      createCredentialsQueryResult({ data: multipleCreds })
    )

    renderWithClient(
      <CredentialManager
        open={true}
        onOpenChange={mockOnOpenChange}
        provider={mockProvider}
        providerTypeInfo={mockProviderTypeInfo}
      />
    )

    expect(screen.getByText('Main Zone')).toBeInTheDocument()
    expect(screen.getByText('Staging Zone')).toBeInTheDocument()
    expect(screen.getByText('*.staging.com')).toBeInTheDocument()
  })

  // 14. Disabled Credential
  it('displays disabled status for disabled credentials', () => {
    const disabledCred = {
      ...mockCredentials[0],
      enabled: false,
    }

    vi.mocked(useCredentials).mockReturnValue(
      createCredentialsQueryResult({ data: [disabledCred] })
    )

    renderWithClient(
      <CredentialManager
        open={true}
        onOpenChange={mockOnOpenChange}
        provider={mockProvider}
        providerTypeInfo={mockProviderTypeInfo}
      />
    )

    expect(screen.getByText('Disabled')).toBeInTheDocument()
  })

  // 15. Credential with Last Error
  it('displays last error when credential has failure', () => {
    const errorCred = {
      ...mockCredentials[0],
      failure_count: 3,
      last_error: 'API rate limit exceeded',
    }

    vi.mocked(useCredentials).mockReturnValue(
      createCredentialsQueryResult({ data: [errorCred] })
    )

    renderWithClient(
      <CredentialManager
        open={true}
        onOpenChange={mockOnOpenChange}
        provider={mockProvider}
        providerTypeInfo={mockProviderTypeInfo}
      />
    )

    expect(screen.getByText('API rate limit exceeded')).toBeInTheDocument()
  })

  // 16. Create Credential Error
  it('shows error toast when create fails', async () => {
    const user = userEvent.setup()
    const { toast } = await import('../../utils/toast')

    mockCreateMutate.mockRejectedValue({
      response: { data: { error: 'Invalid provider' } }
    })

    renderWithClient(
      <CredentialManager
        open={true}
        onOpenChange={mockOnOpenChange}
        provider={mockProvider}
        providerTypeInfo={mockProviderTypeInfo}
      />
    )

    await user.click(screen.getByText('Add Credential'))
    await user.type(screen.getByLabelText(/Label/i), 'Test')
    await user.type(screen.getByLabelText(/API Token/i), 'token')
    await user.click(screen.getByRole('button', { name: 'Save' }))

    await waitFor(() => {
      expect(toast.error).toHaveBeenCalled()
    })
  })

  // 17. Update Credential Error
  it('shows error toast when update fails', async () => {
    const user = userEvent.setup()
    const { toast } = await import('../../utils/toast')

    mockUpdateMutate.mockRejectedValue({
      response: { data: { error: 'Zone filter conflict' } }
    })

    renderWithClient(
      <CredentialManager
        open={true}
        onOpenChange={mockOnOpenChange}
        provider={mockProvider}
        providerTypeInfo={mockProviderTypeInfo}
      />
    )

    const rows = screen.getAllByRole('row')
    const credRow = rows.find(r => r.innerHTML.includes('Main Zone'))
    const editBtn = credRow?.querySelectorAll('button')[1]

    await user.click(editBtn!)
    await user.type(screen.getByLabelText(/Label/i), ' Updated')
    await user.click(screen.getByRole('button', { name: 'Save' }))

    await waitFor(() => {
      expect(toast.error).toHaveBeenCalled()
    })
  })

  // 18. Cancel Delete
  it('cancels delete operation', async () => {
    const user = userEvent.setup()

    renderWithClient(
      <CredentialManager
        open={true}
        onOpenChange={mockOnOpenChange}
        provider={mockProvider}
        providerTypeInfo={mockProviderTypeInfo}
      />
    )

    const rows = screen.getAllByRole('row')
    const credRow = rows.find(r => r.innerHTML.includes('Main Zone'))
    const deleteBtn = credRow?.querySelectorAll('button')[2]

    await user.click(deleteBtn!)
    await user.click(screen.getByRole('button', { name: 'Cancel' }))

    expect(mockDeleteMutate).not.toHaveBeenCalled()
  })

  // 19. Cancel Form Dialog
  it('closes form dialog without saving', async () => {
    const user = userEvent.setup()

    renderWithClient(
      <CredentialManager
        open={true}
        onOpenChange={mockOnOpenChange}
        provider={mockProvider}
        providerTypeInfo={mockProviderTypeInfo}
      />
    )

    await user.click(screen.getByText('Add Credential'))
    await user.type(screen.getByLabelText(/Label/i), 'Test')
    await user.click(screen.getByRole('button', { name: 'Cancel' }))

    expect(mockCreateMutate).not.toHaveBeenCalled()
  })

  // 20. Advanced Options - Propagation Timeout
  it('allows editing propagation timeout in advanced options', async () => {
    const user = userEvent.setup()

    renderWithClient(
      <CredentialManager
        open={true}
        onOpenChange={mockOnOpenChange}
        provider={mockProvider}
        providerTypeInfo={mockProviderTypeInfo}
      />
    )

    await user.click(screen.getByText('Add Credential'))
    await user.type(screen.getByLabelText(/Label/i), 'Advanced Test')
    await user.type(screen.getByLabelText(/API Token/i), 'token')

    // Find and open details element
    const detailsElements = document.querySelectorAll('details')
    const detailsElement = Array.from(detailsElements).find(d =>
      d.textContent?.includes('Advanced Options')
    )

    if (detailsElement) {
      const summary = detailsElement.querySelector('summary')
      if (summary) {
        await user.click(summary)
      }
    }

    // Try to find the propagation timeout input
    const timeoutInputs = document.querySelectorAll('input')
    const timeoutInput = Array.from(timeoutInputs).find(i =>
      i.id === 'propagation_timeout' || i.placeholder?.includes('120')
    ) as HTMLInputElement

    if (timeoutInput) {
      await user.clear(timeoutInput)
      await user.type(timeoutInput, '300')
    }

    await user.click(screen.getByRole('button', { name: 'Save' }))

    await waitFor(() => {
      expect(mockCreateMutate).toHaveBeenCalled()
    })
  })

  // 21. Advanced Options - Polling Interval
  it('allows editing polling interval in advanced options', async () => {
    const user = userEvent.setup()

    renderWithClient(
      <CredentialManager
        open={true}
        onOpenChange={mockOnOpenChange}
        provider={mockProvider}
        providerTypeInfo={mockProviderTypeInfo}
      />
    )

    await user.click(screen.getByText('Add Credential'))
    await user.type(screen.getByLabelText(/Label/i), 'Polling Test')
    await user.type(screen.getByLabelText(/API Token/i), 'token')

    // Find and open details element
    const detailsElements = document.querySelectorAll('details')
    const detailsElement = Array.from(detailsElements).find(d =>
      d.textContent?.includes('Advanced Options')
    )

    if (detailsElement) {
      const summary = detailsElement.querySelector('summary')
      if (summary) {
        await user.click(summary)
      }
    }

    // Try to find the polling interval input
    const inputs = document.querySelectorAll('input')
    const pollingInput = Array.from(inputs).find(i =>
      i.id === 'polling_interval' || (i.type === 'number' && i.placeholder?.includes('5'))
    ) as HTMLInputElement

    if (pollingInput) {
      await user.clear(pollingInput)
      await user.type(pollingInput, '10')
    }

    await user.click(screen.getByRole('button', { name: 'Save' }))

    await waitFor(() => {
      expect(mockCreateMutate).toHaveBeenCalled()
    })
  })

  // 22. Form Success Toast
  it('shows success toast after credential creation', async () => {
    const user = userEvent.setup()
    const { toast } = await import('../../utils/toast')

    mockCreateMutate.mockResolvedValue({ id: 2, label: 'New Cred' })

    renderWithClient(
      <CredentialManager
        open={true}
        onOpenChange={mockOnOpenChange}
        provider={mockProvider}
        providerTypeInfo={mockProviderTypeInfo}
      />
    )

    await user.click(screen.getByText('Add Credential'))
    await user.type(screen.getByLabelText(/Label/i), 'New Cred')
    await user.type(screen.getByLabelText(/API Token/i), 'token')
    await user.click(screen.getByRole('button', { name: 'Save' }))

    await waitFor(() => {
      expect(toast.success).toHaveBeenCalledWith(expect.stringContaining('created successfully'))
    })
  })

  // 23. Form Success Toast - Update
  it('shows success toast after credential update', async () => {
    const user = userEvent.setup()
    const { toast } = await import('../../utils/toast')

    mockUpdateMutate.mockResolvedValue({ id: 1, label: 'Updated' })

    renderWithClient(
      <CredentialManager
        open={true}
        onOpenChange={mockOnOpenChange}
        provider={mockProvider}
        providerTypeInfo={mockProviderTypeInfo}
      />
    )

    const rows = screen.getAllByRole('row')
    const credRow = rows.find(r => r.innerHTML.includes('Main Zone'))
    const editBtn = credRow?.querySelectorAll('button')[1]

    await user.click(editBtn!)
    await user.type(screen.getByLabelText(/Label/i), ' Updated')
    await user.click(screen.getByRole('button', { name: 'Save' }))

    await waitFor(() => {
      expect(toast.success).toHaveBeenCalledWith(expect.stringContaining('updated successfully'))
    })
  })
})
