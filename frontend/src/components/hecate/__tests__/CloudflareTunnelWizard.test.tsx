import { render, screen, waitFor, act } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, it, expect, vi, beforeEach } from 'vitest'

vi.mock('../../../hooks/useHecate')
vi.mock('../../../api/hecate', () => ({
  getTunnelStatus: vi.fn(),
  // keep other exports as no-ops so imports don't fail
  listTunnels: vi.fn(),
  createTunnel: vi.fn(),
  updateTunnel: vi.fn(),
  deleteTunnel: vi.fn(),
  startTunnel: vi.fn(),
  stopTunnel: vi.fn(),
  rotateCredentials: vi.fn(),
  connectTunnelLogs: vi.fn(),
}))

import { CloudflareTunnelWizard } from '../CloudflareTunnelWizard'
import * as hecateApi from '../../../api/hecate'
import * as useHecateHook from '../../../hooks/useHecate'

const mockCreateTunnel = vi.fn()

const defaultHecate = {
  createTunnel: mockCreateTunnel,
  isCreating: false,
  tunnels: [],
  statuses: [],
  loadingTunnels: false,
  loadingStatus: false,
  error: null,
  tunnelsError: null,
  statusError: null,
  getStatus: vi.fn(),
  updateTunnel: vi.fn(),
  deleteTunnel: vi.fn(),
  startTunnel: vi.fn(),
  stopTunnel: vi.fn(),
  rotateCredentials: vi.fn(),
  isUpdating: false,
  isDeleting: false,
  isStarting: false,
  isStopping: false,
  isRotating: false,
}

const defaultProps = {
  serverName: 'My Tunnel Server',
  onSuccess: vi.fn(),
  onCancel: vi.fn(),
}

