import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, it, expect, vi, beforeEach } from 'vitest'

import { renderWithQueryClient } from '../../test-utils/renderWithQueryClient'
import Uptime from '../Uptime'

import type { MonitorSummary } from '../../api/uptime'

vi.mock('react-hot-toast', () => ({
  toast: { success: vi.fn(), error: vi.fn(), loading: vi.fn(), dismiss: vi.fn() },
}))

// Mock react-i18next
vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string, options?: Record<string, unknown>) => {
      const translations: Record<string, string> = {
        'uptime.title': 'Uptime Monitoring',
        'uptime.loadingMonitors': 'Loading monitors...',
        'uptime.noMonitorsFound': 'No monitors found',
        'uptime.syncWithHosts': 'Sync with Hosts',
        'uptime.syncing': 'Syncing...',
        'uptime.addMonitor': 'Add Monitor',
        'uptime.autoRefreshing': 'Auto-refreshing every 30s',
        'uptime.proxyHosts': 'Proxy Hosts',
        'uptime.remoteServers': 'Remote Servers',
        'uptime.otherMonitors': 'Other Monitors',
        'uptime.latency': 'Latency',
        'uptime.lastCheck': 'Last Check',
        'uptime.never': 'Never',
        'uptime.configureMonitor': 'Configure Monitor',
        'uptime.createMonitor': 'Create Monitor',
        'uptime.monitorSettings': 'Monitor Settings',
        'uptime.triggerHealthCheck': 'Trigger Health Check',
        'uptime.paused': 'Paused',
        'uptime.pause': 'Pause',
        'uptime.unpause': 'Resume',
        'uptime.unpaused': 'Unpaused',
        'uptime.maxRetries': 'Max Retries',
        'uptime.maxRetriesHelper': 'Number of retries before marking as down',
        'uptime.checkInterval': 'Check Interval',
        'uptime.checkIntervalHelper': 'Minimum 30 seconds',
        'uptime.checkQueueFull': 'Check queue full, try again in a moment',
        'uptime.recentChecks': 'Recent health checks',
        'uptime.saveChanges': 'Save Changes',
        'uptime.monitorUrl': 'Monitor URL',
        'uptime.urlPlaceholder': 'https://example.com',
        'uptime.urlPlaceholderHttp': 'https://example.com',
        'uptime.urlPlaceholderTcp': '192.168.1.1:8080',
        'uptime.urlHelperHttp': 'Enter the full URL including the scheme',
        'uptime.urlHelperTcp': 'Enter as host:port with no scheme prefix',
        'uptime.invalidTcpFormat': 'TCP monitors require host:port format. Remove the scheme prefix.',
        'uptime.monitorType': 'Monitor Type',
        'uptime.monitorTypeHttp': 'HTTP(S)',
        'uptime.monitorTypeTcp': 'TCP',
        'uptime.noHistoryAvailable': 'No history available',
        'uptime.pending': 'CHECKING...',
        'uptime.pendingFirstCheck': 'Waiting for first check...',
        'uptime.healthCheckTriggered': 'Health check triggered',
        'uptime.failedToTriggerCheck': 'Failed to trigger health check',
        'uptime.monitorCreated': 'Monitor created',
        'uptime.syncComplete': 'Sync complete',
        'errors.genericError': 'Something went wrong',
        'common.configure': 'Configure',
        'common.cancel': 'Cancel',
        'common.delete': 'Delete',
        'common.create': 'Create',
        'common.saving': 'Saving...',
        'common.name': 'Name',
        'common.close': 'Close',
      }
      if (options && typeof options === 'object') {
        let result = translations[key] || key
        for (const [k, v] of Object.entries(options)) {
          result = result.replace(`{{${k}}}`, String(v))
        }
        return result
      }
      return translations[key] || key
    },
  }),
}))

// Mock uptime API
vi.mock('../../api/uptime', () => ({
  getMonitors: vi.fn(),
  getMonitorsSummary: vi.fn(),
  getUptimeHealth: vi.fn(),
  getMonitorHistory: vi.fn(),
  updateMonitor: vi.fn(),
  deleteMonitor: vi.fn(),
  checkMonitor: vi.fn(),
  createMonitor: vi.fn(),
  syncMonitors: vi.fn(),
}))

