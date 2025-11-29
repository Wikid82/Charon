import { describe, it, expect, vi, afterEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
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

  it('calls onCancel when cancel button is clicked', async () => {
    render(
      <RemoteServerForm onSubmit={mockOnSubmit} onCancel={mockOnCancel} />
    )

    await userEvent.click(screen.getByText('Cancel'))
    expect(mockOnCancel).toHaveBeenCalledTimes(1)
  })

  it('submits form with correct data', async () => {
    render(
      <RemoteServerForm onSubmit={mockOnSubmit} onCancel={mockOnCancel} />
    )

    const nameInput = screen.getByPlaceholderText('My Production Server')
    const hostInput = screen.getByPlaceholderText('192.168.1.100')
    const portInput = screen.getByDisplayValue('22')

    await userEvent.clear(nameInput)
    await userEvent.type(nameInput, 'New Server')
    await userEvent.clear(hostInput)
    await userEvent.type(hostInput, '10.0.0.5')
    await userEvent.clear(portInput)
    await userEvent.type(portInput, '9090')

    await userEvent.click(screen.getByText('Create'))

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

  it('handles provider selection', async () => {
    render(
      <RemoteServerForm onSubmit={mockOnSubmit} onCancel={mockOnCancel} />
    )

    const providerSelect = screen.getByDisplayValue('Generic')
    await userEvent.selectOptions(providerSelect, 'docker')

    expect(providerSelect).toHaveValue('docker')
  })

  it('handles submission error', async () => {
    const mockErrorSubmit = vi.fn(() => Promise.reject(new Error('Submission failed')))
    render(
      <RemoteServerForm onSubmit={mockErrorSubmit} onCancel={mockOnCancel} />
    )

    // Fill required fields
    await userEvent.clear(screen.getByPlaceholderText('My Production Server'))
    await userEvent.type(screen.getByPlaceholderText('My Production Server'), 'Test Server')
    await userEvent.clear(screen.getByPlaceholderText('192.168.1.100'))
    await userEvent.type(screen.getByPlaceholderText('192.168.1.100'), '10.0.0.1')

    await userEvent.click(screen.getByText('Create'))

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
    await userEvent.click(testButton)

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

    await userEvent.click(screen.getByText('Test Connection'))

    await waitFor(() => {
      expect(screen.getByText('Connection failed')).toBeInTheDocument()
    })
  })
})