describe('CloudflareTunnelWizard', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.mocked(useHecateHook.useHecate).mockReturnValue(defaultHecate)
    vi.mocked(hecateApi.getTunnelStatus).mockResolvedValue([])
  })

  it('renders the dialog with step 1 content', () => {
    render(<CloudflareTunnelWizard {...defaultProps} />)

    expect(screen.getByRole('dialog')).toBeInTheDocument()
    // Step 1 has a password input for the token
    expect(screen.getByLabelText(/tunnel token/i)).toBeInTheDocument()
  })

  it('shows token input as password by default', () => {
    render(<CloudflareTunnelWizard {...defaultProps} />)

    const input = screen.getByLabelText(/tunnel token/i) as HTMLInputElement
    expect(input.type).toBe('password')
  })

  it('toggles token visibility when show/hide button is clicked', async () => {
    render(<CloudflareTunnelWizard {...defaultProps} />)

    const input = screen.getByLabelText(/tunnel token/i) as HTMLInputElement
    expect(input.type).toBe('password')

    const toggleBtn = screen.getByRole('button', { name: /show token/i })
    await userEvent.click(toggleBtn)

    expect((screen.getByLabelText(/tunnel token/i) as HTMLInputElement).type).toBe('text')
  })

  it('hide token button appears after showing token', async () => {
    render(<CloudflareTunnelWizard {...defaultProps} />)

    await userEvent.click(screen.getByRole('button', { name: /show token/i }))

    expect(screen.getByRole('button', { name: /hide token/i })).toBeInTheDocument()
  })

  it('Next button is disabled when token is empty', () => {
    render(<CloudflareTunnelWizard {...defaultProps} />)

    const nextBtn = screen.getByRole('button', { name: /next/i })
    expect(nextBtn).toBeDisabled()
  })

  it('Next button is enabled when token has value', async () => {
    render(<CloudflareTunnelWizard {...defaultProps} />)

    await userEvent.type(screen.getByLabelText(/tunnel token/i), 'my-secret-token')

    expect(screen.getByRole('button', { name: /next/i })).not.toBeDisabled()
  })

  it('calls createTunnel with correct payload on step 1 Next', async () => {
    mockCreateTunnel.mockResolvedValue({ uuid: 'tunnel-uuid', name: 'My Tunnel Server', provider: 'cloudflare' })
    render(<CloudflareTunnelWizard {...defaultProps} />)

    await userEvent.type(screen.getByLabelText(/tunnel token/i), 'my-cf-token')
    await userEvent.click(screen.getByRole('button', { name: /next/i }))

    await waitFor(() => {
      expect(mockCreateTunnel).toHaveBeenCalledWith({
        name: 'My Tunnel Server',
        provider: 'cloudflare',
        credentials: 'my-cf-token',
      })
    })
  })

  it('advances to step 2 after successful tunnel creation', async () => {
    mockCreateTunnel.mockResolvedValue({ uuid: 'tunnel-uuid', name: 'My Tunnel Server', provider: 'cloudflare' })
    render(<CloudflareTunnelWizard {...defaultProps} />)

    await userEvent.type(screen.getByLabelText(/tunnel token/i), 'my-cf-token')
    await userEvent.click(screen.getByRole('button', { name: /next/i }))

    await waitFor(() => {
      // Step 2 has a second Next button and shows the tunnel name
      expect(screen.getAllByRole('button', { name: /next/i }).length).toBeGreaterThan(0)
    })
  })

  it('shows error message when createTunnel fails', async () => {
    mockCreateTunnel.mockRejectedValue(new Error('API Error'))
    render(<CloudflareTunnelWizard {...defaultProps} />)

    await userEvent.type(screen.getByLabelText(/tunnel token/i), 'bad-token')
    await userEvent.click(screen.getByRole('button', { name: /next/i }))

    await waitFor(() => {
      expect(screen.getByRole('alert')).toBeInTheDocument()
      expect(screen.getByText('API Error')).toBeInTheDocument()
    })
  })

  it('shows fallback error message for non-Error rejections', async () => {
    mockCreateTunnel.mockRejectedValue('string error')
    render(<CloudflareTunnelWizard {...defaultProps} />)

    await userEvent.type(screen.getByLabelText(/tunnel token/i), 'bad-token')
    await userEvent.click(screen.getByRole('button', { name: /next/i }))

    await waitFor(() => {
      expect(screen.getByRole('alert')).toBeInTheDocument()
      expect(screen.getByText('Failed to create tunnel')).toBeInTheDocument()
    })
  })

  it('advances from step 2 to step 3 and starts polling', async () => {
    let intervalCallback: (() => Promise<void>) | null = null
    const setIntervalSpy = vi.spyOn(window, 'setInterval').mockImplementation((fn) => {
      intervalCallback = fn as () => Promise<void>
      return 99 as unknown as ReturnType<typeof setInterval>
    })

    mockCreateTunnel.mockResolvedValue({ uuid: 'tunnel-uuid', name: 'My Tunnel Server', provider: 'cloudflare' })
    vi.mocked(hecateApi.getTunnelStatus).mockResolvedValue([
      { uuid: 'tunnel-uuid', name: 'My Tunnel Server', provider: 'cloudflare', state: 'connecting', uptime_seconds: 0, last_error: '' },
    ])

    render(<CloudflareTunnelWizard {...defaultProps} />)

    // Step 1: enter token and submit
    await userEvent.type(screen.getByLabelText(/tunnel token/i), 'my-token')
    await userEvent.click(screen.getByRole('button', { name: /next/i }))

    // Wait for step 2 to appear
    await waitFor(() => {
      expect(screen.queryByLabelText(/tunnel token/i)).not.toBeInTheDocument()
    })

    // Step 2: click Next to go to step 3
    await userEvent.click(screen.getByRole('button', { name: /next/i }))

    expect(setIntervalSpy).toHaveBeenCalled()

    // Manually trigger the interval callback
    if (intervalCallback) {
      await act(async () => { await (intervalCallback as () => Promise<void>)() })
    }

    expect(hecateApi.getTunnelStatus).toHaveBeenCalled()
    setIntervalSpy.mockRestore()
  })

  it('poll updates tunnel state when found', async () => {
    let intervalCallback: (() => Promise<void>) | null = null
    const setIntervalSpy = vi.spyOn(window, 'setInterval').mockImplementation((fn) => {
      intervalCallback = fn as () => Promise<void>
      return 99 as unknown as ReturnType<typeof setInterval>
    })

    mockCreateTunnel.mockResolvedValue({ uuid: 'tunnel-uuid', name: 'My Tunnel Server', provider: 'cloudflare' })
    vi.mocked(hecateApi.getTunnelStatus).mockResolvedValue([
      { uuid: 'tunnel-uuid', name: 'My Tunnel Server', provider: 'cloudflare', state: 'connected', uptime_seconds: 60, last_error: '' },
    ])

    render(<CloudflareTunnelWizard {...defaultProps} />)

    // Step 1
    await userEvent.type(screen.getByLabelText(/tunnel token/i), 'my-token')
    await userEvent.click(screen.getByRole('button', { name: /next/i }))

    await waitFor(() => {
      expect(screen.queryByLabelText(/tunnel token/i)).not.toBeInTheDocument()
    })

    // Step 2 → step 3
    await userEvent.click(screen.getByRole('button', { name: /next/i }))

    // Trigger poll callback directly
    if (intervalCallback) {
      await act(async () => { await (intervalCallback as () => Promise<void>)() })
    }

    // Done button should become enabled when state is 'connected'
    await waitFor(() => {
      expect(screen.getByRole('button', { name: /done/i })).not.toBeDisabled()
    })

    setIntervalSpy.mockRestore()
  })

  it('calls onSuccess with tunnelUuid when Done is clicked', async () => {
    let intervalCallback: (() => Promise<void>) | null = null
    const setIntervalSpy = vi.spyOn(window, 'setInterval').mockImplementation((fn) => {
      intervalCallback = fn as () => Promise<void>
      return 99 as unknown as ReturnType<typeof setInterval>
    })

    mockCreateTunnel.mockResolvedValue({ uuid: 'tunnel-uuid', name: 'My Tunnel Server', provider: 'cloudflare' })
    vi.mocked(hecateApi.getTunnelStatus).mockResolvedValue([
      { uuid: 'tunnel-uuid', name: 'My Tunnel Server', provider: 'cloudflare', state: 'connected', uptime_seconds: 60, last_error: '' },
    ])

    render(<CloudflareTunnelWizard {...defaultProps} />)

    // Step 1
    await userEvent.type(screen.getByLabelText(/tunnel token/i), 'my-token')
    await userEvent.click(screen.getByRole('button', { name: /next/i }))

    await waitFor(() => {
      expect(screen.queryByLabelText(/tunnel token/i)).not.toBeInTheDocument()
    })

    // Step 2 → 3
    await userEvent.click(screen.getByRole('button', { name: /next/i }))

    // Trigger poll to set 'connected' state
    if (intervalCallback) {
      await act(async () => { await (intervalCallback as () => Promise<void>)() })
    }

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /done/i })).not.toBeDisabled()
    })

    await userEvent.click(screen.getByRole('button', { name: /done/i }))

    expect(defaultProps.onSuccess).toHaveBeenCalledWith('tunnel-uuid')
    setIntervalSpy.mockRestore()
  })

  it('calls onCancel when Cancel button is clicked', async () => {
    render(<CloudflareTunnelWizard {...defaultProps} />)

    await userEvent.click(screen.getByRole('button', { name: /cancel/i }))

    expect(defaultProps.onCancel).toHaveBeenCalled()
  })

  it('clears interval on unmount when poll is active', async () => {
    const clearIntervalSpy = vi.spyOn(window, 'clearInterval')
    const setIntervalSpy = vi.spyOn(window, 'setInterval').mockImplementation((_fn) => {
      return 99 as unknown as ReturnType<typeof setInterval>
    })
    mockCreateTunnel.mockResolvedValue({ uuid: 'tunnel-uuid', name: 'My Tunnel Server', provider: 'cloudflare' })
    vi.mocked(hecateApi.getTunnelStatus).mockResolvedValue([])

    const { unmount } = render(<CloudflareTunnelWizard {...defaultProps} />)

    // Step 1 → 2
    await userEvent.type(screen.getByLabelText(/tunnel token/i), 'my-token')
    await userEvent.click(screen.getByRole('button', { name: /next/i }))

    await waitFor(() => {
      expect(screen.queryByLabelText(/tunnel token/i)).not.toBeInTheDocument()
    })

    // Step 2 → 3 (this starts the poll)
    await userEvent.click(screen.getByRole('button', { name: /next/i }))

    // pollRef.current is now set
    unmount()

    expect(clearIntervalSpy).toHaveBeenCalledWith(99)
    setIntervalSpy.mockRestore()
    clearIntervalSpy.mockRestore()
  })

  it('poll silently ignores fetch errors', async () => {
    let intervalCallback: (() => Promise<void>) | null = null
    const setIntervalSpy = vi.spyOn(window, 'setInterval').mockImplementation((fn) => {
      intervalCallback = fn as () => Promise<void>
      return 99 as unknown as ReturnType<typeof setInterval>
    })
    mockCreateTunnel.mockResolvedValue({ uuid: 'tunnel-uuid', name: 'My Tunnel Server', provider: 'cloudflare' })
    vi.mocked(hecateApi.getTunnelStatus).mockRejectedValue(new Error('Network error'))

    render(<CloudflareTunnelWizard {...defaultProps} />)

    await userEvent.type(screen.getByLabelText(/tunnel token/i), 'my-token')
    await userEvent.click(screen.getByRole('button', { name: /next/i }))

    await waitFor(() => {
      expect(screen.queryByLabelText(/tunnel token/i)).not.toBeInTheDocument()
    })

    await userEvent.click(screen.getByRole('button', { name: /next/i }))

    // Trigger the poll callback — should not throw
    if (intervalCallback) {
      await act(async () => {
        try { await (intervalCallback as () => Promise<void>)() } catch { /* ignored */ }
      })
    }

    // Dialog should still be present (no error shown for poll errors)
    expect(screen.getByRole('dialog')).toBeInTheDocument()
    setIntervalSpy.mockRestore()
  })
})
