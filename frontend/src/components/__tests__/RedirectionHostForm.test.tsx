import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { act } from 'react'
import { describe, it, expect, vi, afterEach, beforeEach } from 'vitest'

import RedirectionHostForm from '../RedirectionHostForm'

import type { RedirectionHost } from '../../api/redirectionHosts'

vi.mock('../../hooks/useCertificates', () => ({
  useCertificates: vi.fn(() => ({
    certificates: [
      { id: 1, uuid: 'cert-1', name: 'Cert 1', domains: 'example.com', provider: 'letsencrypt', issuer: 'LE', status: 'valid', has_key: true, in_use: true, expires_at: '2027-01-01' },
    ],
    isLoading: false,
    error: null,
  })),
}))

vi.mock('../../hooks/useDNSProviders', () => ({
  useDNSProviders: vi.fn(() => ({
    data: [
      { uuid: 'dns-1', name: 'Cloudflare', provider_type: 'cloudflare', enabled: true, has_credentials: true, is_default: true },
    ],
    isLoading: false,
  })),
}))

const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } })

const renderWithClient = (ui: React.ReactElement) =>
  render(<QueryClientProvider client={queryClient}>{ui}</QueryClientProvider>)

const renderWithClientAct = async (ui: React.ReactElement) => {
  await act(async () => {
    renderWithClient(ui)
  })
}

