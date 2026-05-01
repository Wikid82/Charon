import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { describe, it, expect, vi, beforeEach } from 'vitest'

import { useHecate } from '../../../hooks/useHecate'
import { HecateTunnelForm } from '../HecateTunnelForm'

vi.mock('../../../hooks/useHecate', () => ({ useHecate: vi.fn() }))

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

  it('shows tailscale credential fields when tailscale provider selected', () => {
    render(<HecateTunnelForm open onClose={vi.fn()} />)

    const select = screen.getByRole('combobox')
    fireEvent.change(select, { target: { value: 'tailscale' } })

    expect(screen.getAllByLabelText(/api key/i)[0]).toBeInTheDocument()
    expect(screen.getAllByLabelText(/tailnet/i)[0]).toBeInTheDocument()
  })

  it('shows netbird credential fields when netbird provider selected', () => {
    render(<HecateTunnelForm open onClose={vi.fn()} />)

    const select = screen.getByRole('combobox')
    fireEvent.change(select, { target: { value: 'netbird' } })

    expect(screen.getAllByLabelText(/access token/i)[0]).toBeInTheDocument()
  })

  it('shows zerotier credential fields when zerotier provider selected', () => {
    render(<HecateTunnelForm open onClose={vi.fn()} />)

    const select = screen.getByRole('combobox')
    fireEvent.change(select, { target: { value: 'zerotier' } })

    expect(screen.getAllByLabelText(/api token/i)[0]).toBeInTheDocument()
  })

  it('calls createTunnel on submit with valid data', async () => {
    render(<HecateTunnelForm open onClose={vi.fn()} />)

    fireEvent.change(screen.getByLabelText(/^name/i), { target: { value: 'My Tunnel' } })
    fireEvent.change(screen.getByLabelText(/^api token/i), { target: { value: 'token-value' } })
    fireEvent.change(screen.getByLabelText(/^account id/i), { target: { value: 'account-id' } })

    fireEvent.click(screen.getByRole('button', { name: /^create$/i }))

    await waitFor(() => {
      expect(mockCreate).toHaveBeenCalled()
    })
  })

  it('shows error message when createTunnel throws', async () => {
    mockCreate.mockRejectedValueOnce(new Error('API error'))

    render(<HecateTunnelForm open onClose={vi.fn()} />)

    fireEvent.change(screen.getByLabelText(/^name/i), { target: { value: 'Fail Tunnel' } })
    fireEvent.change(screen.getByLabelText(/^api token/i), { target: { value: 'x' } })
    fireEvent.change(screen.getByLabelText(/^account id/i), { target: { value: 'x' } })

    fireEvent.click(screen.getByRole('button', { name: /^create$/i }))

    await waitFor(() => {
      expect(screen.getByRole('alert')).toBeInTheDocument()
    })
  })

  it('toggles field visibility on eye button click', () => {
    render(<HecateTunnelForm open onClose={vi.fn()} />)

    const eyeButton = screen.getAllByRole('button').find(b =>
      b.getAttribute('aria-label')?.toLowerCase().includes('show')
    )
    expect(eyeButton).toBeTruthy()
    fireEvent.click(eyeButton!)

    const hideButton = screen.getAllByRole('button').find(b =>
      b.getAttribute('aria-label')?.toLowerCase().includes('hide')
    )
    expect(hideButton).toBeTruthy()
  })

  it('calls updateTunnel on submit in edit mode', async () => {
    render(<HecateTunnelForm tunnel={mockTunnel} open onClose={vi.fn()} />)

    fireEvent.click(screen.getByRole('button', { name: /^update$/i }))

    await waitFor(() => {
      expect(mockUpdate).toHaveBeenCalled()
    })
  })

  it('shows edit hint text in edit mode', () => {
    render(<HecateTunnelForm tunnel={mockTunnel} open onClose={vi.fn()} />)

    expect(screen.getByText(/leave any field blank/i)).toBeInTheDocument()
  })

  it('renders active checkbox', () => {
    render(<HecateTunnelForm open onClose={vi.fn()} />)

    expect(screen.getByRole('checkbox')).toBeInTheDocument()
    expect(screen.getByRole('checkbox')).toBeChecked()
  })

  it('toggles active state when checkbox is clicked', () => {
    render(<HecateTunnelForm open onClose={vi.fn()} />)

    const checkbox = screen.getByRole('checkbox')
    fireEvent.click(checkbox)

    expect(checkbox).not.toBeChecked()
  })

  it('updates cloudflare tunnel token field when typed', () => {
    render(<HecateTunnelForm open onClose={vi.fn()} />)

    const tunnelTokenInput = screen.getByLabelText(/^tunnel token$/i)
    fireEvent.change(tunnelTokenInput, { target: { value: 'my-tunnel-token' } })

    expect(tunnelTokenInput).toHaveValue('my-tunnel-token')
  })

  it('updates tailscale credential fields when typed', () => {
    render(<HecateTunnelForm open onClose={vi.fn()} />)

    fireEvent.change(screen.getByRole('combobox'), { target: { value: 'tailscale' } })

    const [apiKeyInput] = screen.getAllByLabelText(/api key/i)
    fireEvent.change(apiKeyInput, { target: { value: 'ts-api-key' } })

    const [tailnetInput] = screen.getAllByLabelText(/tailnet/i)
    fireEvent.change(tailnetInput, { target: { value: 'my-tailnet.ts.net' } })

    expect(apiKeyInput).toHaveValue('ts-api-key')
  })

  it('updates netbird credential fields when typed', () => {
    render(<HecateTunnelForm open onClose={vi.fn()} />)

    fireEvent.change(screen.getByRole('combobox'), { target: { value: 'netbird' } })

    const [accessTokenInput] = screen.getAllByLabelText(/access token/i)
    fireEvent.change(accessTokenInput, { target: { value: 'nb-access-token' } })

    const managementUrlInput = screen.getByLabelText(/^management url/i)
    fireEvent.change(managementUrlInput, { target: { value: 'https://netbird.example.com' } })

    expect(accessTokenInput).toHaveValue('nb-access-token')
  })

  it('updates zerotier credential fields when typed', () => {
    render(<HecateTunnelForm open onClose={vi.fn()} />)

    fireEvent.change(screen.getByRole('combobox'), { target: { value: 'zerotier' } })

    const apiTokenInput = screen.getByLabelText(/^api token/i)
    fireEvent.change(apiTokenInput, { target: { value: 'zt-api-token' } })

    const controllerUrlInput = screen.getByLabelText(/^controller url/i)
    fireEvent.change(controllerUrlInput, { target: { value: 'https://zt.example.com' } })

    expect(apiTokenInput).toHaveValue('zt-api-token')
  })

  it('calls onClose when dialog is dismissed via Escape key', () => {
    const onClose = vi.fn()
    render(<HecateTunnelForm open onClose={onClose} />)

    fireEvent.keyDown(document.body, { key: 'Escape', code: 'Escape' })

    expect(onClose).toHaveBeenCalled()
  })
})
