import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { AxiosError, type AxiosResponse } from 'axios'
import { toast } from 'react-hot-toast'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { checkMonitor, deleteMonitor, syncMonitors, type MonitorSummary } from '../../api/uptime'
import { renderWithQueryClient } from '../../test-utils/renderWithQueryClient'
import Uptime from '../Uptime'

const roleState = vi.hoisted(() => ({ role: 'admin' as 'admin' | 'user' }))

vi.mock('../../hooks/useAuth', () => ({
  useAuth: () => ({ user: { user_id: 1, role: roleState.role } }),
}))

const monitors = vi.hoisted(() => ({ list: [] as MonitorSummary[] }))

vi.mock('../../hooks/useUptimeSummary', () => ({
  useUptimeSummary: () => ({ data: monitors.list, isLoading: false }),
}))

vi.mock('../../api/uptime', () => ({
  updateMonitor: vi.fn(),
  deleteMonitor: vi.fn(),
  checkMonitor: vi.fn(),
  createMonitor: vi.fn(),
  syncMonitors: vi.fn(),
}))

vi.mock('react-hot-toast', () => ({
  toast: { success: vi.fn(), error: vi.fn(), loading: vi.fn(), dismiss: vi.fn() },
}))

// Titles are translation keys or English text depending on the i18n setup, so match either.
const HEALTH_CHECK_TITLE = /triggerHealthCheck|Trigger immediate health check/i
const SETTINGS_TITLE = /monitorSettings|Monitor settings/i
const ADMIN_ONLY_MESSAGE = 'Only administrators can change this setting.'

const sampleMonitor: MonitorSummary = {
  id: 'm-1',
  name: 'Media Server',
  type: 'http',
  url: 'https://media.example.com',
  enabled: true,
  status: 'up',
  latency: 12,
  last_check: null,
  interval: 60,
  proxy_host_id: 7,
  remote_server_id: null,
  uptime_24h: 99.5,
  recent_beats: [{ status: 'up', latency: 12, created_at: '2026-11-13T10:00:00Z' }],
  max_retries: 3,
}

describe('Uptime page roles', () => {
  beforeEach(() => {
    monitors.list = [sampleMonitor]
  })

  describe('as an administrator', () => {
    beforeEach(() => {
      roleState.role = 'admin'
    })

    it('shows the sync and add controls and the per-monitor controls', () => {
      renderWithQueryClient(<Uptime />)

      expect(screen.getByTestId('sync-button')).toBeInTheDocument()
      expect(screen.getByTestId('add-monitor-button')).toBeInTheDocument()
      expect(screen.getByTitle(HEALTH_CHECK_TITLE)).toBeInTheDocument()
      expect(screen.getByTitle(SETTINGS_TITLE)).toBeInTheDocument()
    })

    // A session whose role was lowered after the page loaded is refused by the server; the page explains why.
    describe('when the server refuses a monitor change', () => {
      const refused = () =>
        new AxiosError('Request failed with status code 403', 'ERR_BAD_REQUEST', undefined, undefined, {
          status: 403,
          statusText: 'Forbidden',
          headers: {},
          config: {},
          data: { error: 'admin privileges required', error_code: 'permissions_admin_only' },
        } as AxiosResponse)

      beforeEach(() => {
        vi.mocked(toast.error).mockClear()
      })

      it('explains a refused manual check', async () => {
        vi.mocked(checkMonitor).mockRejectedValueOnce(refused())
        renderWithQueryClient(<Uptime />)

        await userEvent.click(screen.getByTitle(HEALTH_CHECK_TITLE))

        await waitFor(() => expect(toast.error).toHaveBeenCalledWith(ADMIN_ONLY_MESSAGE))
      })

      it('explains a refused delete', async () => {
        vi.mocked(deleteMonitor).mockRejectedValueOnce(refused())
        const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(true)
        renderWithQueryClient(<Uptime />)

        await userEvent.click(screen.getByTitle(SETTINGS_TITLE))
        await userEvent.click(screen.getByRole('button', { name: /delete/i }))

        await waitFor(() => expect(toast.error).toHaveBeenCalledWith(ADMIN_ONLY_MESSAGE))
        confirmSpy.mockRestore()
      })

      it('explains a refused sync', async () => {
        vi.mocked(syncMonitors).mockRejectedValueOnce(refused())
        renderWithQueryClient(<Uptime />)

        await userEvent.click(screen.getByTestId('sync-button'))

        await waitFor(() => expect(toast.error).toHaveBeenCalledWith(ADMIN_ONLY_MESSAGE))
      })
    })
  })

  describe('as a standard user', () => {
    beforeEach(() => {
      roleState.role = 'user'
    })

    it('hides the sync and add controls', () => {
      renderWithQueryClient(<Uptime />)

      expect(screen.queryByTestId('sync-button')).not.toBeInTheDocument()
      expect(screen.queryByTestId('add-monitor-button')).not.toBeInTheDocument()
    })

    it('hides the manual check and the settings menu on each monitor card', () => {
      renderWithQueryClient(<Uptime />)

      expect(screen.queryByTitle(HEALTH_CHECK_TITLE)).not.toBeInTheDocument()
      expect(screen.queryByTitle(SETTINGS_TITLE)).not.toBeInTheDocument()
      expect(screen.queryByText(/common\.delete|^Delete$/)).not.toBeInTheDocument()
    })

    it('keeps the monitor status and heartbeat readable', () => {
      renderWithQueryClient(<Uptime />)

      expect(screen.getByText('Media Server')).toBeInTheDocument()
      expect(screen.getByTestId('status-badge')).toHaveAttribute('data-status', 'up')
      expect(screen.getByTestId('heartbeat-bar')).toBeInTheDocument()
      expect(screen.getByTestId('uptime-24h')).toHaveTextContent('99.5%')
    })
  })
})
