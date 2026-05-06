import { render, screen, waitFor } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import userEvent from '@testing-library/user-event'
import { describe, it, expect, vi, beforeEach } from 'vitest'

import { ZeroTierMemberPicker } from '../ZeroTierMemberPicker'

vi.mock('../../../api/hecate', () => ({
  listZeroTierNetworks: vi.fn(),
  listZeroTierMembers: vi.fn(),
}))

import { listZeroTierNetworks, listZeroTierMembers } from '../../../api/hecate'

const mockNetworks = [
  { id: 'net-1', name: 'Home Network', description: '', private: true, total_member_count: 3 },
  { id: 'net-2', name: 'Work Network', description: '', private: true, total_member_count: 1 },
]

const mockMembers = [
  { node_id: 'member-1', name: 'Desktop', description: '', ip_assignments: ['192.168.1.10'], authorized: true, online: true },
  { node_id: 'member-2', name: 'Laptop', description: '', ip_assignments: ['192.168.1.11'], authorized: false, online: false },
]

function makeClient() {
  return new QueryClient({ defaultOptions: { queries: { retry: false } } })
}

function renderPicker(props = {}) {
  return render(
    <QueryClientProvider client={makeClient()}>
      <ZeroTierMemberPicker
        open
        onClose={vi.fn()}
        onSelect={vi.fn()}
        {...props}
      />
    </QueryClientProvider>,
  )
}

describe('ZeroTierMemberPicker', () => {
  beforeEach(() => {
    vi.mocked(listZeroTierNetworks).mockResolvedValue(mockNetworks)
    vi.mocked(listZeroTierMembers).mockResolvedValue(mockMembers)
  })

  it('does not render when closed', () => {
    render(
      <QueryClientProvider client={makeClient()}>
        <ZeroTierMemberPicker open={false} onClose={vi.fn()} onSelect={vi.fn()} />
      </QueryClientProvider>,
    )

    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  })

  it('renders dialog when open', () => {
    renderPicker()
    expect(screen.getByRole('dialog')).toBeInTheDocument()
  })

  it('shows loading state while networks are fetching', () => {
    vi.mocked(listZeroTierNetworks).mockImplementation(() => new Promise(() => {}))
    renderPicker()
    expect(screen.getByRole('dialog')).toBeInTheDocument()
  })

  // --- Additional coverage ---

  it('shows empty message when no networks returned (lines 54-55)', async () => {
    vi.mocked(listZeroTierNetworks).mockResolvedValue([])
    renderPicker()

    await waitFor(() => {
      expect(screen.getByText(/No Tailscale devices found/i)).toBeInTheDocument()
    })
  })

  it('renders network list when networks are available (line 48)', async () => {
    renderPicker()

    await waitFor(() => {
      expect(screen.getByText('Home Network')).toBeInTheDocument()
    })

    expect(screen.getByText('Work Network')).toBeInTheDocument()
  })

  it('clicking a network shows the member list (line 97 / 103)', async () => {
    renderPicker()

    await waitFor(() => {
      expect(screen.getByText('Home Network')).toBeInTheDocument()
    })

    await userEvent.click(screen.getByRole('option', { name: /Home Network/i }))

    await waitFor(() => {
      expect(screen.getByText('Desktop')).toBeInTheDocument()
    })

    expect(screen.getByText('Laptop')).toBeInTheDocument()
  })

  it('shows empty message when network has no members (line 97)', async () => {
    vi.mocked(listZeroTierMembers).mockResolvedValue([])
    renderPicker()

    await waitFor(() => {
      expect(screen.getByText('Home Network')).toBeInTheDocument()
    })

    await userEvent.click(screen.getByRole('option', { name: /Home Network/i }))

    await waitFor(() => {
      expect(screen.getAllByText(/No Tailscale devices found/i).length).toBeGreaterThan(0)
    })
  })

  it('back button returns to network list (line 79)', async () => {
    renderPicker()

    await waitFor(() => {
      expect(screen.getByText('Home Network')).toBeInTheDocument()
    })

    await userEvent.click(screen.getByRole('option', { name: /Home Network/i }))

    await waitFor(() => {
      expect(screen.getByText('Desktop')).toBeInTheDocument()
    })

    await userEvent.click(screen.getByRole('button', { name: /back/i }))

    await waitFor(() => {
      expect(screen.getByText('Home Network')).toBeInTheDocument()
    })
  })

  it('calls onSelect and handleClose when member is clicked (lines 61, 124, 134-135)', async () => {
    const onClose = vi.fn()
    const onSelect = vi.fn()
    renderPicker({ onClose, onSelect })

    await waitFor(() => {
      expect(screen.getByText('Home Network')).toBeInTheDocument()
    })

    await userEvent.click(screen.getByRole('option', { name: /Home Network/i }))

    await waitFor(() => {
      expect(screen.getByText('Desktop')).toBeInTheDocument()
    })

    await userEvent.click(screen.getByRole('option', { name: /Desktop/i }))

    expect(onSelect).toHaveBeenCalledWith(mockMembers[0], mockNetworks[0])
    expect(onClose).toHaveBeenCalledTimes(1)
  })

  it('shows isLoading when selectedNetwork is set and members are loading (line 48)', async () => {
    vi.mocked(listZeroTierMembers).mockImplementation(() => new Promise(() => {}))
    renderPicker()

    await waitFor(() => {
      expect(screen.getByText('Home Network')).toBeInTheDocument()
    })

    await userEvent.click(screen.getByRole('option', { name: /Home Network/i }))

    await waitFor(() => {
      expect(screen.getByText(/Loading\.\.\.$/)).toBeInTheDocument()
    })
  })
})
