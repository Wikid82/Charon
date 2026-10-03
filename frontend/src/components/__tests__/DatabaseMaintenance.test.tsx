import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import type { ReactNode } from 'react'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import * as api from '../../api/databaseMaintenance'
import { toast } from '../../utils/toast'
import {
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
    expect(status).toHaveTextContent(/several failed or interrupted attempts/i)
    expect(status).toHaveTextContent(/let the optimization finish; avoid restarting while it runs/i)
    expect(status).toHaveTextContent(/optional button below to schedule it, then restart Charon/i)
    expect(status).not.toHaveTextContent(/restart charon to let it try again/i)
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
    expect(screen.getByText(/not automatic yet: freed space stays inside the file until charon next optimizes the database/i)).toBeInTheDocument()
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
    ['optimization is off but plenty is reclaimable', { can_request_optimize: false, env_mode: 'off' }],
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
  beforeEach(() => vi.resetAllMocks())

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

it('shows Undo but no false "scheduled" promise when requested while the env is off', async () => {
    const user = userEvent.setup()
    vi.mocked(api.cancelOptimizeOnRestart).mockResolvedValue({ requested: false })
    renderBlock(
      <ReclaimControl db={makeStatus({ env_mode: 'off', compact_requested: true, can_request_optimize: false })} />
    )
    expect(screen.queryByText(/scheduled/i)).toBeNull()
    await user.click(screen.getByRole('button', { name: /undo/i }))
    expect(api.cancelOptimizeOnRestart).toHaveBeenCalledTimes(1)
    expect(screen.getByText('Reclaim space now (optional)')).toBeInTheDocument()
  })

  it('shows the scheduled message with Undo and no reclaim button', () => {
    renderBlock(<ReclaimControl db={makeStatus({ compact_requested: true })} />)
    expect(screen.getByText(/scheduled - the space is reclaimed the next time charon starts/i)).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /reclaim space on next restart/i })).toBeNull()
  })

  it('reports a rejected request (409) in a toast and refreshes the status', async () => {
    const user = userEvent.setup()
    vi.mocked(api.requestOptimizeOnRestart).mockRejectedValue(new Error('already optimized'))
    renderBlock(<ReclaimControl db={makeStatus()} />)
    await user.click(button())
    await waitFor(() => expect(toast.error).toHaveBeenCalledWith(expect.stringContaining('already optimized')))
  })

  it('reports a failed undo, including non-Error rejections', async () => {
    const user = userEvent.setup()
    vi.mocked(api.cancelOptimizeOnRestart).mockRejectedValue('network down')
    renderBlock(<ReclaimControl db={makeStatus({ compact_requested: true })} />)
    await user.click(screen.getByRole('button', { name: /undo/i }))
    await waitFor(() => expect(toast.error).toHaveBeenCalledWith(expect.stringContaining('network down')))
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
