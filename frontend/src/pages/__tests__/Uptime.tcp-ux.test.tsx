import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, it, expect, vi, beforeEach } from 'vitest'

import { renderWithQueryClient } from '../../test-utils/renderWithQueryClient'
import Uptime from '../Uptime'

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

async function openCreateModal() {
  const { getMonitors } = await import('../../api/uptime')
  vi.mocked(getMonitors).mockResolvedValue([])

  renderWithQueryClient(<Uptime />)

  await waitFor(() => {
    expect(screen.getByTestId('add-monitor-button')).toBeInTheDocument()
  })

  const user = userEvent.setup()
  await user.click(screen.getByTestId('add-monitor-button'))

  await waitFor(() => {
    expect(screen.getByText('Create Monitor')).toBeInTheDocument()
  })

  return user
}

describe('CreateMonitorModal — TCP UX', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('renders HTTP placeholder by default', async () => {
    await openCreateModal()

    const urlInput = screen.getByLabelText(/Monitor URL/)
    expect(urlInput).toHaveAttribute('placeholder', 'https://example.com')
  })

  it('renders TCP placeholder when type is TCP', async () => {
    const user = await openCreateModal()

    const typeSelect = screen.getByLabelText(/Monitor Type/)
    await user.selectOptions(typeSelect, 'tcp')

    await waitFor(() => {
      const urlInput = screen.getByLabelText(/Monitor URL/)
      expect(urlInput).toHaveAttribute('placeholder', '192.168.1.1:8080')
    })
  })

  it('shows HTTP helper text by default', async () => {
    await openCreateModal()

    const helper = document.getElementById('create-monitor-url-helper')
    expect(helper).toBeInTheDocument()
    expect(helper?.textContent).toContain('scheme')
  })

  it('shows TCP helper text when type is TCP', async () => {
    const user = await openCreateModal()

    const typeSelect = screen.getByLabelText(/Monitor Type/)
    await user.selectOptions(typeSelect, 'tcp')

    await waitFor(() => {
      const helper = document.getElementById('create-monitor-url-helper')
      expect(helper?.textContent).toContain('host:port')
    })
  })

  it('shows inline error when tcp:// entered in TCP mode', async () => {
    const user = await openCreateModal()

    const typeSelect = screen.getByLabelText(/Monitor Type/)
    await user.selectOptions(typeSelect, 'tcp')

    const urlInput = screen.getByLabelText(/Monitor URL/)
    await user.type(urlInput, 'tcp://192.168.1.1:8080')

    await waitFor(() => {
      const alert = screen.getByRole('alert')
      expect(alert).toBeInTheDocument()
      expect(alert.textContent).toContain('host:port format')
    })
  })

  it('inline error clears when scheme prefix removed', async () => {
    const user = await openCreateModal()

    const typeSelect = screen.getByLabelText(/Monitor Type/)
    await user.selectOptions(typeSelect, 'tcp')

    const urlInput = screen.getByLabelText(/Monitor URL/)
    await user.type(urlInput, 'tcp://192.168.1.1:8080')

    await waitFor(() => {
      expect(screen.getByRole('alert')).toBeInTheDocument()
    })

    await user.clear(urlInput)
    await user.type(urlInput, '192.168.1.1:8080')

    await waitFor(() => {
      expect(screen.queryByRole('alert')).not.toBeInTheDocument()
    })
  })

  it('inline error clears when type changes from TCP to HTTP', async () => {
    const user = await openCreateModal()

    const typeSelect = screen.getByLabelText(/Monitor Type/)
    await user.selectOptions(typeSelect, 'tcp')

    const urlInput = screen.getByLabelText(/Monitor URL/)
    await user.type(urlInput, 'tcp://192.168.1.1:8080')

    await waitFor(() => {
      expect(screen.getByRole('alert')).toBeInTheDocument()
    })

    await user.selectOptions(typeSelect, 'http')

    await waitFor(() => {
      expect(screen.queryByRole('alert')).not.toBeInTheDocument()
    })
  })

  it('handleSubmit blocked when tcp:// in URL while type is TCP', async () => {
    const { createMonitor } = await import('../../api/uptime')
    const user = await openCreateModal()

    const typeSelect = screen.getByLabelText(/Monitor Type/)
    await user.selectOptions(typeSelect, 'tcp')

    const nameInput = screen.getByLabelText(/Name/)
    await user.type(nameInput, 'My TCP Monitor')

    const urlInput = screen.getByLabelText(/Monitor URL/)
    await user.type(urlInput, 'tcp://192.168.1.1:8080')

    const submitButton = screen.getByRole('button', { name: /Create/ })
    await user.click(submitButton)

    expect(createMonitor).not.toHaveBeenCalled()
  })

  it('handleSubmit proceeds when TCP URL is bare host:port', async () => {
    const { createMonitor } = await import('../../api/uptime')
    vi.mocked(createMonitor).mockResolvedValue({
      id: 'new-1',
      name: 'DB Server',
      type: 'tcp',
      url: '192.168.1.1:5432',
      interval: 60,
      enabled: true,
      status: 'pending',
      latency: 0,
      max_retries: 3,
    })

    const user = await openCreateModal()

    const typeSelect = screen.getByLabelText(/Monitor Type/)
    await user.selectOptions(typeSelect, 'tcp')

    const nameInput = screen.getByLabelText(/Name/)
    await user.type(nameInput, 'DB Server')

    const urlInput = screen.getByLabelText(/Monitor URL/)
    await user.type(urlInput, '192.168.1.1:5432')

    const submitButton = screen.getByRole('button', { name: /Create/ })
    await user.click(submitButton)

    await waitFor(() => {
      expect(createMonitor).toHaveBeenCalledWith(
        expect.objectContaining({ url: '192.168.1.1:5432', type: 'tcp' })
      )
    })
  })

  it('type selector appears before URL input in DOM order', async () => {
    await openCreateModal()

    const typeSelect = screen.getByLabelText(/Monitor Type/)
    const urlInput = screen.getByLabelText(/Monitor URL/)

    // Node.DOCUMENT_POSITION_FOLLOWING (4) means typeSelect comes before urlInput
    const position = typeSelect.compareDocumentPosition(urlInput)
    expect(position & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
  })
})
