import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router-dom'
import { describe, it, expect, vi, beforeEach } from 'vitest'

import { ConnectionTypeSelector, type ConnectionTypeSelectorProps } from '../ConnectionTypeSelector'

vi.mock('../ProviderDevicePicker', () => ({
  ProviderDevicePicker: () => <div data-testid="provider-device-picker" />,
}))

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
  { uuid: 'agent-1', name: 'My Agent', status: 'online', resolved_address: '10.0.0.1', created_at: '' },
  { uuid: 'agent-2', name: 'No Provider Agent', status: 'online', resolved_address: undefined, created_at: '' },
]

const defaultProps: ConnectionTypeSelectorProps = {
  mode: 'direct',
  onModeChange: vi.fn(),
  selectedTunnelUUID: null,
  selectedAgentUUID: null,
  selectedDeviceId: '',
  onTunnelSelect: vi.fn(),
  onAgentSelect: vi.fn(),
  onDeviceSelect: vi.fn(),
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

  it('renders 3 radios: direct, agent, provider', () => {
    renderSelector()

    expect(screen.getByRole('radio', { name: /direct/i })).toBeInTheDocument()
    expect(screen.getByRole('radio', { name: /agent/i })).toBeInTheDocument()
    expect(screen.getByRole('radio', { name: /provider/i })).toBeInTheDocument()
  })

  it('direct radio is checked when mode is direct', () => {
    renderSelector({ mode: 'direct' })

    expect(screen.getByRole('radio', { name: /direct/i })).toBeChecked()
    expect(screen.getByRole('radio', { name: /agent/i })).not.toBeChecked()
    expect(screen.getByRole('radio', { name: /provider/i })).not.toBeChecked()
  })

  it('agent radio is checked when mode is agent', () => {
    renderSelector({ mode: 'agent' })

    expect(screen.getByRole('radio', { name: /agent/i })).toBeChecked()
  })

  it('provider radio is checked when mode is provider', () => {
    renderSelector({ mode: 'provider' })

    expect(screen.getByRole('radio', { name: /provider/i })).toBeChecked()
  })

  it('shows agent select when mode is agent', () => {
    renderSelector({ mode: 'agent' })

    expect(screen.getByRole('combobox')).toBeInTheDocument()
    expect(document.getElementById('cts-agent')).not.toBeNull()
  })

  it('does not show agent select when mode is direct', () => {
    renderSelector({ mode: 'direct' })

    expect(screen.queryByRole('combobox')).not.toBeInTheDocument()
  })

  it('shows provider device picker when mode is provider', () => {
    renderSelector({ mode: 'provider' })

    expect(screen.getByTestId('provider-device-picker')).toBeInTheDocument()
  })

  it('calls onModeChange with agent when agent radio is selected', async () => {
    const onModeChange = vi.fn()
    renderSelector({ mode: 'direct', onModeChange })

    await userEvent.click(screen.getByRole('radio', { name: /agent/i }))

    expect(onModeChange).toHaveBeenCalledWith('agent')
  })

  it('calls onModeChange with provider when provider radio is selected', async () => {
    const onModeChange = vi.fn()
    renderSelector({ mode: 'direct', onModeChange })

    await userEvent.click(screen.getByRole('radio', { name: /provider/i }))

    expect(onModeChange).toHaveBeenCalledWith('provider')
  })

  it('calls onModeChange with direct when direct radio is selected', async () => {
    const onModeChange = vi.fn()
    renderSelector({ mode: 'agent', onModeChange })

    await userEvent.click(screen.getByRole('radio', { name: /direct/i }))

    expect(onModeChange).toHaveBeenCalledWith('direct')
  })

  it('calls onAgentSelect when an agent option is chosen', async () => {
    const onAgentSelect = vi.fn()
    renderSelector({ mode: 'agent', onAgentSelect })

    await userEvent.selectOptions(screen.getByRole('combobox'), 'agent-1')

    expect(onAgentSelect).toHaveBeenCalledWith('agent-1')
  })

  it('shows no-provider warning when selected agent lacks resolved_address', () => {
    renderSelector({ mode: 'agent', selectedAgentUUID: 'agent-2' })

    expect(screen.getByRole('alert')).toBeInTheDocument()
    expect(screen.getByRole('link')).toBeInTheDocument()
  })

  it('does not show no-provider warning when selected agent has resolved_address', () => {
    renderSelector({ mode: 'agent', selectedAgentUUID: 'agent-1' })

    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  })

  it('agent options include suffix for agents with no provider', () => {
    renderSelector({ mode: 'agent' })

    const select = screen.getByRole('combobox')
    expect(select).toBeInTheDocument()
    const options = Array.from(select.querySelectorAll('option'))
    const noProviderOption = options.find(o => o.value === 'agent-2')
    expect(noProviderOption?.textContent).toMatch(/No provider assigned/)
  })
})

