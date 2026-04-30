import { render, screen, fireEvent } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { describe, it, expect, vi, beforeEach } from 'vitest'

import Hecate from '../Hecate'

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
      getStatus: vi.fn().mockReturnValue(undefined),
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
  })
})
