import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, it, expect, vi, beforeEach } from 'vitest'

import { renderWithQueryClient } from '../../test-utils/renderWithQueryClient'
import Uptime from '../Uptime'

import type { UptimeMonitor } from '../../api/uptime'

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
        'uptime.maxRetries': 'Max Retries',
        'uptime.maxRetriesHelper': 'Number of retries before marking as down',
        'uptime.checkInterval': 'Check Interval',
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
        'uptime.last60Checks': 'Last 60 Checks',
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
  getMonitorHistory: vi.fn(),
  updateMonitor: vi.fn(),
  deleteMonitor: vi.fn(),
  checkMonitor: vi.fn(),
  createMonitor: vi.fn(),
  syncMonitors: vi.fn(),
}))

const mockProxyHostMonitor: UptimeMonitor = {
  id: 'monitor-1',
  name: 'Example App',
  type: 'http',
  url: 'https://app.example.com',
  interval: 60,
  enabled: true,
  status: 'up',
  latency: 42,
  max_retries: 3,
  proxy_host_id: 1,
  last_check: new Date().toISOString(),
}

const mockRemoteServerMonitor: UptimeMonitor = {
  id: 'monitor-2',
  name: 'Database Server',
  type: 'tcp',
  url: 'db.example.com:5432',
  interval: 60,
  enabled: true,
  status: 'up',
  latency: 15,
  max_retries: 3,
  remote_server_id: 1,
  last_check: new Date().toISOString(),
}

const mockOtherMonitor: UptimeMonitor = {
  id: 'monitor-3',
  name: 'External API',
  type: 'http',
  url: 'https://api.external.com/health',
  interval: 120,
  enabled: true,
  status: 'down',
  latency: 0,
  max_retries: 5,
  last_check: new Date().toISOString(),
}

const mockPausedMonitor: UptimeMonitor = {
  id: 'monitor-4',
  name: 'Paused Service',
  type: 'http',
  url: 'https://paused.example.com',
  interval: 60,
  enabled: false,
  status: 'up',
  latency: 100,
  max_retries: 3,
}

describe('Uptime page', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('renders loading state', async () => {
    const { getMonitors } = await import('../../api/uptime')
    vi.mocked(getMonitors).mockImplementation(() => new Promise(() => {}))

    renderWithQueryClient(<Uptime />)

    expect(screen.getByText('Loading monitors...')).toBeInTheDocument()
  })

  it('falls back to DOWN status when monitor status is unknown', async () => {
    const { getMonitors, getMonitorHistory } = await import('../../api/uptime')
    const monitor = {
      id: 'm-unknown-status', name: 'UnknownStatusMonitor', url: 'http://example.com', type: 'http', interval: 60, enabled: true,
      status: 'mystery', last_check: new Date().toISOString(), latency: 10, max_retries: 3,
    }
    vi.mocked(getMonitors).mockResolvedValue([monitor])
    vi.mocked(getMonitorHistory).mockResolvedValue([])

    renderWithQueryClient(<Uptime />)
    expect(await screen.findByText('UnknownStatusMonitor')).toBeInTheDocument()

    const badge = screen.getByTestId('status-badge')
    expect(badge).toHaveAttribute('data-status', 'down')
    expect(badge).toHaveTextContent('DOWN')
  })

  it('renders empty state when no monitors exist', async () => {
    const { getMonitors } = await import('../../api/uptime')
    vi.mocked(getMonitors).mockResolvedValue([])

    renderWithQueryClient(<Uptime />)

    await waitFor(() => {
      expect(screen.getByText('No monitors found')).toBeInTheDocument()
    })
  })

  it('renders page title and header actions', async () => {
    const { getMonitors } = await import('../../api/uptime')
    vi.mocked(getMonitors).mockResolvedValue([])

    renderWithQueryClient(<Uptime />)

    await waitFor(() => {
      expect(screen.getByText('Uptime Monitoring')).toBeInTheDocument()
    })
    expect(screen.getByTestId('sync-button')).toBeInTheDocument()
    expect(screen.getByTestId('add-monitor-button')).toBeInTheDocument()
  })

  it('renders monitors grouped by type', async () => {
    const { getMonitors, getMonitorHistory } = await import('../../api/uptime')
    vi.mocked(getMonitors).mockResolvedValue([
      mockProxyHostMonitor,
      mockRemoteServerMonitor,
      mockOtherMonitor,
    ])
    vi.mocked(getMonitorHistory).mockResolvedValue([])

    renderWithQueryClient(<Uptime />)

    await waitFor(() => {
      expect(screen.getByText('Proxy Hosts')).toBeInTheDocument()
    })
    expect(screen.getByText('Remote Servers')).toBeInTheDocument()
    expect(screen.getByText('Other Monitors')).toBeInTheDocument()
  })

  it('opens create monitor modal when add button clicked', async () => {
    const user = userEvent.setup()
    const { getMonitors } = await import('../../api/uptime')
    vi.mocked(getMonitors).mockResolvedValue([])

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

  it('displays monitor cards with status badges', async () => {
    const { getMonitors, getMonitorHistory } = await import('../../api/uptime')
    vi.mocked(getMonitors).mockResolvedValue([mockProxyHostMonitor, mockOtherMonitor])
    vi.mocked(getMonitorHistory).mockResolvedValue([])

    renderWithQueryClient(<Uptime />)

    await waitFor(() => {
      expect(screen.getByText('Example App')).toBeInTheDocument()
    })
    expect(screen.getByText('External API')).toBeInTheDocument()

    // Check status badges exist
    const statusBadges = screen.getAllByTestId('status-badge')
    expect(statusBadges.length).toBe(2)
  })

  it('displays paused status for disabled monitors', async () => {
    const { getMonitors, getMonitorHistory } = await import('../../api/uptime')
    vi.mocked(getMonitors).mockResolvedValue([mockPausedMonitor])
    vi.mocked(getMonitorHistory).mockResolvedValue([])

    renderWithQueryClient(<Uptime />)

    await waitFor(() => {
      expect(screen.getByText('Paused Service')).toBeInTheDocument()
    })
    expect(screen.getByTestId('status-badge')).toHaveAttribute('data-status', 'paused')
  })

  it('shows latency and last check information', async () => {
    const { getMonitors, getMonitorHistory } = await import('../../api/uptime')
    vi.mocked(getMonitors).mockResolvedValue([mockProxyHostMonitor])
    vi.mocked(getMonitorHistory).mockResolvedValue([])

    renderWithQueryClient(<Uptime />)

    await waitFor(() => {
      expect(screen.getByText('42ms')).toBeInTheDocument()
    })
    expect(screen.getByTestId('last-check')).toBeInTheDocument()
  })

  it('handles sync button click', async () => {
    const user = userEvent.setup()
    const { getMonitors, syncMonitors } = await import('../../api/uptime')
    vi.mocked(getMonitors).mockResolvedValue([])
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
