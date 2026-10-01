import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import * as api from '../../api/databaseMaintenance'
import { toast } from '../../utils/toast'
import { DatabaseMaintenanceBanner, DatabaseMaintenanceCard } from '../DatabaseMaintenance'

vi.mock('../../api/databaseMaintenance')
vi.mock('../../utils/toast', () => ({ toast: { success: vi.fn(), error: vi.fn() } }))

const GB = 1_000_000_000

const makeStatus = (overrides: Partial<api.DatabaseStatus> = {}): api.DatabaseStatus => ({
  size_bytes: 4.8 * GB,
  wal_bytes: 4_194_304,
  reclaimable_bytes: 1.7 * GB,
  auto_vacuum: 'none',
  env_mode: 'auto',
  compact_requested: false,
  can_request_optimize: true,
  disk_free_bytes: 52 * GB,
  last_result: null,
  notice: null,
  ...overrides,
})

const quiet = makeStatus({
  size_bytes: 120_000_000,
  reclaimable_bytes: 0,
  auto_vacuum: 'incremental',
  can_request_optimize: false,
})

const renderUI = () => {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={queryClient}>
      <DatabaseMaintenanceBanner />
      <DatabaseMaintenanceCard />
    </QueryClientProvider>
  )
}

const reclaimButton = () => screen.queryByRole('button', { name: /reclaim space on next restart/i })

