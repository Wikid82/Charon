import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { BrowserRouter } from 'react-router-dom'
import { describe, it, expect, vi, beforeEach } from 'vitest'

import HecateProviders from '../HecateProviders'

vi.mock('../../hooks/useHecate', () => ({
  useHecate: vi.fn(),
}))

vi.mock('../../components/hecate/HecateTunnelForm', () => ({
  HecateTunnelForm: ({
    open,
    onClose,
    initialProvider,
  }: {
    open: boolean
    onClose: () => void
    initialProvider?: string
  }) =>
    open ? (
      <div data-testid="tunnel-form" data-provider={initialProvider}>
        <button onClick={onClose}>Close Form</button>
      </div>
    ) : null,
}))

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string, opts?: Record<string, unknown>) => {
      if (opts?.count !== undefined) return `${String(opts.count)} tunnels`
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
  getStatus: vi.fn(),
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
        <HecateProviders />
      </BrowserRouter>
    </QueryClientProvider>
  )
}

describe('HecateProviders', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockUseHecate.mockReturnValue(baseMockHecate)
  })

  it('renders all 4 provider cards', async () => {
    renderComponent()
    expect(await screen.findByText('Cloudflare')).toBeInTheDocument()
    expect(await screen.findByText('Tailscale')).toBeInTheDocument()
    expect(await screen.findByText('NetBird')).toBeInTheDocument()
    expect(await screen.findByText('ZeroTier')).toBeInTheDocument()
  })

  it('displays tunnel counts for each provider', async () => {
    mockUseHecate.mockReturnValue({
      ...baseMockHecate,
      tunnels: [
        { uuid: 'u1', name: 'CF1', provider: 'cloudflare' as const, is_active: true, configuration: '', created_at: '', updated_at: '' },
        { uuid: 'u2', name: 'CF2', provider: 'cloudflare' as const, is_active: true, configuration: '', created_at: '', updated_at: '' },
        { uuid: 'u3', name: 'TS1', provider: 'tailscale' as const, is_active: true, configuration: '', created_at: '', updated_at: '' },
      ],
    })
    renderComponent()
    const counts = await screen.findAllByText('2 tunnels')
    expect(counts.length).toBeGreaterThan(0)
  })

  it('opens the tunnel form with the correct provider when New Tunnel is clicked', async () => {
    const user = userEvent.setup()
    renderComponent()

    const netbirdBtn = await screen.findByRole('button', { name: /new netbird tunnel/i })
    await user.click(netbirdBtn)

    await waitFor(() => {
      const form = screen.getByTestId('tunnel-form')
      expect(form).toBeInTheDocument()
      expect(form).toHaveAttribute('data-provider', 'netbird')
    })
  })

  it('opens the tunnel form for Cloudflare when Cloudflare New Tunnel is clicked', async () => {
    const user = userEvent.setup()
    renderComponent()

    const cloudflareBtn = await screen.findByRole('button', { name: /new cloudflare tunnel/i })
    await user.click(cloudflareBtn)

    await waitFor(() => {
      const form = screen.getByTestId('tunnel-form')
      expect(form).toHaveAttribute('data-provider', 'cloudflare')
    })
  })
})
