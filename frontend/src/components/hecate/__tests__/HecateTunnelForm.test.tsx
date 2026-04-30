import { render, screen, fireEvent } from '@testing-library/react'
import { describe, it, expect, vi, beforeEach } from 'vitest'

import { HecateTunnelForm } from '../HecateTunnelForm'

vi.mock('../../../hooks/useHecate', () => ({ useHecate: vi.fn() }))

import { useHecate } from '../../../hooks/useHecate'

const mockCreate = vi.fn()
const mockUpdate = vi.fn()

const mockTunnel = {
  uuid: 'tunnel-1',
  name: 'Existing Tunnel',
  provider: 'cloudflare' as const,
  configuration: '{}' as string,
  is_active: true,
  created_at: '2024-01-01T00:00:00Z',
  updated_at: '2024-01-01T00:00:00Z',
}

describe('HecateTunnelForm', () => {
  beforeEach(() => {
    vi.mocked(useHecate).mockReturnValue({
      createTunnel: mockCreate,
      updateTunnel: mockUpdate,
      isCreating: false,
      isUpdating: false,
    } as unknown as ReturnType<typeof useHecate>)
  })

  it('renders create mode title when no tunnel prop', () => {
    render(<HecateTunnelForm open onClose={vi.fn()} />)

    expect(screen.getByText('Add Provider')).toBeInTheDocument()
  })

  it('renders edit mode title when tunnel prop provided', () => {
    render(<HecateTunnelForm tunnel={mockTunnel} open onClose={vi.fn()} />)

    expect(screen.getByText('Edit Provider')).toBeInTheDocument()
  })

  it('provider select is disabled in edit mode', () => {
    render(<HecateTunnelForm tunnel={mockTunnel} open onClose={vi.fn()} />)

    const providerSelect = screen.getByRole('combobox')
    expect(providerSelect).toBeDisabled()
  })

  it('provider select is enabled in create mode', () => {
    render(<HecateTunnelForm open onClose={vi.fn()} />)

    const providerSelect = screen.getByRole('combobox')
    expect(providerSelect).not.toBeDisabled()
  })

  it('shows Cloudflare credential fields when cloudflare is selected', () => {
    render(<HecateTunnelForm open onClose={vi.fn()} />)

    const [apiTokenInput] = screen.getAllByLabelText(/api token/i)
    expect(apiTokenInput).toBeInTheDocument()
  })

  it('calls onClose when Cancel is clicked', () => {
    const onClose = vi.fn()
    render(<HecateTunnelForm open onClose={onClose} />)

    fireEvent.click(screen.getByRole('button', { name: /cancel/i }))

    expect(onClose).toHaveBeenCalled()
  })
})
