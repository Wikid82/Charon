import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, it, expect, vi, beforeEach } from 'vitest'

import { ProviderDevicePicker } from '../ProviderDevicePicker'

vi.mock('@tanstack/react-query', () => ({
  useQuery: vi.fn(),
}))

vi.mock('../TailscaleDevicePicker', () => ({
  TailscaleDevicePicker: ({ open, onSelect }: { open: boolean; onSelect: (d: unknown) => void }) =>
    open ? (
      <div data-testid="tailscale-picker">
        <button onClick={() => onSelect({ id: 'ts-1', hostname: 'myhost', addresses: ['100.1.2.3'] })}>
          Pick Tailscale
        </button>
      </div>
    ) : null,
}))

vi.mock('../NetBirdPeerPicker', () => ({
  NetBirdPeerPicker: ({ open, onSelect }: { open: boolean; onSelect: (p: unknown) => void }) =>
    open ? (
      <div data-testid="netbird-picker">
        <button onClick={() => onSelect({ id: 'nb-1', name: 'nbhost', ip: '100.64.0.1' })}>
          Pick NetBird
        </button>
      </div>
    ) : null,
}))

vi.mock('../ZeroTierMemberPicker', () => ({
  ZeroTierMemberPicker: ({ open, onSelect }: { open: boolean; onSelect: (m: unknown, n: unknown) => void }) =>
    open ? (
      <div data-testid="zerotier-picker">
        <button onClick={() => onSelect({ node_id: 'zt-1', name: 'zthost', ip_assignments: ['192.168.0.1'] }, {})}>
          Pick ZeroTier
        </button>
      </div>
    ) : null,
}))

import { useQuery } from '@tanstack/react-query'

const mockTunnels = [
  { uuid: 'cf-1', name: 'CF Tunnel', provider: 'cloudflare' as const, configuration: '', is_active: true, created_at: '', updated_at: '' },
  { uuid: 'ts-1', name: 'TS Tunnel', provider: 'tailscale' as const, configuration: '', is_active: true, created_at: '', updated_at: '' },
  { uuid: 'nb-1', name: 'NB Tunnel', provider: 'netbird' as const, configuration: '', is_active: true, created_at: '', updated_at: '' },
  { uuid: 'zt-1', name: 'ZT Tunnel', provider: 'zerotier' as const, configuration: '', is_active: true, created_at: '', updated_at: '' },
]

const defaultProps = {
  selectedTunnelUUID: null,
  selectedDeviceId: '',
  tunnels: mockTunnels,
  onTunnelSelect: vi.fn(),
  onDeviceSelect: vi.fn(),
}

describe('ProviderDevicePicker', () => {
  beforeEach(() => {
    vi.mocked(useQuery).mockReturnValue({
      data: [],
      isLoading: false,
    } as unknown as ReturnType<typeof useQuery>)
  })

  it('renders tunnel select dropdown', () => {
    render(<ProviderDevicePicker {...defaultProps} />)

    const select = screen.getByRole('combobox')
    expect(select).toBeInTheDocument()
    expect(screen.getByText('CF Tunnel')).toBeInTheDocument()
    expect(screen.getByText('TS Tunnel')).toBeInTheDocument()
  })

  it('shows cloudflare hostname input when CF tunnel is selected', async () => {
    render(<ProviderDevicePicker {...defaultProps} selectedTunnelUUID="cf-1" />)

    expect(screen.getByRole('textbox')).toBeInTheDocument()
    expect(document.getElementById('cf-hostname')).not.toBeNull()
  })

  it('calls onDeviceSelect with hostname for cloudflare tunnel', async () => {
    const onDeviceSelect = vi.fn()
    render(<ProviderDevicePicker {...defaultProps} selectedTunnelUUID="cf-1" onDeviceSelect={onDeviceSelect} />)

    const input = screen.getByRole('textbox')
    await userEvent.type(input, 'myapp.example.com')

    expect(onDeviceSelect).toHaveBeenLastCalledWith('', 'myapp.example.com')
  })

  it('calls onTunnelSelect when tunnel changes', async () => {
    const onTunnelSelect = vi.fn()
    render(<ProviderDevicePicker {...defaultProps} onTunnelSelect={onTunnelSelect} />)

    await userEvent.selectOptions(screen.getByRole('combobox'), 'cf-1')

    expect(onTunnelSelect).toHaveBeenCalledWith('cf-1', 'cloudflare')
  })

  it('shows device button when tailscale tunnel is selected', () => {
    render(<ProviderDevicePicker {...defaultProps} selectedTunnelUUID="ts-1" />)

    expect(screen.getByRole('button')).toBeInTheDocument()
  })

  it('calls onDeviceSelect when tailscale device is picked', async () => {
    const onDeviceSelect = vi.fn()
    render(<ProviderDevicePicker {...defaultProps} selectedTunnelUUID="ts-1" onDeviceSelect={onDeviceSelect} />)

    await userEvent.click(screen.getByRole('button'))
    expect(screen.getByTestId('tailscale-picker')).toBeInTheDocument()
    await userEvent.click(screen.getByText('Pick Tailscale'))

    expect(onDeviceSelect).toHaveBeenCalledWith('ts-1', '100.1.2.3')
  })

  it('calls onDeviceSelect when netbird peer is picked', async () => {
    const onDeviceSelect = vi.fn()
    render(<ProviderDevicePicker {...defaultProps} selectedTunnelUUID="nb-1" onDeviceSelect={onDeviceSelect} />)

    await userEvent.click(screen.getByRole('button'))
    expect(screen.getByTestId('netbird-picker')).toBeInTheDocument()
    await userEvent.click(screen.getByText('Pick NetBird'))

    expect(onDeviceSelect).toHaveBeenCalledWith('nb-1', '100.64.0.1')
  })

  it('calls onDeviceSelect when zerotier member is picked', async () => {
    const onDeviceSelect = vi.fn()
    render(<ProviderDevicePicker {...defaultProps} selectedTunnelUUID="zt-1" onDeviceSelect={onDeviceSelect} />)

    await userEvent.click(screen.getByRole('button'))
    expect(screen.getByTestId('zerotier-picker')).toBeInTheDocument()
    await userEvent.click(screen.getByText('Pick ZeroTier'))

    expect(onDeviceSelect).toHaveBeenCalledWith('zt-1', '192.168.0.1')
  })
})
