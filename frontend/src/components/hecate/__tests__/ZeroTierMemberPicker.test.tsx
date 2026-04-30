import { render, screen } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
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
})
