import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'

import RedirectionHosts from '../RedirectionHosts'

import type { RedirectionHost } from '../../api/redirectionHosts'

const mockUseRedirectionHosts = vi.fn()
const mockCreateMutateAsync = vi.fn()
const mockUpdateMutateAsync = vi.fn()
const mockDeleteMutateAsync = vi.fn()

vi.mock('../../hooks/useRedirectionHosts', () => ({
  useRedirectionHosts: () => mockUseRedirectionHosts(),
  useCreateRedirectionHost: () => ({ mutateAsync: mockCreateMutateAsync, isPending: false }),
  useUpdateRedirectionHost: () => ({ mutateAsync: mockUpdateMutateAsync, isPending: false }),
  useDeleteRedirectionHost: () => ({ mutateAsync: mockDeleteMutateAsync, isPending: false }),
}))

vi.mock('../../hooks/useCertificates', () => ({
  useCertificates: vi.fn(() => ({ certificates: [], isLoading: false, error: null })),
}))

vi.mock('../../hooks/useDNSProviders', () => ({
  useDNSProviders: vi.fn(() => ({ data: [], isLoading: false })),
}))

vi.mock('react-hot-toast', () => ({
  toast: { success: vi.fn(), error: vi.fn() },
}))

const createQueryClient = () => new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: 0 } } })
const renderWithProviders = () => {
  const qc = createQueryClient()
  return render(
    <QueryClientProvider client={qc}>
      <RedirectionHosts />
    </QueryClientProvider>
  )
}

const sampleHost = (overrides: Partial<RedirectionHost> = {}): RedirectionHost => ({
  uuid: 'rh-1',
  name: 'Old blog redirect',
  domain_names: 'old-blog.example.com',
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
  dns_provider_id: null,
  dns_provider: null,
  use_dns_challenge: false,
  created_at: '2026-01-01T00:00:00Z',
  updated_at: '2026-01-01T00:00:00Z',
  ...overrides,
})

describe('RedirectionHosts page', () => {
  beforeEach(() => {
    mockCreateMutateAsync.mockReset().mockResolvedValue(sampleHost())
    mockUpdateMutateAsync.mockReset().mockResolvedValue(sampleHost())
    mockDeleteMutateAsync.mockReset().mockResolvedValue(undefined)
  })

  afterEach(() => {
    vi.clearAllMocks()
  })

  it('shows a loading skeleton while fetching', () => {
    mockUseRedirectionHosts.mockReturnValue({ data: undefined, isLoading: true })
    renderWithProviders()
    expect(screen.getByRole('heading', { name: /redirection hosts/i })).toBeInTheDocument()
  })

  it('renders an empty state with an Add action when there are no hosts', async () => {
    mockUseRedirectionHosts.mockReturnValue({ data: [], isLoading: false })
    renderWithProviders()

    expect(await screen.findByText(/no redirection hosts/i)).toBeInTheDocument()
    expect(screen.getAllByRole('button', { name: /add redirection host/i }).length).toBeGreaterThan(0)
  })

  it('renders the table with Domain, Target, and Status Code columns and row data', async () => {
    mockUseRedirectionHosts.mockReturnValue({ data: [sampleHost()], isLoading: false })
    renderWithProviders()

    const table = await screen.findByRole('table')
    expect(within(table).getByRole('columnheader', { name: /domain/i })).toBeInTheDocument()
    expect(within(table).getByRole('columnheader', { name: /target/i })).toBeInTheDocument()
    expect(within(table).getByRole('columnheader', { name: /status code/i })).toBeInTheDocument()

    expect(within(table).getByText('old-blog.example.com')).toBeInTheDocument()
    expect(within(table).getByText('https://newblog.example.com')).toBeInTheDocument()
    expect(within(table).getByText(/301/)).toBeInTheDocument()
  })

  it('opens the Add dialog and creates a redirection host', async () => {
    mockUseRedirectionHosts.mockReturnValue({ data: [], isLoading: false })
    renderWithProviders()

    await userEvent.click(screen.getAllByRole('button', { name: /add redirection host/i })[0])

    const dialog = await screen.findByRole('dialog', { name: /add redirection host/i })
    await userEvent.type(within(dialog).getByLabelText(/domain names/i), 'new.example.com')
    await userEvent.type(within(dialog).getByLabelText(/target url/i), 'https://target.example.com')
    await userEvent.click(within(dialog).getByRole('button', { name: /^save$/i }))

    await waitFor(() => {
      expect(mockCreateMutateAsync).toHaveBeenCalledWith(
        expect.objectContaining({ domain_names: 'new.example.com', target_url: 'https://target.example.com' })
      )
    })

    await waitFor(() => {
      expect(screen.queryByRole('dialog', { name: /add redirection host/i })).not.toBeInTheDocument()
    })
  })

  it('opens the Edit dialog prefilled and updates a redirection host', async () => {
    mockUseRedirectionHosts.mockReturnValue({ data: [sampleHost()], isLoading: false })
    renderWithProviders()

    await userEvent.click(await screen.findByRole('button', { name: /^edit redirection host old blog redirect$/i }))

    const dialog = await screen.findByRole('dialog', { name: /edit redirection host/i })
    expect(within(dialog).getByLabelText(/target url/i)).toHaveValue('https://newblog.example.com')

    await userEvent.clear(within(dialog).getByLabelText(/target url/i))
    await userEvent.type(within(dialog).getByLabelText(/target url/i), 'https://updated.example.com')
    await userEvent.click(within(dialog).getByRole('button', { name: /^save$/i }))

    await waitFor(() => {
      expect(mockUpdateMutateAsync).toHaveBeenCalledWith({
        uuid: 'rh-1',
        data: expect.objectContaining({ target_url: 'https://updated.example.com' }),
      })
    })
  })

  it('deletes a redirection host via the row action and confirmation dialog', async () => {
    mockUseRedirectionHosts.mockReturnValue({ data: [sampleHost()], isLoading: false })
    renderWithProviders()

    await userEvent.click(await screen.findByRole('button', { name: /^delete redirection host old blog redirect$/i }))

    const confirmDialog = await screen.findByRole('dialog', { name: /delete redirection host\?/i })
    await userEvent.click(within(confirmDialog).getByRole('button', { name: /^delete$/i }))

    await waitFor(() => {
      expect(mockDeleteMutateAsync).toHaveBeenCalledWith('rh-1')
    })
  })

  it('toggles enabled state directly from the table', async () => {
    mockUseRedirectionHosts.mockReturnValue({ data: [sampleHost({ enabled: true })], isLoading: false })
    renderWithProviders()

    const toggle = await screen.findByRole('checkbox', { name: /disable redirection host old blog redirect/i })
    await userEvent.click(toggle)

    await waitFor(() => {
      expect(mockUpdateMutateAsync).toHaveBeenCalledWith({ uuid: 'rh-1', data: { enabled: false } })
    })
  })
})
