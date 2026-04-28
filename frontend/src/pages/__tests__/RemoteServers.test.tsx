import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, it, expect, vi, beforeEach } from 'vitest'

import RemoteServers from '../RemoteServers'

vi.mock('../../hooks/useRemoteServers', () => ({
  useRemoteServers: vi.fn(),
}))

vi.mock('../../hooks/useHecate', () => ({
  useHecate: vi.fn(),
}))

vi.mock('../../components/hecate/TunnelLogViewer', () => ({
  TunnelLogViewer: ({ open, serverName, onClose }: { open: boolean; serverName: string; serverUUID: string; onClose: () => void }) =>
    open ? <div data-testid="tunnel-log-viewer"><button onClick={onClose}>Close Viewer</button>{serverName}</div> : null,
}))

vi.mock('../../components/RemoteServerForm', () => ({
  default: ({ onSubmit }: { onSubmit: (data: Record<string, string>) => void; onCancel: () => void }) => (
    <div data-testid="remote-server-form">
      <button onClick={() => onSubmit({ name: 'New Server', host: 'new.host', port: '22', username: 'u' })}>
        Submit
      </button>
    </div>
  ),
}))

import * as remoteServersHook from '../../hooks/useRemoteServers'
import * as hecateHook from '../../hooks/useHecate'
import type { TunnelStatus } from '../../api/hecate'

const mockServer = {
  uuid: 'srv-1',
  name: 'My Server',
  provider: 'cloudflare',
  host: 'my.host',
  port: 22,
  username: 'admin',
  enabled: true,
  reachable: true,
  connection_type: 'cloudflare' as const,
  created_at: '2025-01-01T00:00:00Z',
  updated_at: '2025-01-01T00:00:00Z',
}

const mockDirectServer = {
  uuid: 'srv-2',
  name: 'Direct Server',
  provider: 'direct',
  host: 'direct.host',
  port: 22,
  username: 'admin',
  enabled: true,
  reachable: true,
  connection_type: 'direct' as const,
  created_at: '2025-01-01T00:00:00Z',
  updated_at: '2025-01-01T00:00:00Z',
}

const mockStatus: TunnelStatus = {
  uuid: 'srv-1',
  name: 'My Server',
  provider: 'cloudflare',
  state: 'connected',
  uptime_seconds: 100,
  last_error: '',
}

