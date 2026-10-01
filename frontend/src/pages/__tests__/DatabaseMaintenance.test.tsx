import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import * as api from '../../api/databaseMaintenance'
import { toast } from '../../utils/toast'
import DatabaseMaintenancePage from '../DatabaseMaintenance'

vi.mock('../../api/databaseMaintenance')
vi.mock('../../utils/toast', () => ({ toast: { success: vi.fn(), error: vi.fn() } }))

const GB = 1_000_000_000

const makeStatus = (overrides: Partial<api.DatabaseStatus> = {}): api.DatabaseStatus => ({
  size_bytes: 120_000_000,
  wal_bytes: 4_194_304,
  reclaimable_bytes: 0,
  auto_vacuum: 'incremental',
  env_mode: 'auto',
  compact_requested: false,
  can_request_optimize: false,
  disk_free_bytes: 52 * GB,
  last_result: null,
  notice: null,
  ...overrides,
})

const legacy = (overrides: Partial<api.DatabaseStatus> = {}) =>
  makeStatus({ auto_vacuum: 'none', reclaimable_bytes: 1.7 * GB, can_request_optimize: true, ...overrides })

const renderPage = () => {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={queryClient}>
      <DatabaseMaintenancePage />
    </QueryClientProvider>
  )
}

const reclaimButton = () => screen.getByRole('button', { name: /reclaim space on next restart/i })

const expectExplainer = () => {
  expect(screen.getByRole('heading', { level: 3, name: 'Database' })).toBeInTheDocument()
  expect(screen.getByText(/looks after its database by itself/i)).toBeInTheDocument()
  expect(screen.getByText(/^Checked at every start/)).toBeInTheDocument()
  expect(screen.getByText(/^Converted once when worthwhile/)).toBeInTheDocument()
  expect(screen.getByText(/^Handed back after each hourly cleanup/)).toBeInTheDocument()
}

describe('Database page', () => {
  beforeEach(() => vi.clearAllMocks())

  it('shows the explainer and a loading line while the status loads', () => {
    vi.mocked(api.getDatabaseStatus).mockReturnValue(new Promise(() => {}))
    renderPage()
    expectExplainer()
    expect(screen.getByText(/loading database information/i)).toBeInTheDocument()
    expect(screen.queryByRole('alert')).toBeNull()
  })

  it('is never empty in the quiet incremental state, with exactly one "nothing to do"', async () => {
    vi.mocked(api.getDatabaseStatus).mockResolvedValue(makeStatus())
    renderPage()

    expect(await screen.findByText('No optimization has been needed so far.')).toBeInTheDocument()
    expectExplainer()
    expect(screen.getByRole('region', { name: 'Database' })).toHaveTextContent('120.0 MB')
    expect(screen.getByText('Write-ahead log')).toBeInTheDocument()
    expect(screen.getByText('Free disk space')).toBeInTheDocument()
    expect(screen.getByText(/automatic: freed space goes back to your disk on its own/i)).toBeInTheDocument()
    expect(screen.getByText('Reclaim space now (optional)')).toBeInTheDocument()
    expect(reclaimButton()).toBeDisabled()
    expect(screen.getAllByText(/nothing to do/i)).toHaveLength(1)
    expect(screen.queryByRole('status')).toBeNull()
    expect(screen.queryByRole('alert')).toBeNull()
  })

  it('shows "Not optimized yet" next to an enabled button for a legacy database', async () => {
    vi.mocked(api.getDatabaseStatus).mockResolvedValue(legacy())
    renderPage()
    expect(await screen.findByText('Not optimized yet.')).toBeInTheDocument()
    expect(screen.queryByText(/nothing to do/i)).toBeNull()
    expect(reclaimButton()).toBeEnabled()
  })

  it('shows the scheduled state with Undo and no "nothing to do"', async () => {
    vi.mocked(api.getDatabaseStatus).mockResolvedValue(legacy({ compact_requested: true, can_request_optimize: false }))
    renderPage()
    expect(await screen.findByText('Not optimized yet.')).toBeInTheDocument()
    expect(screen.getByText(/scheduled - the space is reclaimed the next time charon starts/i)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /undo/i })).toBeEnabled()
    expect(screen.queryByText(/nothing to do/i)).toBeNull()
  })

  it('renders a passive notice as status, not alert', async () => {
    vi.mocked(api.getDatabaseStatus).mockResolvedValue(
      legacy({ notice: { code: 'restart_to_optimize', severity: 'info', reclaimable_bytes: 1.7 * GB } })
    )
    renderPage()
    expect(await screen.findByRole('status')).toHaveTextContent(/reclaim about 1\.7 GB automatically/i)
    expect(screen.queryByRole('alert')).toBeNull()
  })

  it('schedules and undoes the request', async () => {
    const user = userEvent.setup()
    let requested = false
    vi.mocked(api.getDatabaseStatus).mockImplementation(async () =>
      legacy({ compact_requested: requested, can_request_optimize: !requested })
    )
    vi.mocked(api.requestOptimizeOnRestart).mockImplementation(async () => {
      requested = true
      return { requested: true }
    })
    vi.mocked(api.cancelOptimizeOnRestart).mockImplementation(async () => {
      requested = false
      return { requested: false }
    })
    renderPage()

    await user.click(await screen.findByRole('button', { name: /reclaim space on next restart/i }))
    expect(await screen.findByText(/scheduled - the space is reclaimed/i)).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: /undo/i }))
    expect(await screen.findByRole('button', { name: /reclaim space on next restart/i })).toBeEnabled()
    expect(api.cancelOptimizeOnRestart).toHaveBeenCalledTimes(1)
  })

  it('shows the server message in a toast when the request is rejected (409)', async () => {
    const user = userEvent.setup()
    vi.mocked(api.getDatabaseStatus).mockResolvedValue(legacy())
    vi.mocked(api.requestOptimizeOnRestart).mockRejectedValue(new Error('already optimized'))
    renderPage()
    await user.click(await screen.findByRole('button', { name: /reclaim space on next restart/i }))
    await waitFor(() => expect(toast.error).toHaveBeenCalledWith(expect.stringContaining('already optimized')))
    await waitFor(() => expect(api.getDatabaseStatus).toHaveBeenCalledTimes(2))
  })

  it('shows a named load-error alert with Retry, the explainer and no notice', async () => {
    const user = userEvent.setup()
    vi.mocked(api.getDatabaseStatus).mockRejectedValueOnce(new Error('boom'))
    renderPage()

    const alert = await screen.findByRole('alert', { name: 'Database information unavailable' })
    expect(alert).toHaveTextContent('Could not load database information')
    expectExplainer()
    expect(screen.queryByRole('status')).toBeNull()

    vi.mocked(api.getDatabaseStatus).mockResolvedValue(makeStatus())
    await user.click(screen.getByRole('button', { name: /retry/i }))
    expect(await screen.findByText('No optimization has been needed so far.')).toBeInTheDocument()
    expect(screen.queryByRole('alert')).toBeNull()
  })
})
