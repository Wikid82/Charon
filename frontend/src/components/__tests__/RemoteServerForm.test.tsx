import { describe, it, expect, vi, afterEach } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import RemoteServerForm from '../RemoteServerForm'
import * as remoteServersApi from '../../api/remoteServers'

// Mock the API
vi.mock('../../api/remoteServers', () => ({
  testRemoteServerConnection: vi.fn(() => Promise.resolve({ address: 'localhost:8080' })),
  testCustomRemoteServerConnection: vi.fn(() => Promise.resolve({ address: 'localhost:8080', reachable: true })),
}))

describe('RemoteServerForm', () => {
  const mockOnSubmit = vi.fn(() => Promise.resolve())
  const mockOnCancel = vi.fn()

  afterEach(() => {
    vi.clearAllMocks()
  })

  it('renders create form', () => {
    render(
      <RemoteServerForm onSubmit={mockOnSubmit} onCancel={mockOnCancel} />
    )

    expect(screen.getByText('Add Remote Server')).toBeInTheDocument()
    expect(screen.getByPlaceholderText('My Production Server')).toHaveValue('')
  })

  it('renders edit form with pre-filled data', () => {
    const mockServer = {
      uuid: '123',
      name: 'Test Server',
      provider: 'docker',
      host: 'localhost',
      port: 5000,
      username: 'admin',
      enabled: true,
      reachable: true,
      created_at: '2025-11-18T10:00:00Z',
      updated_at: '2025-11-18T10:00:00Z',
    }

    render(
      <RemoteServerForm server={mockServer} onSubmit={mockOnSubmit} onCancel={mockOnCancel} />
    )

    expect(screen.getByText('Edit Remote Server')).toBeInTheDocument()
    expect(screen.getByDisplayValue('Test Server')).toBeInTheDocument()
    expect(screen.getByDisplayValue('localhost')).toBeInTheDocument()
    expect(screen.getByDisplayValue('5000')).toBeInTheDocument()
  })

  it('shows test connection button in create and edit mode', () => {
    const { rerender } = render(
      <RemoteServerForm onSubmit={mockOnSubmit} onCancel={mockOnCancel} />
    )

    expect(screen.getByText('Test Connection')).toBeInTheDocument()

    const mockServer = {
      uuid: '123',
      name: 'Test Server',
      provider: 'docker',
      host: 'localhost',
      port: 5000,
      enabled: true,
      reachable: false,
      created_at: '2025-11-18T10:00:00Z',
      updated_at: '2025-11-18T10:00:00Z',
    }

    rerender(
      <RemoteServerForm server={mockServer} onSubmit={mockOnSubmit} onCancel={mockOnCancel} />
    )

    expect(screen.getByText('Test Connection')).toBeInTheDocument()
  })

  it('calls onCancel when cancel button is clicked', () => {
    render(
      <RemoteServerForm onSubmit={mockOnSubmit} onCancel={mockOnCancel} />
    )

    fireEvent.click(screen.getByText('Cancel'))
    expect(mockOnCancel).toHaveBeenCalledOnce()
  })

  it('submits form with correct data', async () => {
    render(
      <RemoteServerForm onSubmit={mockOnSubmit} onCancel={mockOnCancel} />
    )

    const nameInput = screen.getByPlaceholderText('My Production Server')
    const hostInput = screen.getByPlaceholderText('192.168.1.100')
    const portInput = screen.getByDisplayValue('22')

    fireEvent.change(nameInput, { target: { value: 'New Server' } })
    fireEvent.change(hostInput, { target: { value: '10.0.0.5' } })
    fireEvent.change(portInput, { target: { value: '9090' } })

    fireEvent.click(screen.getByText('Create'))

    await waitFor(() => {
      expect(mockOnSubmit).toHaveBeenCalledWith(
        expect.objectContaining({
          name: 'New Server',
          host: '10.0.0.5',
          port: 9090,
        })
      )
    })
  })

  it('handles provider selection', () => {
    render(
      <RemoteServerForm onSubmit={mockOnSubmit} onCancel={mockOnCancel} />
    )

    const providerSelect = screen.getByDisplayValue('Generic')
    fireEvent.change(providerSelect, { target: { value: 'docker' } })

    expect(providerSelect).toHaveValue('docker')
  })

  it('handles submission error', async () => {
    const mockErrorSubmit = vi.fn(() => Promise.reject(new Error('Submission failed')))
    render(
      <RemoteServerForm onSubmit={mockErrorSubmit} onCancel={mockOnCancel} />
    )

    // Fill required fields
    fireEvent.change(screen.getByPlaceholderText('My Production Server'), { target: { value: 'Test Server' } })
    fireEvent.change(screen.getByPlaceholderText('192.168.1.100'), { target: { value: '10.0.0.1' } })

    fireEvent.click(screen.getByText('Create'))

    await waitFor(() => {
      expect(screen.getByText('Submission failed')).toBeInTheDocument()
    })
  })

  it('handles test connection success', async () => {
    const mockServer = {
      uuid: '123',
      name: 'Test Server',
      provider: 'docker',
      host: 'localhost',
      port: 5000,
      enabled: true,
      reachable: true,
      created_at: '2025-11-18T10:00:00Z',
      updated_at: '2025-11-18T10:00:00Z',
    }

    render(
      <RemoteServerForm server={mockServer} onSubmit={mockOnSubmit} onCancel={mockOnCancel} />
    )

    const testButton = screen.getByText('Test Connection')
    fireEvent.click(testButton)

    await waitFor(() => {
      // Check for success state (green background)
      expect(testButton).toHaveClass('bg-green-600')
    })
  })

  it('handles test connection failure', async () => {
    // Override mock for this test
    vi.mocked(remoteServersApi.testCustomRemoteServerConnection).mockRejectedValueOnce(new Error('Connection failed'))

    const mockServer = {
      uuid: '123',
      name: 'Test Server',
      provider: 'docker',
      host: 'localhost',
      port: 5000,
      enabled: true,
      reachable: true,
      created_at: '2025-11-18T10:00:00Z',
      updated_at: '2025-11-18T10:00:00Z',
    }

    render(
      <RemoteServerForm server={mockServer} onSubmit={mockOnSubmit} onCancel={mockOnCancel} />
    )

    fireEvent.click(screen.getByText('Test Connection'))

    await waitFor(() => {
      expect(screen.getByText('Connection failed')).toBeInTheDocument()
    })
  })
})
