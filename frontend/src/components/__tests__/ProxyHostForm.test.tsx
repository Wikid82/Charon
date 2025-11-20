import { describe, it, expect, vi, afterEach } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import ProxyHostForm from '../ProxyHostForm'
import { mockRemoteServers } from '../../test/mockData'

// Mock the hook
vi.mock('../../hooks/useRemoteServers', () => ({
  useRemoteServers: vi.fn(() => ({
    servers: mockRemoteServers,
    isLoading: false,
    error: null,
    createRemoteServer: vi.fn(),
    updateRemoteServer: vi.fn(),
    deleteRemoteServer: vi.fn(),
  })),
}))

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      retry: false,
    },
  },
})

const renderWithClient = (ui: React.ReactElement) => {
  return render(
    <QueryClientProvider client={queryClient}>
      {ui}
    </QueryClientProvider>
  )
}

describe('ProxyHostForm', () => {
  const mockOnSubmit = vi.fn(() => Promise.resolve())
  const mockOnCancel = vi.fn()

  afterEach(() => {
    vi.clearAllMocks()
  })

  it('renders create form with empty fields', async () => {
    renderWithClient(
      <ProxyHostForm onSubmit={mockOnSubmit} onCancel={mockOnCancel} />
    )

    await waitFor(() => {
      expect(screen.getByText('Add Proxy Host')).toBeInTheDocument()
    })
    expect(screen.getByPlaceholderText('example.com, www.example.com')).toHaveValue('')
  })

  it('renders edit form with pre-filled data', async () => {
    const mockHost = {
      uuid: '123',
      domain_names: 'test.com',
      forward_scheme: 'https',
      forward_host: '192.168.1.100',
      forward_port: 8443,
      ssl_forced: true,
      http2_support: true,
      hsts_enabled: true,
      hsts_subdomains: true,
      block_exploits: true,
      websocket_support: false,
      enabled: true,
      created_at: '2025-11-18T10:00:00Z',
      updated_at: '2025-11-18T10:00:00Z',
    }

    renderWithClient(
      <ProxyHostForm host={mockHost} onSubmit={mockOnSubmit} onCancel={mockOnCancel} />
    )

    await waitFor(() => {
      expect(screen.getByText('Edit Proxy Host')).toBeInTheDocument()
    })
    expect(screen.getByDisplayValue('test.com')).toBeInTheDocument()
    expect(screen.getByDisplayValue('192.168.1.100')).toBeInTheDocument()
  })

  it('loads remote servers for quick select', async () => {
    renderWithClient(
      <ProxyHostForm onSubmit={mockOnSubmit} onCancel={mockOnCancel} />
    )

    await waitFor(() => {
      expect(screen.getByText(/Local Docker Registry/)).toBeInTheDocument()
    })
  })

  it('calls onCancel when cancel button is clicked', async () => {
    renderWithClient(
      <ProxyHostForm onSubmit={mockOnSubmit} onCancel={mockOnCancel} />
    )

    await waitFor(() => {
      expect(screen.getByText('Cancel')).toBeInTheDocument()
    })
    fireEvent.click(screen.getByText('Cancel'))
    expect(mockOnCancel).toHaveBeenCalledOnce()
  })

  it('submits form with correct data', async () => {
    renderWithClient(
      <ProxyHostForm onSubmit={mockOnSubmit} onCancel={mockOnCancel} />
    )

    const domainInput = screen.getByPlaceholderText('example.com, www.example.com')
    const hostInput = screen.getByPlaceholderText('192.168.1.100')
    const portInput = screen.getByDisplayValue('80')

    fireEvent.change(domainInput, { target: { value: 'newsite.com' } })
    fireEvent.change(hostInput, { target: { value: '10.0.0.1' } })
    fireEvent.change(portInput, { target: { value: '9000' } })

    fireEvent.click(screen.getByText('Create'))

    await waitFor(() => {
      expect(mockOnSubmit).toHaveBeenCalledWith(
        expect.objectContaining({
          domain_names: 'newsite.com',
          forward_host: '10.0.0.1',
          forward_port: 9000,
        })
      )
    })
  })

  it('handles SSL and WebSocket checkboxes', async () => {
    renderWithClient(
      <ProxyHostForm onSubmit={mockOnSubmit} onCancel={mockOnCancel} />
    )

    await waitFor(() => {
      expect(screen.getByLabelText('Force SSL')).toBeInTheDocument()
    })

    const sslCheckbox = screen.getByLabelText('Force SSL')
    const wsCheckbox = screen.getByLabelText('WebSocket Support')

    expect(sslCheckbox).not.toBeChecked()
    expect(wsCheckbox).not.toBeChecked()

    fireEvent.click(sslCheckbox)
    fireEvent.click(wsCheckbox)

    expect(sslCheckbox).toBeChecked()
    expect(wsCheckbox).toBeChecked()
  })

  it('populates fields when remote server is selected', async () => {
    renderWithClient(
      <ProxyHostForm onSubmit={mockOnSubmit} onCancel={mockOnCancel} />
    )

    await waitFor(() => {
      expect(screen.getByText(/Local Docker Registry/)).toBeInTheDocument()
    })

    const select = screen.getByRole('combobox', { name: /quick select/i })
    fireEvent.change(select, { target: { value: mockRemoteServers[0].uuid } })

    expect(screen.getByDisplayValue(mockRemoteServers[0].host)).toBeInTheDocument()
    expect(screen.getByDisplayValue(mockRemoteServers[0].port)).toBeInTheDocument()
  })

  it('displays error message on submission failure', async () => {
    const mockErrorSubmit = vi.fn(() => Promise.reject(new Error('Submission failed')))
    renderWithClient(
      <ProxyHostForm onSubmit={mockErrorSubmit} onCancel={mockOnCancel} />
    )

    fireEvent.change(screen.getByPlaceholderText('example.com, www.example.com'), {
      target: { value: 'test.com' },
    })
    fireEvent.change(screen.getByPlaceholderText('192.168.1.100'), {
      target: { value: 'localhost' },
    })
    fireEvent.change(screen.getByDisplayValue('80'), {
      target: { value: '8080' },
    })

    fireEvent.click(screen.getByText('Create'))

    await waitFor(() => {
      expect(screen.getByText('Submission failed')).toBeInTheDocument()
    })
  })

  it('handles advanced config input', async () => {
    renderWithClient(
      <ProxyHostForm onSubmit={mockOnSubmit} onCancel={mockOnCancel} />
    )

    const advancedInput = screen.getByLabelText(/Advanced Caddy Config/i)
    fireEvent.change(advancedInput, { target: { value: 'header_up X-Test "True"' } })

    expect(advancedInput).toHaveValue('header_up X-Test "True"')
  })
})
