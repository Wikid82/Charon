import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { describe, it, expect, vi, beforeEach } from 'vitest'

vi.mock('../../hooks/useHecate', () => ({ useHecate: vi.fn() }))
vi.mock('../../hooks/useOrthrus', () => ({
  useAgentList: vi.fn(),
  useProvisionAgent: vi.fn(),
  useOrthrus: vi.fn(),
}))

vi.mock('../../components/hecate/HecateTunnelForm', () => ({
  HecateTunnelForm: ({ open }: { open: boolean }) =>
    open ? <div data-testid="tunnel-form" /> : null,
}))
vi.mock('../../components/hecate/TunnelLogViewer', () => ({
  TunnelLogViewer: ({ open }: { open: boolean }) =>
    open ? <div data-testid="log-viewer" /> : null,
}))
vi.mock('../../components/hecate/TunnelStatusBadge', () => ({
  TunnelStatusBadge: ({ state }: { state: string }) => <span>{state}</span>,
}))
vi.mock('../../components/hecate/OrthrusAgentManager', () => ({
  OrthrusAgentManager: () => <div data-testid="agent-manager" />,
}))
vi.mock('../../components/hecate/OrthrusInstallWizard', () => ({
  OrthrusInstallWizard: ({ open }: { open: boolean }) =>
    open ? <div data-testid="install-wizard" /> : null,
}))

import { useHecate } from '../../hooks/useHecate'
import { useAgentList, useProvisionAgent, useOrthrus } from '../../hooks/useOrthrus'
import Hecate from '../Hecate'

const mockTunnel = {
  uuid: 'tunnel-1',
  name: 'Test Tunnel',
  provider: 'cloudflare' as const,
  configuration: '{}' as string,
  is_active: true,
  created_at: '2024-01-01T00:00:00Z',
  updated_at: '2024-01-01T00:00:00Z',
}

const mockAgent = {
  uuid: 'agent-1',
  name: 'Test Agent',
  status: 'online',
  created_at: '2024-01-01T00:00:00Z',
}

function renderHecate() {
  return render(
    <MemoryRouter>
      <Hecate />
    </MemoryRouter>,
  )
}

