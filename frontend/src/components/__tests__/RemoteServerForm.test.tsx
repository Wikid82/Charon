import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor, fireEvent, act } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router-dom'
import { describe, it, expect, vi, afterEach } from 'vitest'

import * as remoteServersApi from '../../api/remoteServers'
import * as useOrthrusHook from '../../hooks/useOrthrus'
import RemoteServerForm from '../RemoteServerForm'

// Mock the API
vi.mock('../../api/remoteServers', () => ({
  testRemoteServerConnection: vi.fn(() => Promise.resolve({ address: 'localhost:8080' })),
  testCustomRemoteServerConnection: vi.fn(() => Promise.resolve({ address: 'localhost:8080', reachable: true })),
}))

vi.mock('../../hooks/useHecate', () => ({
  useHecate: vi.fn(() => ({
    tunnels: [],
    isLoading: false,
    getStatus: vi.fn(),
  })),
}))

vi.mock('../../hooks/useOrthrus', () => ({
  useAgentList: vi.fn(() => ({ data: [] })),
  useProvisionAgent: vi.fn(() => ({
    mutateAsync: vi.fn(() => Promise.resolve({ agent: { uuid: 'agent-uuid', name: 'agent' }, auth_key: 'auth-key' })),
  })),
  useOrthrus: vi.fn(() => ({
    getInstallSnippets: vi.fn(() => Promise.resolve({})),
  })),
}))

// Capture provider-mode callbacks so tests can drive them directly
let capturedTunnelSelect: ((uuid: string, provider: string) => void) | undefined
let capturedDeviceSelect: ((deviceId: string, address: string) => void) | undefined

vi.mock('../hecate/ProviderDevicePicker', () => ({
  ProviderDevicePicker: (props: {
    onTunnelSelect?: (uuid: string, provider: string) => void
    onDeviceSelect?: (deviceId: string, address: string) => void
  }) => {
    capturedTunnelSelect = props.onTunnelSelect
    capturedDeviceSelect = props.onDeviceSelect
    return <div data-testid="provider-device-picker" />
  },
}))

vi.mock('../hecate/OrthrusInstallWizard', () => ({
  OrthrusInstallWizard: ({ open, onClose }: { open: boolean; onClose: () => void }) =>
    open ? <div data-testid="orthrus-install-wizard"><button onClick={onClose}>CloseWizard</button></div> : null,
}))

vi.mock('../hecate/CloudflareTunnelWizard', () => ({
  CloudflareTunnelWizard: ({ onCancel }: { onCancel: () => void }) =>
    <div data-testid="cloudflare-tunnel-wizard"><button onClick={onCancel}>CancelWizard</button></div>,
}))

