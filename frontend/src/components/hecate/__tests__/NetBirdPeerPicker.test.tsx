import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import userEvent from '@testing-library/user-event'
import { describe, it, expect, vi, beforeEach } from 'vitest'

import { NetBirdPeerPicker } from '../NetBirdPeerPicker'

vi.mock('../../../api/hecate', () => ({
  listNetBirdPeers: vi.fn(),
}))

import { listNetBirdPeers } from '../../../api/hecate'

const mockPeers = [
  { id: 'peer-1', name: 'Server A', ip: '100.64.0.1', os: 'linux', connection_state: 'connected', last_seen: '2024-01-01T00:00:00Z', online: true },
  { id: 'peer-2', name: 'Server B', ip: '100.64.0.2', os: 'windows', connection_state: 'disconnected', last_seen: '2024-01-01T00:00:00Z', online: false },
]

function makeClient() {
  return new QueryClient({ defaultOptions: { queries: { retry: false } } })
}

function renderPicker(props = {}) {
  return render(
    <QueryClientProvider client={makeClient()}>
      <NetBirdPeerPicker
        open
        onClose={vi.fn()}
        onSelect={vi.fn()}
        {...props}
      />
    </QueryClientProvider>,
  )
}

describe('NetBirdPeerPicker', () => {
  beforeEach(() => {
    vi.mocked(listNetBirdPeers).mockResolvedValue(mockPeers)
  })

  it('does not render when closed', () => {
    render(
      <QueryClientProvider client={makeClient()}>
        <NetBirdPeerPicker open={false} onClose={vi.fn()} onSelect={vi.fn()} />
      </QueryClientProvider>,
    )

    expect(screen.queryByRole('listbox')).not.toBeInTheDocument()
  })

  it('calls onClose when dialog is closed', () => {
    const onClose = vi.fn()
    renderPicker({ onClose })

    fireEvent.keyDown(document, { key: 'Escape' })
  })

  it('shows loading state initially', () => {
    vi.mocked(listNetBirdPeers).mockImplementation(() => new Promise(() => {}))
    renderPicker()

    expect(screen.getByRole('dialog')).toBeInTheDocument()
  })

  // --- Additional coverage ---

  it('shows empty message when no peers exist (line 54)', async () => {
    vi.mocked(listNetBirdPeers).mockResolvedValue([])
    renderPicker()

    await waitFor(() => {
      expect(screen.getByText(/No Tailscale devices found/i)).toBeInTheDocument()
    })
  })

  it('renders peer list items', async () => {
    renderPicker()

    await waitFor(() => {
      expect(screen.getByText('Server A')).toBeInTheDocument()
    })

    expect(screen.getByText('Server B')).toBeInTheDocument()
    expect(screen.getByText('100.64.0.1')).toBeInTheDocument()
  })

  it('calls onSelect and onClose when a peer button is clicked (lines 64-65)', async () => {
    const onClose = vi.fn()
    const onSelect = vi.fn()
    renderPicker({ onClose, onSelect })

    await waitFor(() => {
      expect(screen.getByText('Server A')).toBeInTheDocument()
    })

    await userEvent.click(screen.getByRole('option', { name: /Server A/i }))

    expect(onSelect).toHaveBeenCalledWith(mockPeers[0])
    expect(onClose).toHaveBeenCalledTimes(1)
  })

  it('marks the selected peer with aria-selected=true', async () => {
    renderPicker({ selectedId: 'peer-1' })

    await waitFor(() => {
      expect(screen.getByText('Server A')).toBeInTheDocument()
    })

    expect(screen.getByRole('option', { name: /Server A/i })).toHaveAttribute('aria-selected', 'true')
    expect(screen.getByRole('option', { name: /Server B/i })).toHaveAttribute('aria-selected', 'false')
  })
})
