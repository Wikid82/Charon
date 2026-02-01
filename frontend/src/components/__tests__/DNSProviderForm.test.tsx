import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import DNSProviderForm from '../DNSProviderForm'
import { defaultProviderSchemas } from '../../data/dnsProviderSchemas'

// Mock hooks used by DNSProviderForm
vi.mock('../../hooks/useDNSProviders', () => ({
  useDNSProviderTypes: vi.fn(() => ({ data: [defaultProviderSchemas.script], isLoading: false })),
  useDNSProviderMutations: vi.fn(() => ({ createMutation: { isPending: false }, updateMutation: { isPending: false }, testCredentialsMutation: { isPending: false } })),
}))
vi.mock('../../hooks/useCredentials', () => ({
  useCredentials: vi.fn(() => ({ data: [] })),
  useEnableMultiCredentials: vi.fn(() => ({ mutate: vi.fn(), isPending: false }))
}))

const renderWithClient = (ui: React.ReactElement) => {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(<QueryClientProvider client={queryClient}>{ui}</QueryClientProvider>)
}

describe('DNSProviderForm — Script provider (accessibility)', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('renders `Script Path` input when Script provider is selected (add flow)', async () => {
    renderWithClient(<DNSProviderForm open={true} onOpenChange={() => {}} provider={null} onSuccess={() => {}} />)

    // Open provider selector and choose the script provider
    const select = screen.getByLabelText(/provider type/i)
    await userEvent.click(select)

    const scriptOption = await screen.findByRole('option', { name: /script|custom script/i })
    await userEvent.click(scriptOption)

    // The input should be present, labelled "Script Path", have the expected placeholder and be required (add flow)
    const scriptInput = await screen.findByRole('textbox', { name: /script path/i })
    expect(scriptInput).toBeInTheDocument()
    expect(scriptInput).toHaveAttribute('placeholder', expect.stringMatching(/dns-challenge\.sh/i))
    expect(scriptInput).toBeRequired()

    // Keyboard focus works
    scriptInput.focus()
    await waitFor(() => expect(scriptInput).toHaveFocus())
  })

  it('renders Script Path when editing an existing script provider (not required)', async () => {
    const existingProvider = {
      id: 1,
      uuid: 'p-1',
      name: 'local-script',
      provider_type: 'script',
      enabled: true,
      is_default: false,
      has_credentials: true,
      propagation_timeout: 120,
      polling_interval: 5,
      success_count: 0,
      failure_count: 0,
      created_at: new Date().toISOString(),
      updated_at: new Date().toISOString(),
    }

    renderWithClient(
      <DNSProviderForm open={true} onOpenChange={() => {}} provider={existingProvider as any} onSuccess={() => {}} />
    )

    // Since provider prop is provided, providerType should be pre-populated and the field rendered
    const scriptInput = await screen.findByRole('textbox', { name: /script path/i })
    expect(scriptInput).toBeInTheDocument()
    // Not required when editing
    expect(scriptInput).not.toBeRequired()
  })
})
