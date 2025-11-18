import { describe, it, expect, vi, afterEach } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import ProxyHostForm from '../ProxyHostForm'
import { mockRemoteServers } from '../../test/mockData'

// Mock the API
vi.mock('../../services/api', () => ({
  remoteServersAPI: {
    list: vi.fn(() => Promise.resolve(mockRemoteServers)),
  },
}))

describe('ProxyHostForm', () => {
  const mockOnSubmit = vi.fn(() => Promise.resolve())
  const mockOnCancel = vi.fn()

  afterEach(() => {
    vi.clearAllMocks()
  })

  it('renders create form with empty fields', async () => {
    render(
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

    render(
      <ProxyHostForm host={mockHost} onSubmit={mockOnSubmit} onCancel={mockOnCancel} />
    )

    await waitFor(() => {
      expect(screen.getByText('Edit Proxy Host')).toBeInTheDocument()
    })
    expect(screen.getByDisplayValue('test.com')).toBeInTheDocument()
    expect(screen.getByDisplayValue('192.168.1.100')).toBeInTheDocument()
  })

  it('loads remote servers for quick select', async () => {
    render(
      <ProxyHostForm onSubmit={mockOnSubmit} onCancel={mockOnCancel} />
    )

    await waitFor(() => {
      expect(screen.getByText(/Local Docker Registry/)).toBeInTheDocument()
    })
  })

  it('calls onCancel when cancel button is clicked', async () => {
    render(
      <ProxyHostForm onSubmit={mockOnSubmit} onCancel={mockOnCancel} />
    )

    await waitFor(() => {
      expect(screen.getByText('Cancel')).toBeInTheDocument()
    })
    fireEvent.click(screen.getByText('Cancel'))
    expect(mockOnCancel).toHaveBeenCalledOnce()
  })

  it('submits form with correct data', async () => {
    render(
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
    render(
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
})