describe('Hecate page', () => {
  beforeEach(() => {
    vi.mocked(useHecate).mockReturnValue({
      tunnels: [mockTunnel],
      statuses: [],
      loadingTunnels: false,
      loadingStatus: false,
      error: null,
      tunnelsError: null,
      statusError: null,
      getStatus: vi.fn().mockReturnValue(null),
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
    vi.mocked(useAgentList).mockReturnValue({
      data: [mockAgent],
    } as unknown as ReturnType<typeof useAgentList>)
    vi.mocked(useProvisionAgent).mockReturnValue({
      mutateAsync: vi.fn(),
      isPending: false,
    } as unknown as ReturnType<typeof useProvisionAgent>)
    vi.mocked(useOrthrus).mockReturnValue({
      agents: [mockAgent],
      loading: false,
      error: null,
      provisionAgent: vi.fn(),
      deleteAgent: vi.fn(),
      revokeAgent: vi.fn(),
      getInstallSnippets: vi.fn(),
      isProvisioning: false,
      isDeleting: false,
      isRevoking: false,
      isFetchingSnippets: false,
      provisionResult: null,
    } as unknown as ReturnType<typeof useOrthrus>)
  })

  it('renders the page title', () => {
    renderHecate()
    expect(screen.getByText('Hecate')).toBeInTheDocument()
  })

  it('renders the tunnel table with data', () => {
    renderHecate()
    expect(screen.getByText('Test Tunnel')).toBeInTheDocument()
  })

  it('opens tunnel form when Add Provider button is clicked', () => {
    renderHecate()

    fireEvent.click(screen.getByRole('button', { name: /add provider/i }))

    expect(screen.getByTestId('tunnel-form')).toBeInTheDocument()
  })

  it('renders orthrus agent manager section', () => {
    renderHecate()
    expect(screen.getByTestId('agent-manager')).toBeInTheDocument()
  })

  it('shows loading state when tunnels are loading', () => {
    vi.mocked(useHecate).mockReturnValue({
      ...vi.mocked(useHecate).mock.results[0]?.value,
      loadingTunnels: true,
      tunnels: [],
    } as unknown as ReturnType<typeof useHecate>)

    renderHecate()

    expect(screen.getByText('Hecate')).toBeInTheDocument()
  })

  it('shows error alert when error is returned', () => {
    vi.mocked(useHecate).mockReturnValue({
      tunnels: [],
      statuses: [],
      loadingTunnels: false,
      loadingStatus: false,
      error: 'Load failed',
      tunnelsError: null,
      statusError: null,
      getStatus: vi.fn().mockReturnValue(null),
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

    renderHecate()

    expect(screen.getByText(/load failed/i)).toBeInTheDocument()
  })

  it('shows empty state when no tunnels', () => {
    vi.mocked(useHecate).mockReturnValue({
      tunnels: [],
      statuses: [],
      loadingTunnels: false,
      loadingStatus: false,
      error: null,
      tunnelsError: null,
      statusError: null,
      getStatus: vi.fn().mockReturnValue(null),
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

    renderHecate()

    expect(screen.getByText(/no providers configured/i)).toBeInTheDocument()
  })

  it('renders provider badge for tunnel', () => {
    renderHecate()
    expect(screen.getByText('cloudflare')).toBeInTheDocument()
  })

  it('shows delete confirmation dialog when delete button clicked', async () => {
    renderHecate()

    const deleteBtn = await screen.findByRole('button', { name: /delete provider test tunnel/i })
    fireEvent.click(deleteBtn)

    expect(await screen.findByText(/are you sure/i)).toBeInTheDocument()
  })

  it('calls deleteTunnel on confirm', async () => {
    const mockDelete = vi.fn(() => Promise.resolve())
    vi.mocked(useHecate).mockReturnValue({
      tunnels: [mockTunnel],
      statuses: [],
      loadingTunnels: false,
      loadingStatus: false,
      error: null,
      tunnelsError: null,
      statusError: null,
      getStatus: vi.fn().mockReturnValue(null),
      createTunnel: vi.fn(),
      updateTunnel: vi.fn(),
      deleteTunnel: mockDelete as unknown as ReturnType<typeof useHecate>['deleteTunnel'],
      startTunnel: vi.fn() as unknown as ReturnType<typeof useHecate>['startTunnel'],
      stopTunnel: vi.fn() as unknown as ReturnType<typeof useHecate>['stopTunnel'],
      rotateCredentials: vi.fn() as unknown as ReturnType<typeof useHecate>['rotateCredentials'],
      isCreating: false,
      isUpdating: false,
      isDeleting: false,
      isStarting: false,
      isStopping: false,
      isRotating: false,
    })

    renderHecate()

    const deleteBtn = screen.getByRole('button', { name: /delete provider test tunnel/i })
    fireEvent.click(deleteBtn)

    const confirmBtn = await screen.findByRole('button', { name: /^delete$/i })
    fireEvent.click(confirmBtn)

    await waitFor(() => {
      expect(mockDelete).toHaveBeenCalledWith('tunnel-1')
    })
  })

  it('calls startTunnel when start button clicked', async () => {
    const mockStart = vi.fn(() => Promise.resolve())
    vi.mocked(useHecate).mockReturnValue({
      tunnels: [mockTunnel],
      statuses: [],
      loadingTunnels: false,
      loadingStatus: false,
      error: null,
      tunnelsError: null,
      statusError: null,
      getStatus: vi.fn().mockReturnValue({ state: 'disconnected' }),
      createTunnel: vi.fn(),
      updateTunnel: vi.fn(),
      deleteTunnel: vi.fn(),
      startTunnel: mockStart as unknown as ReturnType<typeof useHecate>['startTunnel'],
      stopTunnel: vi.fn() as unknown as ReturnType<typeof useHecate>['stopTunnel'],
      rotateCredentials: vi.fn() as unknown as ReturnType<typeof useHecate>['rotateCredentials'],
      isCreating: false,
      isUpdating: false,
      isDeleting: false,
      isStarting: false,
      isStopping: false,
      isRotating: false,
    })

    renderHecate()

    const startBtn = screen.getByRole('button', { name: /start test tunnel/i })
    fireEvent.click(startBtn)

    await waitFor(() => {
      expect(mockStart).toHaveBeenCalledWith('tunnel-1')
    })
  })

  it('calls stopTunnel when stop button clicked on connected tunnel', async () => {
    const mockStop = vi.fn(() => Promise.resolve())
    vi.mocked(useHecate).mockReturnValue({
      tunnels: [mockTunnel],
      statuses: [],
      loadingTunnels: false,
      loadingStatus: false,
      error: null,
      tunnelsError: null,
      statusError: null,
      getStatus: vi.fn().mockReturnValue({ state: 'connected' }),
      createTunnel: vi.fn(),
      updateTunnel: vi.fn(),
      deleteTunnel: vi.fn(),
      startTunnel: vi.fn() as unknown as ReturnType<typeof useHecate>['startTunnel'],
      stopTunnel: mockStop as unknown as ReturnType<typeof useHecate>['stopTunnel'],
      rotateCredentials: vi.fn() as unknown as ReturnType<typeof useHecate>['rotateCredentials'],
      isCreating: false,
      isUpdating: false,
      isDeleting: false,
      isStarting: false,
      isStopping: false,
      isRotating: false,
    })

    renderHecate()

    const stopBtn = screen.getByRole('button', { name: /stop test tunnel/i })
    fireEvent.click(stopBtn)

    await waitFor(() => {
      expect(mockStop).toHaveBeenCalledWith('tunnel-1')
    })
  })

  it('shows rotate credentials dialog', async () => {
    renderHecate()

    const rotateBtn = screen.getByRole('button', { name: /rotate credentials test tunnel/i })
    fireEvent.click(rotateBtn)

    expect(await screen.findByRole('textbox', { name: /rotate credentials/i })).toBeInTheDocument()
  })

  it('opens provision agent dialog on button click', async () => {
    renderHecate()

    const provisionBtn = screen.getByRole('button', { name: /provision new agent/i })
    fireEvent.click(provisionBtn)

    expect(await screen.findByRole('textbox', { name: /^name/i })).toBeInTheDocument()
  })

  it('opens log viewer when view logs button clicked', async () => {
    renderHecate()

    const logsBtn = screen.getByRole('button', { name: /view logs test tunnel/i })
    fireEvent.click(logsBtn)

    expect(await screen.findByTestId('log-viewer')).toBeInTheDocument()
  })

  it('opens edit tunnel form when edit button clicked', async () => {
    renderHecate()

    const editBtn = screen.getByRole('button', { name: /^edit test tunnel$/i })
    fireEvent.click(editBtn)

    expect(await screen.findByTestId('tunnel-form')).toBeInTheDocument()
  })

  it('renders status badge when tunnel has status', () => {
    vi.mocked(useHecate).mockReturnValue({
      tunnels: [mockTunnel],
      statuses: [],
      loadingTunnels: false,
      loadingStatus: false,
      error: null,
      tunnelsError: null,
      statusError: null,
      getStatus: vi.fn().mockReturnValue({ state: 'connected' }),
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

    renderHecate()

    expect(screen.getByText('connected')).toBeInTheDocument()
  })
})