describe('RemoteServers', () => {
  beforeEach(() => {
    vi.clearAllMocks()

    vi.mocked(remoteServersHook.useRemoteServers).mockReturnValue({
      servers: [mockServer, mockDirectServer],
      loading: false,
      isFetching: false,
      error: null,
      createServer: vi.fn().mockResolvedValue(undefined),
      updateServer: vi.fn().mockResolvedValue(undefined),
      deleteServer: vi.fn().mockResolvedValue(undefined),
      testConnection: vi.fn().mockResolvedValue({ address: 'my.host:22' }),
      isCreating: false,
      isUpdating: false,
      isDeleting: false,
      isTestingConnection: false,
    })

    vi.mocked(hecateHook.useHecate).mockReturnValue({
      tunnels: [],
      statuses: [mockStatus],
      loadingTunnels: false,
      loadingStatus: false,
      error: null,
      tunnelsError: null,
      statusError: null,
      getStatus: (uuid: string) => (uuid === 'srv-1' ? mockStatus : undefined),
      createTunnel: vi.fn(),
      updateTunnel: vi.fn(),
      deleteTunnel: vi.fn(),
      startTunnel: vi.fn(),
      stopTunnel: vi.fn(),
      rotateCredentials: vi.fn(),
      isCreating: false,
      isUpdating: false,
      isDeleting: false,
      isStarting: false,
      isStopping: false,
      isRotating: false,
    })
  })

  it('renders the page with servers', () => {
    render(<RemoteServers />)

    expect(screen.getByText('My Server')).toBeInTheDocument()
    expect(screen.getByText('Direct Server')).toBeInTheDocument()
  })

  it('shows TunnelStatusBadge for non-direct server with known status', () => {
    render(<RemoteServers />)

    expect(screen.getByRole('status')).toBeInTheDocument()
  })

  it('does not show TunnelStatusBadge for direct server', () => {
    vi.mocked(hecateHook.useHecate).mockReturnValue({
      ...vi.mocked(hecateHook.useHecate).mock.results[0]?.value,
      getStatus: () => undefined,
      tunnels: [],
      statuses: [],
      loadingTunnels: false,
      loadingStatus: false,
      error: null,
      tunnelsError: null,
      statusError: null,
      createTunnel: vi.fn(),
      updateTunnel: vi.fn(),
      deleteTunnel: vi.fn(),
      startTunnel: vi.fn(),
      stopTunnel: vi.fn(),
      rotateCredentials: vi.fn(),
      isCreating: false,
      isUpdating: false,
      isDeleting: false,
      isStarting: false,
      isStopping: false,
      isRotating: false,
    })

    vi.mocked(remoteServersHook.useRemoteServers).mockReturnValue({
      servers: [mockDirectServer],
      loading: false,
      isFetching: false,
      error: null,
      createServer: vi.fn(),
      updateServer: vi.fn(),
      deleteServer: vi.fn(),
      testConnection: vi.fn(),
      isCreating: false,
      isUpdating: false,
      isDeleting: false,
      isTestingConnection: false,
    })

    render(<RemoteServers />)

    expect(screen.queryByRole('status')).not.toBeInTheDocument()
  })

  it('shows "View Logs" button for non-direct servers', () => {
    render(<RemoteServers />)

    // aria-label = "View tunnel logs for My Server" (viewLogsFor translation)
    expect(screen.getByRole('button', { name: /tunnel logs.*My Server/i })).toBeInTheDocument()
  })

  it('does not show "View Logs" button for direct servers', () => {
    render(<RemoteServers />)

    expect(screen.queryByRole('button', { name: /tunnel logs.*Direct Server/i })).not.toBeInTheDocument()
  })

  it('clicking "View Logs" opens TunnelLogViewer', async () => {
    render(<RemoteServers />)

    await userEvent.click(screen.getByRole('button', { name: /tunnel logs.*My Server/i }))

    await waitFor(() => {
      expect(screen.getByTestId('tunnel-log-viewer')).toBeInTheDocument()
    })
  })

  it('shows loading skeleton while loading', () => {
    vi.mocked(remoteServersHook.useRemoteServers).mockReturnValue({
      servers: [],
      loading: true,
      isFetching: false,
      error: null,
      createServer: vi.fn(),
      updateServer: vi.fn(),
      deleteServer: vi.fn(),
      testConnection: vi.fn(),
      isCreating: false,
      isUpdating: false,
      isDeleting: false,
      isTestingConnection: false,
    })

    render(<RemoteServers />)

    // In grid mode, SkeletonCard is shown; in list mode, SkeletonTable is shown
    // Either skeleton should be visible
    const skeletons = document.querySelectorAll('[class*="skeleton"], [class*="animate-pulse"]')
    expect(skeletons.length).toBeGreaterThan(0)
  })

  it('shows error alert on error', () => {
    vi.mocked(remoteServersHook.useRemoteServers).mockReturnValue({
      servers: [],
      loading: false,
      isFetching: false,
      error: 'Connection failed',
      createServer: vi.fn(),
      updateServer: vi.fn(),
      deleteServer: vi.fn(),
      testConnection: vi.fn(),
      isCreating: false,
      isUpdating: false,
      isDeleting: false,
      isTestingConnection: false,
    })

    render(<RemoteServers />)

    expect(screen.getByRole('alert')).toBeInTheDocument()
  })

  it('shows empty state when no servers exist', () => {
    vi.mocked(remoteServersHook.useRemoteServers).mockReturnValue({
      servers: [],
      loading: false,
      isFetching: false,
      error: null,
      createServer: vi.fn(),
      updateServer: vi.fn(),
      deleteServer: vi.fn(),
      testConnection: vi.fn(),
      isCreating: false,
      isUpdating: false,
      isDeleting: false,
      isTestingConnection: false,
    })

    render(<RemoteServers />)

    // The empty state renders a heading
    expect(screen.getByRole('heading', { name: /no remote servers/i })).toBeInTheDocument()
  })

  it('opens server form when Add button is clicked', async () => {
    render(<RemoteServers />)

    const addBtn = screen.getByRole('button', { name: /add.*server|new.*server/i })
    await userEvent.click(addBtn)

    await waitFor(() => {
      expect(screen.getByTestId('remote-server-form')).toBeInTheDocument()
    })
  })

  it('toggles between grid and list view', async () => {
    render(<RemoteServers />)

    // Default is grid view — switch to list
    const listViewBtn = screen.getByTitle(/list/i)
    await userEvent.click(listViewBtn)

    // After switching, list view elements appear (DataTable)
    expect(screen.getByText('My Server')).toBeInTheDocument()
  })

  it('shows badge with connection type when status not available', () => {
    vi.mocked(hecateHook.useHecate).mockReturnValue({
      tunnels: [],
      statuses: [],
      loadingTunnels: false,
      loadingStatus: false,
      error: null,
      tunnelsError: null,
      statusError: null,
      getStatus: () => undefined,
      createTunnel: vi.fn(),
      updateTunnel: vi.fn(),
      deleteTunnel: vi.fn(),
      startTunnel: vi.fn(),
      stopTunnel: vi.fn(),
      rotateCredentials: vi.fn(),
      isCreating: false,
      isUpdating: false,
      isDeleting: false,
      isStarting: false,
      isStopping: false,
      isRotating: false,
    })

    render(<RemoteServers />)

    // Grid view shows both the provider badge and the connection_type fallback badge.
    // mockServer has provider='cloudflare' and connection_type='cloudflare', so both
    // badges render "cloudflare". Use getAllByText to handle the multiple matches.
    const cloudflareElements = screen.getAllByText('cloudflare')
    expect(cloudflareElements.length).toBeGreaterThanOrEqual(1)
  })

  it('opens TunnelLogViewer from list view View Logs button', async () => {
    render(<RemoteServers />)

    // Switch to list view
    const listViewBtn = screen.getByTitle(/list/i)
    await userEvent.click(listViewBtn)

    // View Logs button is in the list view DataTable
    const viewLogsBtn = screen.getByRole('button', { name: /tunnel logs.*My Server/i })
    await userEvent.click(viewLogsBtn)

    await waitFor(() => {
      expect(screen.getByTestId('tunnel-log-viewer')).toBeInTheDocument()
    })
  })

  it('closes TunnelLogViewer when onClose is called', async () => {
    render(<RemoteServers />)

    // Open the viewer via the View Logs button
    await userEvent.click(screen.getByRole('button', { name: /tunnel logs.*My Server/i }))

    await waitFor(() => {
      expect(screen.getByTestId('tunnel-log-viewer')).toBeInTheDocument()
    })

    // Close via the mock's Close Viewer button (calls onClose -> setLogsServer(null))
    await userEvent.click(screen.getByRole('button', { name: /close viewer/i }))

    await waitFor(() => {
      expect(screen.queryByTestId('tunnel-log-viewer')).not.toBeInTheDocument()
    })
  })
})
