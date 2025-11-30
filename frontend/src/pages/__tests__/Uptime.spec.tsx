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
})