describe('RemoteServerForm', () => {
  const mockOnSubmit = vi.fn(() => Promise.resolve())
  const mockOnCancel = vi.fn()

  function renderForm(props: { server?: Parameters<typeof RemoteServerForm>[0]['server']; onSubmit?: typeof mockOnSubmit; onCancel?: typeof mockOnCancel } = {}) {
    const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    return render(
      <MemoryRouter>
        <QueryClientProvider client={qc}>
          <RemoteServerForm
            onSubmit={props.onSubmit ?? mockOnSubmit}
            onCancel={props.onCancel ?? mockOnCancel}
            server={props.server}
          />
        </QueryClientProvider>
      </MemoryRouter>
    )
  }

  afterEach(() => {
    vi.clearAllMocks()
    capturedTunnelSelect = undefined
    capturedDeviceSelect = undefined
  })

  it('renders create form', () => {
    renderForm()

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

    renderForm({ server: mockServer })

    expect(screen.getByText('Edit Remote Server')).toBeInTheDocument()
    expect(screen.getByDisplayValue('Test Server')).toBeInTheDocument()
    expect(screen.getByDisplayValue('localhost')).toBeInTheDocument()
    expect(screen.getByDisplayValue('5000')).toBeInTheDocument()
  })

  it('shows test connection button in create and edit mode', () => {
    const { unmount } = renderForm()
    expect(screen.getByText('Test Connection')).toBeInTheDocument()
    unmount()

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

    renderForm({ server: mockServer })
    expect(screen.getByText('Test Connection')).toBeInTheDocument()
  })

  it('calls onCancel when cancel button is clicked', async () => {
    renderForm()

    await userEvent.click(screen.getByText('Cancel'))
    expect(mockOnCancel).toHaveBeenCalledTimes(1)
  })

  it('submits form with correct data', async () => {
    renderForm()

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
    renderForm()

    const providerSelect = screen.getByDisplayValue('Generic')
    await userEvent.selectOptions(providerSelect, 'docker')

    expect(providerSelect).toHaveValue('docker')
  })

  it('handles submission error', async () => {
    const mockErrorSubmit = vi.fn(() => Promise.reject(new Error('Submission failed')))
    renderForm({ onSubmit: mockErrorSubmit })

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

    renderForm({ server: mockServer })

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

    renderForm({ server: mockServer })

    await userEvent.click(screen.getByText('Test Connection'))

    await waitFor(() => {
      expect(screen.getByText('Connection failed')).toBeInTheDocument()
    })
  })

  it('calls onCancel when Escape key is pressed on the overlay', () => {
    renderForm()

    // Background overlay has role="button", tabIndex={-1}, and aria-label="Cancel"
    const overlay = screen.getAllByRole('button', { name: /cancel/i })
      .find(el => el.getAttribute('tabindex') === '-1')
    expect(overlay).toBeTruthy()

    fireEvent.keyDown(overlay!, { key: 'Escape' })

    expect(mockOnCancel).toHaveBeenCalled()
  })

  it('switches to agent mode and shows agent select', async () => {
    vi.mocked(useOrthrusHook.useAgentList).mockReturnValue({
      data: [
        { uuid: 'agent-1', name: 'Agent One', status: 'online', capabilities: '[]', last_heartbeat: null, last_seen: null, created_at: '', updated_at: '' },
      ],
    } as unknown as ReturnType<typeof useOrthrusHook.useAgentList>)

    renderForm()

    const agentRadio = screen.getByRole('radio', { name: /agent/i })
    await userEvent.click(agentRadio)

    await waitFor(() => {
      expect(document.getElementById('cts-agent')).not.toBeNull()
    })
  })

  it('selects an orthrus agent from the agent select', async () => {
    vi.mocked(useOrthrusHook.useAgentList).mockReturnValue({
      data: [
        { uuid: 'agent-1', name: 'Agent One', status: 'online', capabilities: '[]', last_heartbeat: null, last_seen: null, created_at: '', updated_at: '' },
      ],
    } as unknown as ReturnType<typeof useOrthrusHook.useAgentList>)

    renderForm()

    await userEvent.click(screen.getByRole('radio', { name: /agent/i }))

    const agentSelect = await waitFor(() => {
      const el = document.getElementById('cts-agent') as HTMLSelectElement
      expect(el).not.toBeNull()
      return el
    })

    await userEvent.selectOptions(agentSelect, 'agent-1')
    expect(agentSelect.value).toBe('agent-1')
  })

  it('test connection failure with non-reachable result shows error', async () => {
    vi.mocked(remoteServersApi.testCustomRemoteServerConnection).mockResolvedValueOnce({
      reachable: false,
      error: 'Timeout',
      address: '',
    })

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

    renderForm({ server: mockServer })

    await userEvent.click(screen.getByText('Test Connection'))

    await waitFor(() => {
      expect(screen.getByText(/Connection failed.*Timeout/)).toBeInTheDocument()
    })
  })

  it('agent mode: submits with agent.resolved_address as host', async () => {
    vi.mocked(useOrthrusHook.useAgentList).mockReturnValue({
      data: [
        {
          uuid: 'agent-1',
          name: 'Agent One',
          status: 'online',
          capabilities: '[]',
          last_heartbeat: null,
          last_seen: null,
          created_at: '',
          updated_at: '',
          resolved_address: '100.64.0.5',
        },
      ],
    } as unknown as ReturnType<typeof useOrthrusHook.useAgentList>)

    const agentServer = {
      uuid: '1',
      name: 'Agent Server',
      provider: 'generic',
      host: '',
      port: 22,
      enabled: true,
      reachable: false,
      connection_type: 'orthrus' as const,
      orthrus_agent_uuid: 'agent-1',
      created_at: '2025-01-01T00:00:00Z',
      updated_at: '2025-01-01T00:00:00Z',
    }

    renderForm({ server: agentServer })

    await userEvent.click(screen.getByText('Update'))

    await waitFor(() => {
      expect(mockOnSubmit).toHaveBeenCalledWith(
        expect.objectContaining({
          host: '100.64.0.5',
          connection_type: 'orthrus',
        })
      )
    })
  })

  it('agent mode: falls back to formData.host when agent has no resolved_address', async () => {
    vi.mocked(useOrthrusHook.useAgentList).mockReturnValue({
      data: [
        {
          uuid: 'agent-2',
          name: 'Agent Two',
          status: 'online',
          capabilities: '[]',
          last_heartbeat: null,
          last_seen: null,
          created_at: '',
          updated_at: '',
        },
      ],
    } as unknown as ReturnType<typeof useOrthrusHook.useAgentList>)

    const agentServer = {
      uuid: '2',
      name: 'Fallback Server',
      provider: 'generic',
      host: 'fallback-host',
      port: 22,
      enabled: true,
      reachable: false,
      connection_type: 'orthrus' as const,
      orthrus_agent_uuid: 'agent-2',
      created_at: '2025-01-01T00:00:00Z',
      updated_at: '2025-01-01T00:00:00Z',
    }

    renderForm({ server: agentServer })

    await userEvent.click(screen.getByText('Update'))

    await waitFor(() => {
      expect(mockOnSubmit).toHaveBeenCalledWith(
        expect.objectContaining({
          host: 'fallback-host',
          connection_type: 'orthrus',
        })
      )
    })
  })

  it('provider mode (Tailscale): submits with resolved_address and hecate_tunnel_uuid', async () => {
    renderForm()

    await userEvent.click(screen.getByRole('radio', { name: /provider/i }))

    await waitFor(() => {
      expect(capturedTunnelSelect).toBeDefined()
      expect(capturedDeviceSelect).toBeDefined()
    })

    await act(async () => {
      capturedTunnelSelect!('abc-123', 'tailscale')
      capturedDeviceSelect!('device-1', '100.64.1.1')
    })

    await userEvent.clear(screen.getByPlaceholderText('My Production Server'))
    await userEvent.type(screen.getByPlaceholderText('My Production Server'), 'Tailscale Server')

    await userEvent.click(screen.getByText('Create'))

    await waitFor(() => {
      expect(mockOnSubmit).toHaveBeenCalledWith(
        expect.objectContaining({
          host: '100.64.1.1',
          connection_type: 'tailscale',
          hecate_tunnel_uuid: 'abc-123',
        })
      )
    })
  })

  it('provider mode (Cloudflare): submits hostname as host', async () => {
    renderForm()

    await userEvent.click(screen.getByRole('radio', { name: /provider/i }))

    await waitFor(() => {
      expect(capturedTunnelSelect).toBeDefined()
      expect(capturedDeviceSelect).toBeDefined()
    })

    await act(async () => {
      capturedTunnelSelect!('cf-uuid', 'cloudflare')
      capturedDeviceSelect!('', 'app.example.com')
    })

    await userEvent.clear(screen.getByPlaceholderText('My Production Server'))
    await userEvent.type(screen.getByPlaceholderText('My Production Server'), 'Cloudflare Server')

    await userEvent.click(screen.getByText('Create'))

    await waitFor(() => {
      expect(mockOnSubmit).toHaveBeenCalledWith(
        expect.objectContaining({
          host: 'app.example.com',
          connection_type: 'cloudflare',
          hecate_tunnel_uuid: 'cf-uuid',
        })
      )
    })
  })

  it('direct mode: submits formData.host without orthrus_agent_uuid', async () => {
    renderForm()

    await userEvent.clear(screen.getByPlaceholderText('My Production Server'))
    await userEvent.type(screen.getByPlaceholderText('My Production Server'), 'Direct Server')
    await userEvent.clear(screen.getByPlaceholderText('192.168.1.100'))
    await userEvent.type(screen.getByPlaceholderText('192.168.1.100'), '192.168.1.1')

    await userEvent.click(screen.getByText('Create'))

    await waitFor(() => {
      expect(mockOnSubmit).toHaveBeenCalledWith(
        expect.objectContaining({ host: '192.168.1.1' })
      )
      expect(mockOnSubmit).not.toHaveBeenCalledWith(
        expect.objectContaining({ orthrus_agent_uuid: expect.anything() })
      )
    })
  })

  it('renders agent mode when server has orthrus connection type', () => {
    const mockServer = {
      uuid: '123',
      name: 'Orthrus Server',
      provider: 'generic',
      host: '',
      port: 22,
      username: '',
      enabled: true,
      reachable: false,
      connection_type: 'orthrus' as const,
      orthrus_agent_uuid: 'existing-agent',
      created_at: '2025-01-01T00:00:00Z',
      updated_at: '2025-01-01T00:00:00Z',
    }

    renderForm({ server: mockServer })

    expect(screen.getByRole('radio', { name: /agent/i })).toBeChecked()
    expect(document.getElementById('cts-agent')).not.toBeNull()
  })
})