const baseSummary: Omit<MonitorSummary, 'id' | 'name'> = {
  type: 'http',
  url: 'https://app.example.com',
  interval: 60,
  enabled: true,
  status: 'up',
  latency: 42,
  last_check: new Date().toISOString(),
  proxy_host_id: null,
  remote_server_id: null,
  uptime_24h: null,
  recent_beats: [],
}

const mockProxyHostMonitor: MonitorSummary = {
  ...baseSummary,
  id: 'monitor-1',
  name: 'Example App',
  proxy_host_id: 1,
}

const mockRemoteServerMonitor: MonitorSummary = {
  ...baseSummary,
  id: 'monitor-2',
  name: 'Database Server',
  type: 'tcp',
  url: 'db.example.com:5432',
  latency: 15,
  remote_server_id: 1,
}

const mockOtherMonitor: MonitorSummary = {
  ...baseSummary,
  id: 'monitor-3',
  name: 'External API',
  url: 'https://api.external.com/health',
  status: 'down',
  latency: 0,
}

const mockPausedMonitor: MonitorSummary = {
  ...baseSummary,
  id: 'monitor-4',
  name: 'Paused Service',
  url: 'https://paused.example.com',
  enabled: false,
  latency: 100,
  last_check: null,
}

describe('Uptime page', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('renders loading state', async () => {
    const { getMonitorsSummary } = await import('../../api/uptime')
    vi.mocked(getMonitorsSummary).mockImplementation(() => new Promise(() => {}))

    renderWithQueryClient(<Uptime />)

    expect(screen.getByText('Loading monitors...')).toBeInTheDocument()
  })

  it('falls back to DOWN status when monitor status is unknown', async () => {
    const { getMonitorsSummary } = await import('../../api/uptime')
    vi.mocked(getMonitorsSummary).mockResolvedValue([
      { ...baseSummary, id: 'm-unknown-status', name: 'UnknownStatusMonitor', url: 'http://example.com', status: 'mystery', latency: 10 },
    ])

    renderWithQueryClient(<Uptime />)
    expect(await screen.findByText('UnknownStatusMonitor')).toBeInTheDocument()

    const badge = screen.getByTestId('status-badge')
    expect(badge).toHaveAttribute('data-status', 'down')
    expect(badge).toHaveTextContent('DOWN')
  })

  it('renders empty state when no monitors exist', async () => {
    const { getMonitorsSummary } = await import('../../api/uptime')
    vi.mocked(getMonitorsSummary).mockResolvedValue([])

    renderWithQueryClient(<Uptime />)

    await waitFor(() => {
      expect(screen.getByText('No monitors found')).toBeInTheDocument()
    })
  })

  it('renders page title and header actions', async () => {
    const { getMonitorsSummary } = await import('../../api/uptime')
    vi.mocked(getMonitorsSummary).mockResolvedValue([])

    renderWithQueryClient(<Uptime />)

    await waitFor(() => {
      expect(screen.getByText('Uptime Monitoring')).toBeInTheDocument()
    })
    expect(screen.getByTestId('sync-button')).toBeInTheDocument()
    expect(screen.getByTestId('add-monitor-button')).toBeInTheDocument()
  })

  it('renders monitors grouped by type', async () => {
    const { getMonitorsSummary } = await import('../../api/uptime')
    vi.mocked(getMonitorsSummary).mockResolvedValue([
      mockProxyHostMonitor,
      mockRemoteServerMonitor,
      mockOtherMonitor,
    ])

    renderWithQueryClient(<Uptime />)

    await waitFor(() => {
      expect(screen.getByText('Proxy Hosts')).toBeInTheDocument()
    })
    expect(screen.getByText('Remote Servers')).toBeInTheDocument()
    expect(screen.getByText('Other Monitors')).toBeInTheDocument()
  })

  it('does not issue any per-monitor history request on load', async () => {
    const { getMonitorsSummary, getMonitorHistory } = await import('../../api/uptime')
    vi.mocked(getMonitorsSummary).mockResolvedValue([mockProxyHostMonitor, mockOtherMonitor])

    renderWithQueryClient(<Uptime />)

    await waitFor(() => {
      expect(screen.getByText('Example App')).toBeInTheDocument()
    })
    expect(getMonitorHistory).not.toHaveBeenCalled()
    expect(getMonitorsSummary).toHaveBeenCalledTimes(1)
  })

  it('renders status badge, latency and sparkline from the summary payload', async () => {
    const { getMonitorsSummary } = await import('../../api/uptime')
    vi.mocked(getMonitorsSummary).mockResolvedValue([
      {
        ...baseSummary,
        id: 'm-beats',
        name: 'HasBeats',
        latency: 77,
        uptime_24h: 99.42,
        recent_beats: [
          { status: 'up', latency: 12, created_at: new Date(Date.now() - 60000).toISOString() },
          { status: 'down', latency: 0, created_at: new Date(Date.now() - 30000).toISOString() },
          { status: 'up', latency: 15, created_at: new Date().toISOString() },
        ],
      },
    ])

    renderWithQueryClient(<Uptime />)

    expect(await screen.findByText('HasBeats')).toBeInTheDocument()
    expect(screen.getByText('77ms')).toBeInTheDocument()
    expect(screen.getByTestId('heartbeat-bar')).toBeInTheDocument()
    expect(screen.getByTestId('uptime-24h')).toHaveTextContent('99.4% · 24h')
    // Trailing beat is `up` -> card resolves to UP.
    expect(screen.getByTestId('status-badge')).toHaveAttribute('data-status', 'up')
    const barTitles = Array.from(document.querySelectorAll('[title*="Status:"]'))
    expect(barTitles.some((el) => (el.getAttribute('title') || '').includes('Status: DOWN'))).toBe(true)
  })

  it('opens create monitor modal when add button clicked', async () => {
    const user = userEvent.setup()
    const { getMonitorsSummary } = await import('../../api/uptime')
    vi.mocked(getMonitorsSummary).mockResolvedValue([])

    renderWithQueryClient(<Uptime />)

    await waitFor(() => {
      expect(screen.getByTestId('add-monitor-button')).toBeInTheDocument()
    })

    await user.click(screen.getByTestId('add-monitor-button'))

    await waitFor(() => {
      expect(screen.getByText('Create Monitor')).toBeInTheDocument()
    })
    expect(screen.getByLabelText(/Name/)).toBeInTheDocument()
    expect(screen.getByLabelText(/Monitor URL/)).toBeInTheDocument()
    expect(screen.getByLabelText(/Monitor Type/)).toBeInTheDocument()
  })

  it('interval field advertises the 30s floor and clamps a below-floor value on blur', async () => {
    const user = userEvent.setup()
    const { getMonitorsSummary } = await import('../../api/uptime')
    vi.mocked(getMonitorsSummary).mockResolvedValue([])

    renderWithQueryClient(<Uptime />)
    await waitFor(() => expect(screen.getByTestId('add-monitor-button')).toBeInTheDocument())
    await user.click(screen.getByTestId('add-monitor-button'))
    await waitFor(() => expect(screen.getByText('Create Monitor')).toBeInTheDocument())

    const interval = document.getElementById('create-monitor-interval') as HTMLInputElement
    expect(interval).toHaveAttribute('min', '30')
    expect(document.getElementById('create-monitor-interval-helper')).toHaveTextContent('Minimum 30 seconds')

    await user.clear(interval)
    await user.type(interval, '10')
    interval.blur()

    await waitFor(() => expect(interval.value).toBe('30'))
  })

  it('forwards a valid interval unchanged in the create payload', async () => {
    const user = userEvent.setup()
    const { getMonitorsSummary, createMonitor } = await import('../../api/uptime')
    vi.mocked(getMonitorsSummary).mockResolvedValue([])
    vi.mocked(createMonitor).mockResolvedValue({
      id: 'new-1', name: 'Valid', type: 'http', url: 'https://valid.example.test', interval: 45, enabled: true, status: 'pending', latency: 0, max_retries: 3,
    })

    renderWithQueryClient(<Uptime />)
    await waitFor(() => expect(screen.getByTestId('add-monitor-button')).toBeInTheDocument())
    await user.click(screen.getByTestId('add-monitor-button'))
    await waitFor(() => expect(screen.getByText('Create Monitor')).toBeInTheDocument())

    await user.type(screen.getByLabelText(/Name/), 'Valid')
    await user.type(screen.getByLabelText(/Monitor URL/), 'https://valid.example.test')
    const interval = document.getElementById('create-monitor-interval') as HTMLInputElement
    await user.clear(interval)
    await user.type(interval, '45')
    await user.click(screen.getByRole('button', { name: /Create/ }))

    await waitFor(() => {
      expect(createMonitor).toHaveBeenCalledWith(expect.objectContaining({ interval: 45 }))
    })
  })

  it('displays monitor cards with status badges', async () => {
    const { getMonitorsSummary } = await import('../../api/uptime')
    vi.mocked(getMonitorsSummary).mockResolvedValue([mockProxyHostMonitor, mockOtherMonitor])

    renderWithQueryClient(<Uptime />)

    await waitFor(() => {
      expect(screen.getByText('Example App')).toBeInTheDocument()
    })
    expect(screen.getByText('External API')).toBeInTheDocument()

    const statusBadges = screen.getAllByTestId('status-badge')
    expect(statusBadges.length).toBe(2)
  })

  it('displays paused status for disabled monitors', async () => {
    const { getMonitorsSummary } = await import('../../api/uptime')
    vi.mocked(getMonitorsSummary).mockResolvedValue([mockPausedMonitor])

    renderWithQueryClient(<Uptime />)

    await waitFor(() => {
      expect(screen.getByText('Paused Service')).toBeInTheDocument()
    })
    expect(screen.getByTestId('status-badge')).toHaveAttribute('data-status', 'paused')
  })

  it('shows latency and last check information', async () => {
    const { getMonitorsSummary } = await import('../../api/uptime')
    vi.mocked(getMonitorsSummary).mockResolvedValue([mockProxyHostMonitor])

    renderWithQueryClient(<Uptime />)

    await waitFor(() => {
      expect(screen.getByText('42ms')).toBeInTheDocument()
    })
    expect(screen.getByTestId('last-check')).toBeInTheDocument()
  })

  it('surfaces a queue-full message when a manual check returns 503', async () => {
    const user = userEvent.setup()
    const { getMonitorsSummary, checkMonitor } = await import('../../api/uptime')
    const { toast } = await import('react-hot-toast')
    vi.mocked(getMonitorsSummary).mockResolvedValue([mockProxyHostMonitor])
    vi.mocked(checkMonitor).mockRejectedValue({ response: { status: 503 }, message: 'check queue is full, try again' })

    renderWithQueryClient(<Uptime />)
    await waitFor(() => expect(screen.getByText('Example App')).toBeInTheDocument())

    await user.click(screen.getByTitle('Trigger Health Check'))

    await waitFor(() => {
      expect(toast.error).toHaveBeenCalledWith('Check queue full, try again in a moment')
    })
  })

  it('handles sync button click', async () => {
    const user = userEvent.setup()
    const { getMonitorsSummary, syncMonitors } = await import('../../api/uptime')
    vi.mocked(getMonitorsSummary).mockResolvedValue([])
    vi.mocked(syncMonitors).mockResolvedValue({ message: 'Synced 2 monitors' })

    renderWithQueryClient(<Uptime />)

    await waitFor(() => {
      expect(screen.getByTestId('sync-button')).toBeInTheDocument()
    })

    await user.click(screen.getByTestId('sync-button'))

    await waitFor(() => {
      expect(syncMonitors).toHaveBeenCalled()
    })
  })
})
