import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { AxiosError, type AxiosResponse } from 'axios'
import { toast } from 'react-hot-toast'
import { describe, it, expect, vi, beforeEach } from 'vitest'

import { createMonitor, getMonitorsSummary } from '../../api/uptime'
import { renderWithQueryClient } from '../../test-utils/renderWithQueryClient'
import Uptime from '../Uptime'

// Components under test read the caller's role; default to an administrator.
vi.mock('../../hooks/useAuth', () => ({
  useAuth: () => ({ user: { user_id: 1, role: 'admin' } }),
}))

vi.mock('react-hot-toast', () => ({
  toast: { success: vi.fn(), error: vi.fn(), loading: vi.fn(), dismiss: vi.fn() },
}))

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

/** A rejected request as the client leaves it: the server's text is copied into the message. */
const rejectedRequest = (status: number, serverText: string, errorCode?: string): AxiosError => {
  const data = errorCode ? { error: serverText, error_code: errorCode } : { error: serverText }
  const response = { status, statusText: '', headers: {}, config: {}, data } as unknown as AxiosResponse
  return new AxiosError(serverText, 'ERR_BAD_REQUEST', undefined, undefined, response)
}

async function submitCreateMonitorWith(err: AxiosError) {
  vi.mocked(getMonitorsSummary).mockResolvedValue([])
  vi.mocked(createMonitor).mockRejectedValueOnce(err)

  renderWithQueryClient(<Uptime />)
  const user = userEvent.setup()
  await user.click(await screen.findByTestId('add-monitor-button'))
  await screen.findByText('Create Monitor')

  await user.type(screen.getByLabelText(/^Name/), 'Metadata check')
  await user.selectOptions(screen.getByLabelText(/^Type/), 'tcp')
  await user.type(screen.getByLabelText(/^URL/), '169.254.169.254:80')
  await user.click(screen.getByRole('button', { name: 'Create' }))
}

describe('CreateMonitorModal destination refusal', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('shows the fixed destination text when the server refuses the destination', async () => {
    await submitCreateMonitorWith(rejectedRequest(400, 'This destination is not allowed.', 'destination_refused'))

    await waitFor(() => {
      expect(toast.error).toHaveBeenCalledWith('This destination is not allowed.')
    })
    expect(createMonitor).toHaveBeenCalledTimes(1)
  })

  it('keeps the server message for a validation error without a code', async () => {
    await submitCreateMonitorWith(rejectedRequest(400, 'interval must be at least 30 seconds'))

    await waitFor(() => {
      expect(toast.error).toHaveBeenCalledWith('interval must be at least 30 seconds')
    })
  })
})
