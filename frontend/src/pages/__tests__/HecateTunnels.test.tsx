import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { BrowserRouter } from 'react-router-dom'
import { describe, it, expect, vi, beforeEach } from 'vitest'

import HecateTunnels from '../HecateTunnels'

vi.mock('../../hooks/useHecate', () => ({
  useHecate: vi.fn(),
}))

vi.mock('../../components/hecate/HecateTunnelForm', () => ({
  HecateTunnelForm: ({ open, onClose }: { open: boolean; onClose: () => void }) =>
    open ? <div data-testid="tunnel-form"><button onClick={onClose}>Close Form</button></div> : null,
}))

vi.mock('../../components/hecate/TunnelLogViewer', () => ({
  TunnelLogViewer: ({ open, onClose }: { open: boolean; onClose: () => void }) =>
    open ? <div data-testid="log-viewer"><button onClick={onClose}>Close Logs</button></div> : null,
}))

vi.mock('../../components/hecate/TunnelStatusBadge', () => ({
  TunnelStatusBadge: ({ state }: { state: string }) =>
    <span data-testid="status-badge">{state}</span>,
}))

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string, opts?: Record<string, unknown>) => {
      if (opts?.name) return `${key}:${String(opts.name)}`
      return key
    },
  }),
}))

import { useHecate } from '../../hooks/useHecate'

const mockUseHecate = vi.mocked(useHecate)

const baseMockHecate = {
  tunnels: [] as import('../../api/hecate').TunnelConfig[],
  statuses: [] as import('../../api/hecate').TunnelStatus[],
  loadingTunnels: false,
  loadingStatus: false,
  error: null,
  tunnelsError: null,
  statusError: null,
  getStatus: vi.fn().mockReturnValue(null),
  startTunnel: vi.fn(),
  stopTunnel: vi.fn(),
  deleteTunnel: vi.fn(),
  rotateCredentials: vi.fn(),
  isStarting: false,
  isStopping: false,
  isDeleting: false,
  isRotating: false,
  createTunnel: vi.fn(),
  updateTunnel: vi.fn(),
  isCreating: false,
  isUpdating: false,
}

const renderComponent = () => {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={queryClient}>
      <BrowserRouter>
        <HecateTunnels />
      </BrowserRouter>
    </QueryClientProvider>
  )
}

describe('HecateTunnels', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockUseHecate.mockReturnValue(baseMockHecate)
  })

  it('renders empty state when there are no tunnels', async () => {
    renderComponent()
    expect(await screen.findByText('hecate.page.emptyState.title')).toBeInTheDocument()
  })

  it('renders tunnel rows when tunnels exist', async () => {
    mockUseHecate.mockReturnValue({
      ...baseMockHecate,
      tunnels: [
        {
          uuid: 'uuid-1',
          name: 'My Tunnel',
          provider: 'cloudflare' as const,
          is_active: true,
          configuration: '',
          created_at: '2024-01-01T00:00:00Z',
          updated_at: '2024-01-01T00:00:00Z',
        },
      ],
    })
    renderComponent()
    expect(await screen.findByText('My Tunnel')).toBeInTheDocument()
    expect(await screen.findByText('cloudflare')).toBeInTheDocument()
  })

  it('opens tunnel form when Add Tunnel button is clicked', async () => {
    const user = userEvent.setup()
    renderComponent()
    // The header action button is the first one when on empty state
    const addBtns = await screen.findAllByText('hecate.page.addTunnel')
    await user.click(addBtns[0])
    expect(await screen.findByTestId('tunnel-form')).toBeInTheDocument()
  })

  it('shows delete confirmation dialog when delete action is triggered', async () => {
    const user = userEvent.setup()
    mockUseHecate.mockReturnValue({
      ...baseMockHecate,
      tunnels: [
        {
          uuid: 'uuid-1',
          name: 'DeleteMe',
          provider: 'tailscale' as const,
          is_active: true,
          configuration: '',
          created_at: '2024-01-01T00:00:00Z',
          updated_at: '2024-01-01T00:00:00Z',
        },
      ],
    })
    renderComponent()

    const deleteBtn = await screen.findByTitle('hecate.page.deleteTunnel')
    await user.click(deleteBtn)

    await waitFor(() => {
      expect(screen.getByText('hecate.page.confirmDeleteTitle')).toBeInTheDocument()
    })
  })
})
