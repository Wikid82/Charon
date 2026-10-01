import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import type { ReactNode } from 'react'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import * as api from '../../api/databaseMaintenance'
import { toast } from '../../utils/toast'
import {
  DatabaseMaintenanceBanner,
  DatabaseMaintenanceCard,
  DatabaseNotice,
  DatabaseStatusList,
  LastOptimization,
  ReclaimControl,
} from '../DatabaseMaintenance'

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

describe('Legacy database card and banner (removed with the System Settings card)', () => {
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
      ['restart_to_optimize', /reclaim about 1\.7 GB automatically the next time it starts/i],
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

const renderBlock = (ui: ReactNode) => {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(<QueryClientProvider client={queryClient}>{ui}</QueryClientProvider>)
}

const nothingToDo = () => screen.queryAllByText(/nothing to do/i)

describe('DatabaseNotice', () => {
  const info = (code: api.DatabaseNotice['code']): api.DatabaseNotice => ({
    code,
    severity: 'info',
    reclaimable_bytes: 1.7 * GB,
  })

  it('renders nothing without a notice', () => {
    const { container } = renderBlock(<DatabaseNotice notice={null} />)
    expect(container).toBeEmptyDOMElement()
  })

  it.each([
    ['restart_to_optimize', /reclaim about 1\.7 GB automatically.*nothing is needed from you/i],
    ['database_busy', /postponed because the database was in use.*retried on the next start/i],
    ['disabled_by_env', /CHARON_DB_COMPACT_ON_START=off/],
  ] as const)('%s is plain status text and never an alert', (code, text) => {
    renderBlock(<DatabaseNotice notice={info(code)} />)
    expect(screen.getByRole('status')).toHaveTextContent(text)
    expect(screen.queryByRole('alert')).toBeNull()
    expect(nothingToDo()).toHaveLength(0)
  })

  it('insufficient_disk is a calm status notice with a short title and the figures', () => {
    renderBlock(
      <DatabaseNotice
        notice={{
          code: 'insufficient_disk',
          severity: 'warning',
          reclaimable_bytes: 1.7 * GB,
          required_bytes: 9.6 * GB,
          available_bytes: 2 * GB,
        }}
      />
    )
    const status = screen.getByRole('status')
    expect(status).toHaveTextContent('Automatic cleanup could not run')
    expect(status).toHaveTextContent(/because the disk is nearly full.*free some disk space/i)
    expect(status).toHaveTextContent('9.6 GB')
    expect(status).toHaveTextContent('2.0 GB')
    expect(status).not.toHaveTextContent(/NaN|-\d/)
    expect(screen.queryByRole('alert')).toBeNull()
  })

  it('insufficient_disk without figures never shows NaN or a negative number', () => {
    renderBlock(
      <DatabaseNotice notice={{ code: 'insufficient_disk', severity: 'warning', reclaimable_bytes: 1 }} />
    )
    expect(screen.getByRole('status')).not.toHaveTextContent(/NaN|-\d/)
  })

  it('too_many_failures is a calm status notice and never an alert', () => {
    renderBlock(
      <DatabaseNotice notice={{ code: 'too_many_failures', severity: 'warning', reclaimable_bytes: 1 }} />
    )
    const status = screen.getByRole('status')
    expect(status).toHaveTextContent('Automatic cleanup has stopped')
    expect(status).toHaveTextContent(/stopped trying its automatic database cleanup.*proxies are not affected/i)
    expect(screen.queryByRole('alert')).toBeNull()
  })
})

describe('DatabaseStatusList', () => {
  it('shows size, write-ahead log, free disk, reclaimable space and the mode in plain words', () => {
    render(<DatabaseStatusList db={makeStatus({ auto_vacuum: 'none' })} />)
    expect(screen.getByText('Database size').nextSibling).toHaveTextContent('4.8 GB')
    expect(screen.getByText('Write-ahead log').nextSibling).toHaveTextContent('4.2 MB')
    expect(screen.getByText('Free disk space').nextSibling).toHaveTextContent('52.0 GB')
    expect(screen.getByText('Space that could be reclaimed').nextSibling).toHaveTextContent('1.7 GB')
    expect(screen.getByText(/manual: freed space stays inside the file/i)).toBeInTheDocument()
  })

  it.each([
    ['incremental', /automatic: freed space goes back to your disk on its own/i],
    ['full', /automatic: freed space goes back to your disk immediately/i],
  ] as const)('describes %s mode', (mode, text) => {
    render(<DatabaseStatusList db={makeStatus({ auto_vacuum: mode })} />)
    expect(screen.getByText(text)).toBeInTheDocument()
  })

  it('omits the mode row for an unknown value and never prints it', () => {
    const { container } = render(
      <DatabaseStatusList db={makeStatus({ auto_vacuum: 'weird' as api.DatabaseStatus['auto_vacuum'] })} />
    )
    expect(screen.queryByText('Optimization mode')).toBeNull()
    expect(container).not.toHaveTextContent('weird')
  })
})

describe('LastOptimization', () => {
  const last = (over: Partial<api.DatabaseLastResult>): api.DatabaseLastResult => ({
    at: '2026-09-30T04:00:00Z',
    outcome: 'converted',
    reason: '',
    bytes_before: 2 * GB,
    bytes_after: 1 * GB,
    ...over,
  })
  const text = () => screen.getByText(/./, { selector: 'p' })

  it('says no optimization has been needed when quiet and null', () => {
    render(<LastOptimization db={quiet} />)
    expect(screen.getByText('No optimization has been needed so far.')).toBeInTheDocument()
    expect(screen.queryByText(/not optimized yet/i)).toBeNull()
  })

  it.each([
    ['can_request_optimize', { can_request_optimize: true }],
    ['compact_requested', { can_request_optimize: false, compact_requested: true }],
  ] as const)('says "Not optimized yet" (no "nothing to do") when %s', (_name, over) => {
    render(<LastOptimization db={makeStatus(over)} />)
    expect(screen.getByText('Not optimized yet.')).toBeInTheDocument()
    expect(screen.queryByText(/no optimization has been needed/i)).toBeNull()
    expect(nothingToDo()).toHaveLength(0)
  })

  it('renders a converted result with a real date and the saved size', () => {
    render(<LastOptimization db={makeStatus({ last_result: last({}) })} />)
    expect(text()).toHaveTextContent(/optimized on .*2026.*: 2\.0 GB → 1\.0 GB \(saved 1\.0 GB\)/i)
    expect(text()).not.toHaveTextContent(/invalid date|converted/i)
  })

  it('adds the background note for a pending checkpoint', () => {
    render(<LastOptimization db={makeStatus({ last_result: last({ outcome: 'converted_pending_checkpoint' }) })} />)
    expect(text()).toHaveTextContent(/finishes in the background/i)
    expect(text()).not.toHaveTextContent(/pending_checkpoint/)
  })

  it('clamps the saved size at zero', () => {
    render(<LastOptimization db={makeStatus({ last_result: last({ bytes_before: GB, bytes_after: 2 * GB }) })} />)
    expect(text()).toHaveTextContent('saved 0 B')
  })

  it('omits the date instead of printing "Invalid Date"', () => {
    render(<LastOptimization db={makeStatus({ last_result: last({ at: 'not-a-date' }) })} />)
    expect(text()).toHaveTextContent(/^Optimized: /)
    expect(text()).not.toHaveTextContent(/invalid date/i)
  })

  it.each([
    ['integrity_check_failed', /did not pass its safety check/],
    ['database_busy', /database was in use/],
    ['caddy_not_ready', /proxy engine was not ready/],
    ['startup_timeout', /starting up took too long/],
    ['shutting_down', /shutting down/],
    ['conversion_failed', /conversion did not complete/],
    ['internal_error', /unexpected internal problem/],
    ['insufficient_disk', /not enough free disk space/],
    ['too_many_failures', /failed several times/],
    ['disabled_by_env', /turned off by the server configuration/],
    ['already_optimized', /already set up to return unused space on its own/],
    ['nothing_to_reclaim', /no unused space worth reclaiming/],
    ['below_threshold', /too little space/],
    ['brand_new_code', /no reason was recorded/],
  ])('renders a skipped result for %s in plain words', (reason, expected) => {
    render(<LastOptimization db={makeStatus({ last_result: last({ outcome: 'skipped', reason }) })} />)
    expect(text()).toHaveTextContent(/^Skipped on /)
    expect(text()).toHaveTextContent(expected)
    expect(text().textContent).not.toContain(reason === 'brand_new_code' ? 'brand_new_code' : '_')
    expect(nothingToDo()).toHaveLength(0)
  })

  it.each([
    ['failed', /did not finish/],
    ['interrupted', /was interrupted/],
    ['cancelled', /was cancelled/],
    ['something_else', /last looked at/],
  ])('renders the %s outcome as one plain line', (outcome, expected) => {
    render(<LastOptimization db={makeStatus({ last_result: last({ outcome }) })} />)
    expect(text()).toHaveTextContent(expected)
    expect(text().textContent).not.toContain(outcome === 'something_else' ? 'something_else' : '_')
  })
})

describe('ReclaimControl', () => {
  const button = () => screen.getByRole('button', { name: /reclaim space on next restart/i })

  it('always carries the optional title and the automatic-already help line', () => {
    renderBlock(<ReclaimControl db={makeStatus()} />)
    expect(screen.getByText('Reclaim space now (optional)')).toBeInTheDocument()
    expect(
      screen.getByText('Charon already does this automatically. This only schedules it for the next start.')
    ).toBeInTheDocument()
  })

  it('is enabled with the size helper when the server allows it', () => {
    renderBlock(<ReclaimControl db={makeStatus()} />)
    expect(button()).toBeEnabled()
    expect(screen.getByText(/1\.7 GB could be reclaimed\. Charon restarts only when you restart it/)).toBeInTheDocument()
  })

  it.each([
    [
      'incremental',
      { auto_vacuum: 'incremental', can_request_optimize: false, reclaimable_bytes: GB },
      /nothing to do: the database already returns unused space automatically/i,
    ],
    [
      'below the floor',
      { can_request_optimize: false, reclaimable_bytes: 50_000_000 },
      /not worth reclaiming.*under about 100 MB/i,
    ],
    [
      'env off',
      { env_mode: 'off', can_request_optimize: false },
      /disabled by CHARON_DB_COMPACT_ON_START=off/i,
    ],
    ['other', { can_request_optimize: false }, /not available right now/i],
  ] as const)('is disabled with its reason when %s', (_name, over, reason) => {
    renderBlock(<ReclaimControl db={makeStatus(over)} />)
    expect(button()).toBeDisabled()
    const reasonLine = screen.getByText(reason)
    expect(button()).toHaveAttribute('aria-describedby', reasonLine.id)
  })

  it('lets the incremental reason win over a disabled env override', () => {
    renderBlock(
      <ReclaimControl db={makeStatus({ auto_vacuum: 'incremental', env_mode: 'off', can_request_optimize: false })} />
    )
    expect(screen.getByText(/already returns unused space automatically/i)).toBeInTheDocument()
  })

  it('shows exactly one "nothing to do" in the quiet incremental state, even with a skipped last result', () => {
    for (const reason of ['already_optimized', 'nothing_to_reclaim']) {
      const db = makeStatus({
        ...quiet,
        last_result: { at: '2026-09-30T04:00:00Z', outcome: 'skipped', reason, bytes_before: 0, bytes_after: 0 },
      })
      const { unmount } = renderBlock(
        <>
          <LastOptimization db={db} />
          <DatabaseNotice notice={db.notice} />
          <ReclaimControl db={db} />
        </>
      )
      expect(screen.getByText(/skipped on/i)).toBeInTheDocument()
      expect(nothingToDo()).toHaveLength(1)
      unmount()
    }
  })
})