const selectComboboxOption = async (label: string | RegExp, optionText: string | RegExp) => {
  const trigger = screen.getByRole('combobox', { name: label })
  await userEvent.click(trigger)
  const option = await screen.findByRole('option', { name: optionText })
  await userEvent.click(option)
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

describe('RedirectionHostForm', () => {
  const mockOnSubmit = vi.fn(() => Promise.resolve())
  const mockOnCancel = vi.fn()

  afterEach(() => {
    vi.clearAllMocks()
  })

  it('renders the Add dialog with all expected fields', async () => {
    await renderWithClientAct(<RedirectionHostForm onSubmit={mockOnSubmit} onCancel={mockOnCancel} />)

    expect(screen.getByRole('dialog', { name: /add redirection host/i })).toBeInTheDocument()
    expect(screen.getByLabelText(/^name/i)).toBeInTheDocument()
    expect(screen.getByLabelText(/domain names/i)).toBeInTheDocument()
    expect(screen.getByLabelText(/target url/i)).toBeInTheDocument()
    expect(screen.getByRole('combobox', { name: /status code/i })).toBeInTheDocument()
    expect(screen.getByLabelText(/preserve path/i)).toBeInTheDocument()
  })

  it('renders the Edit dialog title and prefills fields from the host', async () => {
    const host = sampleHost({ name: 'My Redirect', status_code: 308, preserve_path: false })
    await renderWithClientAct(<RedirectionHostForm host={host} onSubmit={mockOnSubmit} onCancel={mockOnCancel} />)

    expect(screen.getByRole('dialog', { name: /edit redirection host/i })).toBeInTheDocument()
    expect(screen.getByLabelText(/^name/i)).toHaveValue('My Redirect')
    expect(screen.getByLabelText(/domain names/i)).toHaveValue('old-blog.example.com')
    expect(screen.getByLabelText(/target url/i)).toHaveValue('https://newblog.example.com')
    expect(screen.getByLabelText(/preserve path/i)).not.toBeChecked()
    expect(screen.getByRole('combobox', { name: /status code/i })).toHaveTextContent(/308.*permanent/i)
  })

  it('offers exactly the four supported status codes with plain-language labels', async () => {
    await renderWithClientAct(<RedirectionHostForm onSubmit={mockOnSubmit} onCancel={mockOnCancel} />)

    await userEvent.click(screen.getByRole('combobox', { name: /status code/i }))

    expect(await screen.findByRole('option', { name: /301.*permanent/i })).toBeInTheDocument()
    expect(screen.getByRole('option', { name: /302.*temporary/i })).toBeInTheDocument()
    expect(screen.getByRole('option', { name: /307.*temporary/i })).toBeInTheDocument()
    expect(screen.getByRole('option', { name: /308.*permanent/i })).toBeInTheDocument()
    expect(screen.queryByRole('option', { name: /^300\b/ })).not.toBeInTheDocument()
    expect(screen.queryByRole('option', { name: /^303\b/ })).not.toBeInTheDocument()
    expect(screen.queryByRole('option', { name: /^304\b/ })).not.toBeInTheDocument()
  })

  it('selecting a status code updates the combobox value', async () => {
    await renderWithClientAct(<RedirectionHostForm onSubmit={mockOnSubmit} onCancel={mockOnCancel} />)

    await selectComboboxOption(/status code/i, /302.*temporary/i)

    expect(screen.getByRole('combobox', { name: /status code/i })).toHaveTextContent(/302.*temporary/i)
  })

  it('toggling Preserve Path off is reflected in the submitted payload', async () => {
    await renderWithClientAct(<RedirectionHostForm onSubmit={mockOnSubmit} onCancel={mockOnCancel} />)

    await userEvent.type(screen.getByLabelText(/domain names/i), 'example.com')
    await userEvent.type(screen.getByLabelText(/target url/i), 'https://elsewhere.example.com')
    await userEvent.click(screen.getByLabelText(/preserve path/i))

    await userEvent.click(screen.getByRole('button', { name: /^save$/i }))

    await waitFor(() => {
      expect(mockOnSubmit).toHaveBeenCalledWith(
        expect.objectContaining({ preserve_path: false, domain_names: 'example.com', target_url: 'https://elsewhere.example.com' })
      )
    })
  })

  it('shows a validation error and does not submit when Domain Names is empty', async () => {
    await renderWithClientAct(<RedirectionHostForm onSubmit={mockOnSubmit} onCancel={mockOnCancel} />)

    await userEvent.type(screen.getByLabelText(/target url/i), 'https://elsewhere.example.com')
    await userEvent.click(screen.getByRole('button', { name: /^save$/i }))

    expect(await screen.findByText(/domain names is required/i)).toBeInTheDocument()
    expect(mockOnSubmit).not.toHaveBeenCalled()
  })

  it('shows a validation error and does not submit when Target URL is empty', async () => {
    await renderWithClientAct(<RedirectionHostForm onSubmit={mockOnSubmit} onCancel={mockOnCancel} />)

    await userEvent.type(screen.getByLabelText(/domain names/i), 'example.com')
    await userEvent.click(screen.getByRole('button', { name: /^save$/i }))

    expect(await screen.findByText(/target url is required/i)).toBeInTheDocument()
    expect(mockOnSubmit).not.toHaveBeenCalled()
  })

  it('submits the full payload with all defaults on create', async () => {
    await renderWithClientAct(<RedirectionHostForm onSubmit={mockOnSubmit} onCancel={mockOnCancel} />)

    await userEvent.type(screen.getByLabelText(/^name/i), 'My Redirect')
    await userEvent.type(screen.getByLabelText(/domain names/i), 'old.example.com')
    await userEvent.type(screen.getByLabelText(/target url/i), 'https://new.example.com')

    await userEvent.click(screen.getByRole('button', { name: /^save$/i }))

    await waitFor(() => {
      expect(mockOnSubmit).toHaveBeenCalledWith({
        name: 'My Redirect',
        domain_names: 'old.example.com',
        target_url: 'https://new.example.com',
        status_code: 301,
        preserve_path: true,
        ssl_forced: true,
        http2_support: true,
        hsts_enabled: false,
        hsts_subdomains: false,
        certificate_id: null,
        dns_provider_id: null,
        use_dns_challenge: false,
      })
    })
    expect(mockOnCancel).toHaveBeenCalled()
  })

  it('displays the server-returned error message when submit rejects (e.g. self-redirect guard)', async () => {
    const failingSubmit = vi.fn().mockRejectedValue(
      new Error("redirect target cannot point back to one of this host's own domains")
    )
    await renderWithClientAct(<RedirectionHostForm onSubmit={failingSubmit} onCancel={mockOnCancel} />)

    await userEvent.type(screen.getByLabelText(/domain names/i), 'self.example.com')
    await userEvent.type(screen.getByLabelText(/target url/i), 'https://self.example.com')
    await userEvent.click(screen.getByRole('button', { name: /^save$/i }))

    expect(
      await screen.findByText(/redirect target cannot point back to one of this host's own domains/i)
    ).toBeInTheDocument()
    expect(mockOnCancel).not.toHaveBeenCalled()
  })

  it('displays the server-returned domain conflict error', async () => {
    const failingSubmit = vi.fn().mockRejectedValue(new Error('domain already in use by another host'))
    await renderWithClientAct(<RedirectionHostForm onSubmit={failingSubmit} onCancel={mockOnCancel} />)

    await userEvent.type(screen.getByLabelText(/domain names/i), 'taken.example.com')
    await userEvent.type(screen.getByLabelText(/target url/i), 'https://elsewhere.example.com')
    await userEvent.click(screen.getByRole('button', { name: /^save$/i }))

    expect(await screen.findByText(/domain already in use by another host/i)).toBeInTheDocument()
  })

  it('requires a DNS provider once "Use DNS Challenge" is enabled', async () => {
    await renderWithClientAct(<RedirectionHostForm onSubmit={mockOnSubmit} onCancel={mockOnCancel} />)

    await userEvent.type(screen.getByLabelText(/domain names/i), 'example.com')
    await userEvent.type(screen.getByLabelText(/target url/i), 'https://elsewhere.example.com')
    await userEvent.click(screen.getByLabelText(/use dns challenge/i))

    expect(await screen.findByText(/dns provider/i)).toBeInTheDocument()

    await userEvent.click(screen.getByRole('button', { name: /^save$/i }))

    expect(await screen.findByText(/dns provider is required/i)).toBeInTheDocument()
    expect(mockOnSubmit).not.toHaveBeenCalled()
  })

  it('calls onCancel when Cancel is clicked', async () => {
    await renderWithClientAct(<RedirectionHostForm onSubmit={mockOnSubmit} onCancel={mockOnCancel} />)

    await userEvent.click(screen.getByRole('button', { name: /^cancel$/i }))
    expect(mockOnCancel).toHaveBeenCalled()
  })

  describe('Certificate selector', () => {
    beforeEach(() => {
      mockOnSubmit.mockClear()
    })

    it('lists available certificates plus the auto-manage option', async () => {
      await renderWithClientAct(<RedirectionHostForm onSubmit={mockOnSubmit} onCancel={mockOnCancel} />)

      await userEvent.click(screen.getByRole('combobox', { name: /certificate/i }))

      expect(await screen.findByRole('option', { name: /let's encrypt/i })).toBeInTheDocument()
      expect(screen.getByRole('option', { name: /cert 1/i })).toBeInTheDocument()
    })
  })
})
