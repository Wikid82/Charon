import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router-dom'
import { describe, it, expect, vi, beforeEach } from 'vitest'

import { ConnectionTypeSelector, type ConnectionTypeSelectorProps } from '../ConnectionTypeSelector'

vi.mock('../../../hooks/useHecate', () => ({
  useHecate: vi.fn(),
}))

vi.mock('../../../hooks/useOrthrus', () => ({
  useAgentList: vi.fn(),
}))

import { useHecate } from '../../../hooks/useHecate'
import { useAgentList } from '../../../hooks/useOrthrus'

const mockTunnels = [
  { uuid: 'tunnel-1', name: 'My Tunnel', provider: 'cloudflare', is_active: true, created_at: '', updated_at: '' },
]
const mockAgents = [
  { uuid: 'agent-1', name: 'My Agent', status: 'online', created_at: '' },
]

const defaultProps: ConnectionTypeSelectorProps = {
  mode: 'direct',
  onModeChange: vi.fn(),
  selectedTunnelUUID: null,
  selectedAgentUUID: null,
  onTunnelSelect: vi.fn(),
  onAgentSelect: vi.fn(),
}

function renderSelector(props: Partial<ConnectionTypeSelectorProps> = {}) {
  return render(
    <MemoryRouter>
      <ConnectionTypeSelector {...defaultProps} {...props} />
    </MemoryRouter>,
  )
}

describe('ConnectionTypeSelector', () => {
  beforeEach(() => {
    vi.mocked(useHecate).mockReturnValue({
      tunnels: mockTunnels,
    } as unknown as ReturnType<typeof useHecate>)
    vi.mocked(useAgentList).mockReturnValue({
      data: mockAgents,
    } as unknown as ReturnType<typeof useAgentList>)
  })

  it('renders direct and agent radio buttons', () => {
    renderSelector()

    expect(screen.getByRole('radio', { name: /direct/i })).toBeInTheDocument()
    expect(screen.getByRole('radio', { name: /agent/i })).toBeInTheDocument()
  })

  it('direct radio is checked when mode is direct', () => {
    renderSelector({ mode: 'direct' })

    expect(screen.getByRole('radio', { name: /direct/i })).toBeChecked()
    expect(screen.getByRole('radio', { name: /agent/i })).not.toBeChecked()
  })

  it('agent radio is checked when mode is agent', () => {
    renderSelector({ mode: 'agent' })

    expect(screen.getByRole('radio', { name: /agent/i })).toBeChecked()
  })

  it('shows provider select when mode is agent', () => {
    renderSelector({ mode: 'agent' })

    expect(screen.getByRole('combobox')).toBeInTheDocument()
  })

  it('does not show provider select when mode is direct', () => {
    renderSelector({ mode: 'direct' })

    expect(screen.queryByRole('combobox')).not.toBeInTheDocument()
  })

  it('calls onModeChange with agent when agent radio is selected', async () => {
    const onModeChange = vi.fn()
    renderSelector({ mode: 'direct', onModeChange })

    await userEvent.click(screen.getByRole('radio', { name: /agent/i }))

    expect(onModeChange).toHaveBeenCalledWith('agent')
  })

  it('calls onModeChange with direct when direct radio is selected', async () => {
    const onModeChange = vi.fn()
    renderSelector({ mode: 'agent', onModeChange })

    await userEvent.click(screen.getByRole('radio', { name: /direct/i }))

    expect(onModeChange).toHaveBeenCalledWith('direct')
  })

  it('renders tunnel and agent optgroups in provider select', () => {
    renderSelector({ mode: 'agent' })

    expect(screen.getByText('My Tunnel')).toBeInTheDocument()
    expect(screen.getByText('My Agent')).toBeInTheDocument()
    const optgroup = document.querySelector('optgroup[label="Cloudflare"]')
    expect(optgroup).not.toBeNull()
  })

  it('shows no-providers hint with Hecate link when there are no tunnels or agents', () => {
    vi.mocked(useHecate).mockReturnValue({ tunnels: [] } as unknown as ReturnType<typeof useHecate>)
    vi.mocked(useAgentList).mockReturnValue({ data: [] } as unknown as ReturnType<typeof useAgentList>)

    renderSelector({ mode: 'agent' })

    expect(screen.getByRole('link', { name: /hecate/i })).toBeInTheDocument()
  })

  it('calls onTunnelSelect when a tunnel option is chosen', async () => {
    const onTunnelSelect = vi.fn()
    renderSelector({ mode: 'agent', onTunnelSelect })

    await userEvent.selectOptions(screen.getByRole('combobox'), 'tunnel-1')

    expect(onTunnelSelect).toHaveBeenCalledWith('tunnel-1', 'cloudflare')
  })

  it('calls onAgentSelect when an orthrus agent option is chosen', async () => {
    const onAgentSelect = vi.fn()
    renderSelector({ mode: 'agent', onAgentSelect })

    await userEvent.selectOptions(screen.getByRole('combobox'), `orthrus:agent-1`)

    expect(onAgentSelect).toHaveBeenCalledWith('agent-1')
  })
})

