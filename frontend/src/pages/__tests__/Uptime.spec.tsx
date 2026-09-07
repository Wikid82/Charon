import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, it, expect, vi, beforeEach } from 'vitest'

import * as uptimeApi from '../../api/uptime'
import Uptime from '../Uptime'

import type { MonitorSummary } from '../../api/uptime'

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

const mkSummary = (partial: Partial<MonitorSummary> & Pick<MonitorSummary, 'id' | 'name'>): MonitorSummary => ({
  type: 'http',
  url: 'http://example.com',
  interval: 60,
  enabled: true,
  status: 'up',
  latency: 10,
  last_check: new Date().toISOString(),
  proxy_host_id: null,
  remote_server_id: null,
  uptime_24h: null,
  recent_beats: [],
  max_retries: 3,
  ...partial,
})

describe('Uptime page', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('renders no monitors message', async () => {
    vi.mocked(uptimeApi.getMonitorsSummary).mockResolvedValue([])
    renderWithProviders(<Uptime />)
    expect(await screen.findByText(/No monitors found/i)).toBeTruthy()
  })

  it('calls updateMonitor when toggling monitoring', async () => {
    const monitor = mkSummary({ id: 'm1', name: 'Test Monitor', proxy_host_id: 1 })
    vi.mocked(uptimeApi.getMonitorsSummary).mockResolvedValue([monitor])
    vi.mocked(uptimeApi.updateMonitor).mockResolvedValue({
      id: 'm1', name: 'Test Monitor', type: 'http', url: 'http://example.com', interval: 60, enabled: false, status: 'up', latency: 10, max_retries: 3,
    })

    renderWithProviders(<Uptime />)
    expect(await screen.findByText('Test Monitor')).toBeInTheDocument()
    const card = screen.getByText('Test Monitor').closest('div') as HTMLElement
    const settingsBtn = within(card).getByTitle('Monitor settings')
    await userEvent.click(settingsBtn)
    const toggleBtn = within(card).getByText('Pause')
    await userEvent.click(toggleBtn)
    await waitFor(() => expect(uptimeApi.updateMonitor).toHaveBeenCalledWith('m1', { enabled: false }))
  })

  it('shows Never when last_check is missing', async () => {
    vi.mocked(uptimeApi.getMonitorsSummary).mockResolvedValue([mkSummary({ id: 'm2', name: 'NoLastCheck', last_check: null })])

    renderWithProviders(<Uptime />)
    expect(await screen.findByText('NoLastCheck')).toBeInTheDocument()
    expect(screen.getByText('Never')).toBeTruthy()
  })

  it('shows PAUSED state when monitor is disabled', async () => {
    vi.mocked(uptimeApi.getMonitorsSummary).mockResolvedValue([mkSummary({ id: 'm3', name: 'PausedMonitor', enabled: false, status: 'down' })])

    renderWithProviders(<Uptime />)
    expect(await screen.findByText('PausedMonitor')).toBeInTheDocument()
    expect(screen.getByText('PAUSED')).toBeTruthy()
  })

  it('renders heartbeat bars from recent_beats and displays status in bar titles', async () => {
    const now = Date.now()
    const monitor = mkSummary({
      id: 'm4',
      name: 'WithHistory',
      recent_beats: [
        { status: 'up', latency: 10, created_at: new Date(now - 30000).toISOString() },
        { status: 'down', latency: 20, created_at: new Date(now - 20000).toISOString() },
        { status: 'up', latency: 5, created_at: new Date(now - 10000).toISOString() },
      ],
    })
    vi.mocked(uptimeApi.getMonitorsSummary).mockResolvedValue([monitor])

    renderWithProviders(<Uptime />)
    expect(await screen.findByText('WithHistory')).toBeInTheDocument()

    await waitFor(() => expect(document.querySelectorAll('[title*="Status:"]').length).toBeGreaterThanOrEqual(3))
    const barTitles = Array.from(document.querySelectorAll('[title*="Status:"]'))
    expect(barTitles.some(el => (el.getAttribute('title') || '').includes('Status: UP'))).toBeTruthy()
    expect(barTitles.some(el => (el.getAttribute('title') || '').includes('Status: DOWN'))).toBeTruthy()
  })

  it('does not issue a per-monitor history request when rendering cards', async () => {
    vi.mocked(uptimeApi.getMonitorsSummary).mockResolvedValue([mkSummary({ id: 'm4b', name: 'NoHistoryFetch' })])

    renderWithProviders(<Uptime />)
    expect(await screen.findByText('NoHistoryFetch')).toBeInTheDocument()
    expect(uptimeApi.getMonitorHistory).not.toHaveBeenCalled()
  })

  it('pause button is yellow and appears before delete in settings menu', async () => {
    vi.mocked(uptimeApi.getMonitorsSummary).mockResolvedValue([mkSummary({ id: 'm12', name: 'OrderTest' })])

    renderWithProviders(<Uptime />)
    expect(await screen.findByText('OrderTest')).toBeInTheDocument()
    const card = screen.getByText('OrderTest').closest('div') as HTMLElement
    await userEvent.click(within(card).getByTitle('Monitor settings'))

    const configureBtn = within(card).getByText('Configure')
    let menuContainer: HTMLElement | null = configureBtn.parentElement
    while (menuContainer && !menuContainer.className.includes('absolute')) {
      menuContainer = menuContainer.parentElement
    }
    expect(menuContainer).toBeTruthy()
    const buttons = Array.from(menuContainer!.querySelectorAll('button'))
    const pauseBtn = buttons.find(b => b.textContent?.trim() === 'Pause')
    const deleteBtn = buttons.find(b => b.textContent?.trim() === 'Delete')
    expect(pauseBtn).toBeTruthy()
    expect(deleteBtn).toBeTruthy()
    expect(buttons.indexOf(pauseBtn!)).toBeLessThan(buttons.indexOf(deleteBtn!))
    expect(pauseBtn!.className).toContain('text-yellow-600')
  })

  it('deletes monitor when delete confirmed and shows toast', async () => {
    vi.mocked(uptimeApi.getMonitorsSummary).mockResolvedValue([mkSummary({ id: 'm5', name: 'DeleteMe' })])
    vi.mocked(uptimeApi.deleteMonitor).mockResolvedValue()

    const confirmSpy = vi.spyOn(window, 'confirm').mockImplementation(() => true)
    renderWithProviders(<Uptime />)
    expect(await screen.findByText('DeleteMe')).toBeInTheDocument()
    const card = screen.getByText('DeleteMe').closest('div') as HTMLElement
    const settingsBtn = within(card).getByTitle('Monitor settings')
    await userEvent.click(settingsBtn)
    const deleteBtn = within(card).getByText('Delete')
    await userEvent.click(deleteBtn)
    await waitFor(() => expect(uptimeApi.deleteMonitor).toHaveBeenCalledWith('m5'))
    confirmSpy.mockRestore()
  })

  it('opens configure modal and saves changes via updateMonitor', async () => {
    const monitor = mkSummary({ id: 'm6', name: 'ConfigMe', proxy_host_id: 1 })
    vi.mocked(uptimeApi.getMonitorsSummary).mockResolvedValue([monitor])
    vi.mocked(uptimeApi.updateMonitor).mockResolvedValue({
      id: 'm6', name: 'ConfigMe', type: 'http', url: 'http://example.com', interval: 60, enabled: true, status: 'up', latency: 10, max_retries: 6,
    })

    renderWithProviders(<Uptime />)
    expect(await screen.findByText('ConfigMe')).toBeInTheDocument()
    const card = screen.getByText('ConfigMe').closest('div') as HTMLElement
    await userEvent.click(within(card).getByTitle('Monitor settings'))
    await userEvent.click(within(card).getByText('Configure'))
    expect(await screen.findByText('Configure Monitor')).toBeInTheDocument()
    const spinbuttons = screen.getAllByRole('spinbutton')
    const maxRetriesInput = spinbuttons.find(el => el.getAttribute('value') === '3') as HTMLInputElement
    await userEvent.clear(maxRetriesInput)
    await userEvent.type(maxRetriesInput, '6')
    await userEvent.clear(screen.getByLabelText('Name'))
    await userEvent.type(screen.getByLabelText('Name'), 'Renamed Monitor')
    await userEvent.click(screen.getByText('Save Changes'))
    await waitFor(() => expect(uptimeApi.updateMonitor).toHaveBeenCalledWith('m6', { name: 'Renamed Monitor', max_retries: 6, interval: 60 }))
  })

  it('does not call deleteMonitor when canceling delete', async () => {
    vi.mocked(uptimeApi.getMonitorsSummary).mockResolvedValue([mkSummary({ id: 'm7', name: 'DoNotDelete' })])
    vi.mocked(uptimeApi.deleteMonitor).mockResolvedValue()

    const confirmSpy = vi.spyOn(window, 'confirm').mockImplementation(() => false)
    renderWithProviders(<Uptime />)
    expect(await screen.findByText('DoNotDelete')).toBeInTheDocument()
    const card = screen.getByText('DoNotDelete').closest('div') as HTMLElement
    await userEvent.click(within(card).getByTitle('Monitor settings'))
    await userEvent.click(within(card).getByText('Delete'))
    expect(uptimeApi.deleteMonitor).not.toHaveBeenCalled()
    confirmSpy.mockRestore()
  })

  it('shows toast error when toggle update fails', async () => {
    vi.mocked(uptimeApi.getMonitorsSummary).mockResolvedValue([mkSummary({ id: 'm8', name: 'ToggleFail', proxy_host_id: 1 })])
    vi.mocked(uptimeApi.updateMonitor).mockRejectedValue(new Error('Update failed'))

    renderWithProviders(<Uptime />)
    expect(await screen.findByText('ToggleFail')).toBeInTheDocument()
    const card = screen.getByText('ToggleFail').closest('div') as HTMLElement
    await userEvent.click(within(card).getByTitle('Monitor settings'))
    await userEvent.click(within(card).getByText('Pause'))
    const toast = (await import('react-hot-toast')).toast
    await waitFor(() => expect(toast.error).toHaveBeenCalled())
  })

  it('separates monitors into Proxy Hosts, Remote Servers and Other sections', async () => {
    vi.mocked(uptimeApi.getMonitorsSummary).mockResolvedValue([
      mkSummary({ id: 'm9', name: 'ProxyMon', proxy_host_id: 1 }),
      mkSummary({ id: 'm10', name: 'RemoteMon', remote_server_id: 2 }),
      mkSummary({ id: 'm11', name: 'OtherMon' }),
    ])

    renderWithProviders(<Uptime />)
    expect(await screen.findByText('Proxy Hosts')).toBeInTheDocument()
    expect(screen.getByText('Remote Servers')).toBeInTheDocument()
    expect(screen.getByText('Other Monitors')).toBeInTheDocument()
    expect(screen.getByText('ProxyMon')).toBeInTheDocument()
    expect(screen.getByText('RemoteMon')).toBeInTheDocument()
    expect(screen.getByText('OtherMon')).toBeInTheDocument()
  })

  it('shows CHECKING... state for pending monitor with no history', async () => {
    vi.mocked(uptimeApi.getMonitorsSummary).mockResolvedValue([
      mkSummary({ id: 'm13', name: 'PendingMonitor', status: 'pending', last_check: null, latency: 0 }),
    ])

    renderWithProviders(<Uptime />)
    expect(await screen.findByText('PendingMonitor')).toBeInTheDocument()
    const badge = screen.getByTestId('status-badge')
    expect(badge).toHaveAttribute('data-status', 'pending')
    expect(badge).toHaveAttribute('role', 'status')
    expect(badge.textContent).toContain('CHECKING...')
    expect(badge.className).toContain('bg-amber-100')
    expect(badge.className).toContain('animate-pulse')
    expect(screen.getByText('Waiting for first check...')).toBeInTheDocument()
  })

  it('treats pending monitor with heartbeat history as normal (not pending)', async () => {
    vi.mocked(uptimeApi.getMonitorsSummary).mockResolvedValue([
      mkSummary({
        id: 'm14',
        name: 'PendingWithHistory',
        status: 'pending',
        recent_beats: [{ status: 'up', latency: 10, created_at: new Date().toISOString() }],
      }),
    ])

    renderWithProviders(<Uptime />)
    expect(await screen.findByText('PendingWithHistory')).toBeInTheDocument()
    await waitFor(() => {
      const badge = screen.getByTestId('status-badge')
      expect(badge.textContent).not.toContain('CHECKING...')
      expect(badge.className).toContain('bg-green-100')
    })
  })

  it('shows DOWN indicator for down monitor (no regression)', async () => {
    vi.mocked(uptimeApi.getMonitorsSummary).mockResolvedValue([
      mkSummary({ id: 'm15', name: 'DownMonitor', status: 'down', latency: 0 }),
    ])

    renderWithProviders(<Uptime />)
    expect(await screen.findByText('DownMonitor')).toBeInTheDocument()
    const badge = screen.getByTestId('status-badge')
    expect(badge).toHaveAttribute('data-status', 'down')
    expect(badge.textContent).toContain('DOWN')
    expect(badge.className).toContain('bg-red-100')
  })
})