describe('Database maintenance card and banner', () => {
  beforeEach(() => vi.clearAllMocks())

  describe('quiet state', () => {
    it('shows only the size: no notice, no alert, no button', async () => {
      vi.mocked(api.getDatabaseStatus).mockResolvedValue(quiet)
      renderUI()
      const card = await screen.findByRole('region', { name: 'Database' })
      expect(card).toHaveTextContent('120.0 MB')
      expect(card).not.toHaveTextContent(/could be reclaimed/i)
      expect(card).not.toHaveTextContent(/next time it starts/i)
      expect(screen.queryByRole('alert')).toBeNull()
      expect(reclaimButton()).toBeNull()
    })

    it('renders nothing when the status is unavailable (non-admin)', async () => {
      vi.mocked(api.getDatabaseStatus).mockRejectedValue(new Error('forbidden'))
      renderUI()
      await waitFor(() => expect(api.getDatabaseStatus).toHaveBeenCalled())
      expect(screen.queryByRole('region')).toBeNull()
      expect(screen.queryByRole('alert')).toBeNull()
    })
  })

  describe('notices', () => {
    it.each([
      ['restart_to_optimize', /shrink the database by about 1\.7 GB the next time it starts.*nothing to do/i],
      ['database_busy', /postponed because the database was in use.*retried on the next start/i],
      ['disabled_by_env', /CHARON_DB_COMPACT_ON_START/],
    ] as const)('%s (info) is a quiet line in the card and not an alert', async (code, text) => {
      vi.mocked(api.getDatabaseStatus).mockResolvedValue(
        makeStatus({ notice: { code, severity: 'info', reclaimable_bytes: 1.7 * GB } })
      )
      renderUI()
      expect(await screen.findByRole('region', { name: 'Database' })).toHaveTextContent(text)
      expect(screen.queryByRole('alert')).toBeNull()
    })

    it('insufficient_disk (warning) is a banner with the shortfall, above the card', async () => {
      vi.mocked(api.getDatabaseStatus).mockResolvedValue(
        makeStatus({
          notice: {
            code: 'insufficient_disk',
            severity: 'warning',
            reclaimable_bytes: 1.7 * GB,
            required_bytes: 9.6 * GB,
            available_bytes: 2 * GB,
          },
        })
      )
      renderUI()
      const alert = await screen.findByRole('alert', { name: /database/i })
      expect(alert).toHaveTextContent(/free up about 7\.6 GB/i)
      expect(alert).toHaveTextContent('9.6 GB')
      expect(alert).toHaveTextContent('2.0 GB')
      expect(alert).toHaveTextContent(/restart/i)
      const card = screen.getByRole('region', { name: 'Database' })
      expect(alert.compareDocumentPosition(card) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
      // The card itself carries no copy of the warning.
      expect(card).not.toHaveTextContent(/free up about/i)
    })

    it('insufficient_disk without figures never shows a negative or NaN shortfall', async () => {
      vi.mocked(api.getDatabaseStatus).mockResolvedValue(
        makeStatus({ notice: { code: 'insufficient_disk', severity: 'warning', reclaimable_bytes: 1 } })
      )
      renderUI()
      expect(await screen.findByRole('alert')).not.toHaveTextContent(/NaN|-\d/)
    })

    it('too_many_failures (warning) is a banner with the plain instructions', async () => {
      vi.mocked(api.getDatabaseStatus).mockResolvedValue(
        makeStatus({ notice: { code: 'too_many_failures', severity: 'warning', reclaimable_bytes: 1.7 * GB } })
      )
      renderUI()
      expect(await screen.findByRole('alert')).toHaveTextContent(/stopped after 3 failed attempts/i)
    })

    it('an unknown warning code renders no banner', async () => {
      vi.mocked(api.getDatabaseStatus).mockResolvedValue(
        makeStatus({
          notice: { code: 'database_busy', severity: 'warning', reclaimable_bytes: 1 },
        })
      )
      renderUI()
      await screen.findByRole('region', { name: 'Database' })
      expect(screen.queryByRole('alert')).toBeNull()
    })
  })

  describe('reclaim button', () => {
    it('is offered with the reclaimable size when can_request_optimize is true', async () => {
      vi.mocked(api.getDatabaseStatus).mockResolvedValue(makeStatus())
      renderUI()
      const card = await screen.findByRole('region', { name: 'Database' })
      expect(card).toHaveTextContent('1.7 GB could be reclaimed')
      expect(await screen.findByRole('button', { name: /reclaim space on next restart/i })).toBeEnabled()
    })

    it('is hidden when auto_vacuum is already incremental', async () => {
      vi.mocked(api.getDatabaseStatus).mockResolvedValue(
        makeStatus({ auto_vacuum: 'incremental', reclaimable_bytes: 500_000_000 })
      )
      renderUI()
      const card = await screen.findByRole('region', { name: 'Database' })
      expect(reclaimButton()).toBeNull()
      expect(card).not.toHaveTextContent(/could be reclaimed/i)
    })

    it('is hidden below the 100 MB floor', async () => {
      vi.mocked(api.getDatabaseStatus).mockResolvedValue(
        makeStatus({ reclaimable_bytes: 50_000_000, can_request_optimize: false })
      )
      renderUI()
      await screen.findByRole('region', { name: 'Database' })
      expect(reclaimButton()).toBeNull()
    })

    it('is hidden when the server says no for a reason other than the env switch', async () => {
      vi.mocked(api.getDatabaseStatus).mockResolvedValue(makeStatus({ can_request_optimize: false }))
      renderUI()
      await screen.findByRole('region', { name: 'Database' })
      expect(reclaimButton()).toBeNull()
    })

    it('is shown disabled with the reason when the env override is off', async () => {
      vi.mocked(api.getDatabaseStatus).mockResolvedValue(
        makeStatus({ env_mode: 'off', can_request_optimize: false })
      )
      renderUI()
      const button = await screen.findByRole('button', { name: /reclaim space on next restart/i })
      expect(button).toBeDisabled()
      expect(screen.getByRole('region', { name: 'Database' })).toHaveTextContent(
        /disabled by CHARON_DB_COMPACT_ON_START=off/i
      )
      expect(api.requestOptimizeOnRestart).not.toHaveBeenCalled()
    })

    it('schedules on click, shows the scheduled message and Undo, and undo restores the button', async () => {
      const user = userEvent.setup()
      let requested = false
      vi.mocked(api.getDatabaseStatus).mockImplementation(async () => makeStatus({ compact_requested: requested }))
      vi.mocked(api.requestOptimizeOnRestart).mockImplementation(async () => {
        requested = true
        return { requested: true }
      })
      vi.mocked(api.cancelOptimizeOnRestart).mockImplementation(async () => {
        requested = false
        return { requested: false }
      })
      renderUI()

      await user.click(await screen.findByRole('button', { name: /reclaim space on next restart/i }))
      expect(api.requestOptimizeOnRestart).toHaveBeenCalledTimes(1)
      const card = screen.getByRole('region', { name: 'Database' })
      await waitFor(() =>
        expect(card).toHaveTextContent(/scheduled - the space is reclaimed the next time charon starts/i)
      )
      expect(reclaimButton()).toBeNull()

      await user.click(screen.getByRole('button', { name: /undo/i }))
      expect(api.cancelOptimizeOnRestart).toHaveBeenCalledTimes(1)
      expect(await screen.findByRole('button', { name: /reclaim space on next restart/i })).toBeEnabled()
    })

    it('shows Undo but no false "scheduled" promise when requested while the env is off', async () => {
      vi.mocked(api.getDatabaseStatus).mockResolvedValue(
        makeStatus({
          env_mode: 'off',
          compact_requested: true,
          can_request_optimize: false,
          notice: { code: 'disabled_by_env', severity: 'info', reclaimable_bytes: 1.7 * GB },
        })
      )
      renderUI()
      const card = await screen.findByRole('region', { name: 'Database' })
      expect(await screen.findByRole('button', { name: /undo/i })).toBeEnabled()
      expect(card).not.toHaveTextContent(/scheduled/i)
      expect(card).toHaveTextContent(/CHARON_DB_COMPACT_ON_START/)
    })

    it('shows the server message when the request is rejected (409)', async () => {
      const user = userEvent.setup()
      vi.mocked(api.getDatabaseStatus).mockResolvedValue(makeStatus())
      vi.mocked(api.requestOptimizeOnRestart).mockRejectedValue(
        new Error('the database is already optimized or has too little reclaimable space')
      )
      renderUI()
      await user.click(await screen.findByRole('button', { name: /reclaim space on next restart/i }))
      await waitFor(() =>
        expect(toast.error).toHaveBeenCalledWith(expect.stringContaining('already optimized'))
      )
      // The card is refreshed from the server afterwards.
      await waitFor(() => expect(api.getDatabaseStatus).toHaveBeenCalledTimes(2))
    })

    it('reports a failed undo', async () => {
      const user = userEvent.setup()
      vi.mocked(api.getDatabaseStatus).mockResolvedValue(makeStatus({ compact_requested: true }))
      vi.mocked(api.cancelOptimizeOnRestart).mockRejectedValue('network down')
      renderUI()
      await user.click(await screen.findByRole('button', { name: /undo/i }))
      await waitFor(() => expect(toast.error).toHaveBeenCalledWith(expect.stringContaining('network down')))
    })
  })
})
