import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import Uptime from '../Uptime'
import * as uptimeApi from '../../api/uptime'

vi.mock('react-hot-toast', () => ({ toast: { success: vi.fn(), error: vi.fn(), loading: vi.fn(), dismiss: vi.fn() } }))
vi.mock('../../api/uptime')

const createQueryClient = () => new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } })

const renderWithProviders = (ui: React.ReactNode) => {
  const qc = createQueryClient()
  return render(
    <QueryClientProvider client={qc}>
      {ui}
    </QueryClientProvider>
  )
}

describe('Uptime page', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('renders no monitors message', async () => {
    vi.mocked(uptimeApi.getMonitors).mockResolvedValue([])
    renderWithProviders(<Uptime />)
    expect(await screen.findByText(/No monitors found/i)).toBeTruthy()
  })

  it('calls updateMonitor when toggling monitoring', async () => {
    const monitor = {
      id: 'm1', name: 'Test Monitor', url: 'http://example.com', type: 'http', interval: 60, enabled: true,
      status: 'up', last_check: new Date().toISOString(), latency: 10, max_retries: 3, proxy_host_id: 1,
    }
    vi.mocked(uptimeApi.getMonitors).mockResolvedValue([monitor])
    vi.mocked(uptimeApi.getMonitorHistory).mockResolvedValue([])
    vi.mocked(uptimeApi.updateMonitor).mockResolvedValue({ ...monitor, enabled: false })

    renderWithProviders(<Uptime />)
    await waitFor(() => expect(screen.getByText('Test Monitor')).toBeInTheDocument())
    const card = screen.getByText('Test Monitor').closest('div') as HTMLElement
    const settingsBtn = within(card).getByTitle('Monitor settings')
    await userEvent.click(settingsBtn)
    const toggleBtn = within(card).getByText('Disable Monitoring')
    await userEvent.click(toggleBtn)
    await waitFor(() => expect(uptimeApi.updateMonitor).toHaveBeenCalledWith('m1', { enabled: false }))
  })

  it('shows Never when last_check is missing', async () => {
    const monitor = {
      id: 'm2', name: 'NoLastCheck', url: 'http://example.com', type: 'http', interval: 60, enabled: true,
      status: 'up', last_check: null, latency: 10, max_retries: 3,
    }
    vi.mocked(uptimeApi.getMonitors).mockResolvedValue([monitor])
    vi.mocked(uptimeApi.getMonitorHistory).mockResolvedValue([])

    renderWithProviders(<Uptime />)
    await waitFor(() => expect(screen.getByText('NoLastCheck')).toBeInTheDocument())
    const lastCheck = screen.getByText('Never')
    expect(lastCheck).toBeTruthy()
  })

  it('shows PAUSED state when monitor is disabled', async () => {
    const monitor = {
      id: 'm3', name: 'PausedMonitor', url: 'http://example.com', type: 'http', interval: 60, enabled: false,
      status: 'down', last_check: new Date().toISOString(), latency: 10, max_retries: 3,
    }
    vi.mocked(uptimeApi.getMonitors).mockResolvedValue([monitor])
    vi.mocked(uptimeApi.getMonitorHistory).mockResolvedValue([])

    renderWithProviders(<Uptime />)
    await waitFor(() => expect(screen.getByText('PausedMonitor')).toBeInTheDocument())
    expect(screen.getByText('PAUSED')).toBeTruthy()
  })

  it('renders heartbeat bars from history and displays status in bar titles', async () => {
    const monitor = {
      id: 'm4', name: 'WithHistory', url: 'http://example.com', type: 'http', interval: 60, enabled: true,
      status: 'up', last_check: new Date().toISOString(), latency: 10, max_retries: 3,
    }
    const now = new Date()
    const history = [
      { id: 1, monitor_id: 'm4', status: 'up', latency: 10, message: 'OK', created_at: new Date(now.getTime() - 30000).toISOString() },
      { id: 2, monitor_id: 'm4', status: 'down', latency: 20, message: 'Fail', created_at: new Date(now.getTime() - 20000).toISOString() },
      { id: 3, monitor_id: 'm4', status: 'up', latency: 5, message: 'OK', created_at: new Date(now.getTime() - 10000).toISOString() },
    ]
    vi.mocked(uptimeApi.getMonitors).mockResolvedValue([monitor])
    vi.mocked(uptimeApi.getMonitorHistory).mockResolvedValue(history)

    renderWithProviders(<Uptime />)
    await waitFor(() => expect(screen.getByText('WithHistory')).toBeInTheDocument())

    // Bar titles include 'Status:' and the status should be capitalized
    await waitFor(() => expect(document.querySelectorAll('[title*="Status:"]').length).toBeGreaterThanOrEqual(history.length))
    const barTitles = Array.from(document.querySelectorAll('[title*="Status:"]'))
    expect(barTitles.some(el => (el.getAttribute('title') || '').includes('Status: UP'))).toBeTruthy()
    expect(barTitles.some(el => (el.getAttribute('title') || '').includes('Status: DOWN'))).toBeTruthy()
  })

  it('deletes monitor when delete confirmed and shows toast', async () => {
    const monitor = {
      id: 'm5', name: 'DeleteMe', url: 'http://example.com', type: 'http', interval: 60, enabled: true,
      status: 'up', last_check: new Date().toISOString(), latency: 10, max_retries: 3,
    }
    vi.mocked(uptimeApi.getMonitors).mockResolvedValue([monitor])
    vi.mocked(uptimeApi.getMonitorHistory).mockResolvedValue([])
    vi.mocked(uptimeApi.deleteMonitor).mockResolvedValue(undefined)

    const confirmSpy = vi.spyOn(window, 'confirm').mockImplementation(() => true)
    renderWithProviders(<Uptime />)
    await waitFor(() => expect(screen.getByText('DeleteMe')).toBeInTheDocument())
    const card = screen.getByText('DeleteMe').closest('div') as HTMLElement
    const settingsBtn = within(card).getByTitle('Monitor settings')
    await userEvent.click(settingsBtn)
    const deleteBtn = within(card).getByText('Delete')
    await userEvent.click(deleteBtn)
    await waitFor(() => expect(uptimeApi.deleteMonitor).toHaveBeenCalledWith('m5'))
    confirmSpy.mockRestore()
  })

  it('opens configure modal and saves changes via updateMonitor', async () => {
    const monitor = {
      id: 'm6', name: 'ConfigMe', url: 'http://example.com', type: 'http', interval: 60, enabled: true,
      status: 'up', last_check: new Date().toISOString(), latency: 10, max_retries: 3, proxy_host_id: 1,
    }
    vi.mocked(uptimeApi.getMonitors).mockResolvedValue([monitor])
    vi.mocked(uptimeApi.getMonitorHistory).mockResolvedValue([])
    vi.mocked(uptimeApi.updateMonitor).mockResolvedValue({ ...monitor, max_retries: 6 })

    renderWithProviders(<Uptime />)
    await waitFor(() => expect(screen.getByText('ConfigMe')).toBeInTheDocument())
    const card = screen.getByText('ConfigMe').closest('div') as HTMLElement
    await userEvent.click(within(card).getByTitle('Monitor settings'))
    await userEvent.click(within(card).getByText('Configure'))
    // Modal should open
    await waitFor(() => expect(screen.getByText('Configure Monitor')).toBeInTheDocument())
    const spinbuttons = screen.getAllByRole('spinbutton')
    const maxRetriesInput = spinbuttons.find(el => el.getAttribute('value') === '3') as HTMLInputElement
    await userEvent.clear(maxRetriesInput)
    await userEvent.type(maxRetriesInput, '6')
    await userEvent.click(screen.getByText('Save Changes'))
    await waitFor(() => expect(uptimeApi.updateMonitor).toHaveBeenCalledWith('m6', { max_retries: 6, interval: 60 }))
  })

  it('does not call deleteMonitor when canceling delete', async () => {
    const monitor = {
      id: 'm7', name: 'DoNotDelete', url: 'http://example.com', type: 'http', interval: 60, enabled: true,
      status: 'up', last_check: new Date().toISOString(), latency: 10, max_retries: 3,
    }
    vi.mocked(uptimeApi.getMonitors).mockResolvedValue([monitor])
    vi.mocked(uptimeApi.getMonitorHistory).mockResolvedValue([])
    vi.mocked(uptimeApi.deleteMonitor).mockResolvedValue(undefined)

    const confirmSpy = vi.spyOn(window, 'confirm').mockImplementation(() => false)
    renderWithProviders(<Uptime />)
    await waitFor(() => expect(screen.getByText('DoNotDelete')).toBeInTheDocument())
    const card = screen.getByText('DoNotDelete').closest('div') as HTMLElement
    await userEvent.click(within(card).getByTitle('Monitor settings'))
    await userEvent.click(within(card).getByText('Delete'))
    expect(uptimeApi.deleteMonitor).not.toHaveBeenCalled()
    confirmSpy.mockRestore()
  })

  it('shows toast error when toggle update fails', async () => {
    const monitor = {
      id: 'm8', name: 'ToggleFail', url: 'http://example.com', type: 'http', interval: 60, enabled: true,
      status: 'up', last_check: new Date().toISOString(), latency: 10, max_retries: 3, proxy_host_id: 1,
    }
    vi.mocked(uptimeApi.getMonitors).mockResolvedValue([monitor])
    vi.mocked(uptimeApi.getMonitorHistory).mockResolvedValue([])
    vi.mocked(uptimeApi.updateMonitor).mockRejectedValue(new Error('Update failed'))

    renderWithProviders(<Uptime />)
    await waitFor(() => expect(screen.getByText('ToggleFail')).toBeInTheDocument())
    const card = screen.getByText('ToggleFail').closest('div') as HTMLElement
    await userEvent.click(within(card).getByTitle('Monitor settings'))
    await userEvent.click(within(card).getByText('Disable Monitoring'))
    const toast = (await import('react-hot-toast')).toast
    await waitFor(() => expect(toast.error).toHaveBeenCalled())
  })

  it('separates monitors into Proxy Hosts, Remote Servers and Other sections', async () => {
    const proxyMonitor = { id: 'm9', name: 'ProxyMon', url: 'http://p', type: 'http', interval: 60, enabled: true, status: 'up', last_check: new Date().toISOString(), latency: 1, max_retries: 2, proxy_host_id: 1 }
    const remoteMonitor = { id: 'm10', name: 'RemoteMon', url: 'http://r', type: 'http', interval: 60, enabled: true, status: 'up', last_check: new Date().toISOString(), latency: 2, max_retries: 2, remote_server_id: 2 }
    const otherMonitor = { id: 'm11', name: 'OtherMon', url: 'http://o', type: 'http', interval: 60, enabled: true, status: 'up', last_check: new Date().toISOString(), latency: 3, max_retries: 2 }
    vi.mocked(uptimeApi.getMonitors).mockResolvedValue([proxyMonitor, remoteMonitor, otherMonitor])
    vi.mocked(uptimeApi.getMonitorHistory).mockResolvedValue([])

    renderWithProviders(<Uptime />)
    await waitFor(() => expect(screen.getByText('Proxy Hosts')).toBeInTheDocument())
    expect(screen.getByText('Remote Servers')).toBeInTheDocument()
    expect(screen.getByText('Other Monitors')).toBeInTheDocument()
    expect(screen.getByText('ProxyMon')).toBeInTheDocument()
    expect(screen.getByText('RemoteMon')).toBeInTheDocument()
    expect(screen.getByText('OtherMon')).toBeInTheDocument()
  })
})
