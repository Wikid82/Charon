import { render, screen, fireEvent } from '@testing-library/react'
import { describe, it, expect, vi } from 'vitest'

import { type TailscaleDevice } from '../../../api/hecate'
import { TailscaleDevicePicker } from '../TailscaleDevicePicker'

const mockDevices: TailscaleDevice[] = [
  {
    id: 'dev-1',
    hostname: 'my-laptop',
    addresses: ['100.64.0.1'],
    online: true,
    os: 'linux',
    last_seen: '2024-01-02T00:00:00Z',
  },
  {
    id: 'dev-2',
    hostname: 'my-server',
    addresses: ['100.64.0.2'],
    online: false,
    os: 'linux',
    last_seen: '2024-01-02T00:00:00Z',
  },
]

describe('TailscaleDevicePicker', () => {
  it('renders device list when open', () => {
    render(
      <TailscaleDevicePicker
        devices={mockDevices}
        open
        onClose={vi.fn()}
        onSelect={vi.fn()}
      />
    )

    expect(screen.getByText('my-laptop')).toBeInTheDocument()
    expect(screen.getByText('my-server')).toBeInTheDocument()
  })

  it('shows empty state when no devices', () => {
    render(
      <TailscaleDevicePicker
        devices={[]}
        open
        onClose={vi.fn()}
        onSelect={vi.fn()}
      />
    )

    expect(screen.getByText(/no tailscale devices/i)).toBeInTheDocument()
  })

  it('calls onSelect with correct device when clicked', () => {
    const onSelect = vi.fn()
    render(
      <TailscaleDevicePicker
        devices={mockDevices}
        open
        onClose={vi.fn()}
        onSelect={onSelect}
      />
    )

    fireEvent.click(screen.getByText('my-laptop'))

    expect(onSelect).toHaveBeenCalledWith(
      expect.objectContaining({ id: 'dev-1', hostname: 'my-laptop' })
    )
  })

  it('does not render when closed', () => {
    render(
      <TailscaleDevicePicker
        devices={mockDevices}
        open={false}
        onClose={vi.fn()}
        onSelect={vi.fn()}
      />
    )

    expect(screen.queryByText('my-laptop')).not.toBeInTheDocument()
  })

  it('shows online/offline badge', () => {
    render(
      <TailscaleDevicePicker
        devices={mockDevices}
        open
        onClose={vi.fn()}
        onSelect={vi.fn()}
      />
    )

    expect(screen.getByText(/online/i)).toBeInTheDocument()
    expect(screen.getByText(/offline/i)).toBeInTheDocument()
  })

  it('calls onClose when dialog is dismissed via Escape key', () => {
    const onClose = vi.fn()
    render(
      <TailscaleDevicePicker
        devices={[]}
        open
        onClose={onClose}
        onSelect={vi.fn()}
      />
    )

    fireEvent.keyDown(document.body, { key: 'Escape', code: 'Escape' })

    expect(onClose).toHaveBeenCalled()
  })
})
