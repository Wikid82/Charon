import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { AxiosError } from 'axios'
import { describe, it, expect, vi, beforeEach } from 'vitest'

import * as api from '../../../api/crowdsec'
import HubItemsRestorePanel from '../HubItemsRestorePanel'

vi.mock('../../../api/crowdsec')

const MISSING: api.HubItem[] = [
  { type: 'collections', name: 'crowdsecurity/caddy' },
  { type: 'parsers', name: 'crowdsecurity/sshd-logs' },
  { type: 'scenarios', name: 'crowdsecurity/ssh-bf' },
]

const result = (over: Partial<api.RestoreHubItemsResult> = {}): api.RestoreHubItemsResult => ({
  installed: [],
  already_installed: [],
  failed: [],
  remaining: 0,
  skipped_invalid: 0,
  reloaded: false,
  ...over,
})

const httpError = (status: number) =>
  new AxiosError('failed', undefined, undefined, undefined, {
    status,
    statusText: String(status),
    headers: {},
    config: {},
    data: { error: 'x' },
  } as never)

const BUSY_TEXT = 'CrowdSec is busy installing a preset. Try again when it finishes.'

// A 409 from the busy guard while a background install runs (not a restore conflict).
const busyError = () =>
  new AxiosError('failed', undefined, undefined, undefined, {
    status: 409,
    statusText: '409',
    headers: {},
    config: {},
    data: { error: 'busy', error_code: 'crowdsec_busy', job_id: 'job-1' },
  } as never)

const renderPanel = (skipped = 0, missing = MISSING) =>
  render(
    <QueryClientProvider client={new QueryClient({ defaultOptions: { mutations: { retry: false } } })}>
      <HubItemsRestorePanel hubItems={{ in_export: 5, skipped_invalid: skipped, missing }} />
    </QueryClientProvider>,
  )

const installButton = () => screen.getByRole('button', { name: 'Install missing hub items' })

describe('HubItemsRestorePanel', () => {
  beforeEach(() => vi.clearAllMocks())

  it('lists the missing items with an enabled install button', () => {
    renderPanel()
    for (const item of MISSING) {
      expect(screen.getByText(item.name, { exact: true })).toBeInTheDocument()
    }
    expect(installButton()).toBeEnabled()
    expect(screen.queryByText(/could not be used/i)).not.toBeInTheDocument()
  })

  it('shows a neutral note when entries were skipped', () => {
    renderPanel(2)
    expect(screen.getByText('2 entries in the export could not be used.')).toBeInTheDocument()
  })

  it('disables the button and announces progress while installing', async () => {
    let release: (value: api.RestoreHubItemsResult) => void = () => undefined
    vi.mocked(api.restoreHubItems).mockReturnValue(new Promise((resolve) => { release = resolve }))
    renderPanel()

    await userEvent.click(installButton())

    expect(installButton()).toBeDisabled()
    expect(screen.getByRole('status')).toHaveTextContent('Installing hub items')
    expect(screen.getByTestId('hub-items-panel')).toHaveAttribute('aria-busy', 'true')
    expect(api.restoreHubItems).toHaveBeenCalledWith(MISSING)

    release(result({ installed: MISSING }))
    await waitFor(() => expect(screen.queryByRole('button', { name: 'Install missing hub items' })).not.toBeInTheDocument())
  })

  it('reports a full success without further buttons', async () => {
    vi.mocked(api.restoreHubItems).mockResolvedValue(result({ installed: MISSING.slice(0, 2), already_installed: [MISSING[2]] }))
    renderPanel()

    await userEvent.click(installButton())

    expect(await screen.findByText('2 installed, 1 already installed, 0 failed.')).toBeInTheDocument()
    expect(screen.getByText(MISSING[0].name, { exact: true })).toBeInTheDocument()
    expect(screen.getByText(MISSING[2].name, { exact: true })).toBeInTheDocument()
    expect(screen.queryByRole('button')).not.toBeInTheDocument()
  })

  it('shows translated failure reasons and resends only not_attempted entries', async () => {
    vi.mocked(api.restoreHubItems)
      .mockResolvedValueOnce(
        result({
          installed: [MISSING[0]],
          failed: [
            { ...MISSING[1], reason: 'install_failed' },
            { ...MISSING[2], reason: 'not_attempted' },
          ],
          remaining: 1,
        }),
      )
      .mockResolvedValueOnce(result({ installed: [MISSING[2]] }))
    renderPanel()

    await userEvent.click(installButton())

    expect(await screen.findByText('Installation failed')).toBeInTheDocument()
    expect(screen.getByText('Not attempted yet')).toBeInTheDocument()
    expect(screen.getByText('1 items were not attempted before the time limit.')).toBeInTheDocument()
    expect(screen.getByText(MISSING[1].name, { exact: true })).toBeVisible()
    expect(screen.queryByRole('button', { name: 'Install missing hub items' })).not.toBeInTheDocument()

    await userEvent.click(screen.getByRole('button', { name: 'Install remaining' }))

    await waitFor(() => expect(api.restoreHubItems).toHaveBeenCalledTimes(2))
    expect(vi.mocked(api.restoreHubItems).mock.calls[1][0]).toEqual([MISSING[2]])
    await waitFor(() => expect(screen.queryByRole('button', { name: 'Install remaining' })).not.toBeInTheDocument())
  })

  it('keeps the earlier result when a retry fails', async () => {
    vi.mocked(api.restoreHubItems)
      .mockResolvedValueOnce(result({ failed: [{ ...MISSING[2], reason: 'not_attempted' }], remaining: 1 }))
      .mockRejectedValueOnce(httpError(502))
    renderPanel()

    await userEvent.click(installButton())
    await userEvent.click(await screen.findByRole('button', { name: 'Install remaining' }))

    expect(await screen.findByText(/hub could not be reached/i)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Install remaining' })).toBeEnabled()
    expect(screen.getByText('Not attempted yet')).toBeInTheDocument()
  })

  it('shows passive reload-pending text after a restore whose reload was queued', async () => {
    vi.mocked(api.restoreHubItems).mockResolvedValue(result({ installed: [MISSING[0]], reloaded: false, reload_pending: true }))
    renderPanel()

    await userEvent.click(installButton())

    expect(await screen.findByTestId('hub-items-reload-pending')).toHaveTextContent(
      'The reload is scheduled and will happen once CrowdSec has finished starting.',
    )
  })

  it('omits the reload-pending text when the restore did not queue a reload', async () => {
    vi.mocked(api.restoreHubItems).mockResolvedValue(result({ installed: [MISSING[0]], reloaded: true }))
    renderPanel()

    await userEvent.click(installButton())

    expect(await screen.findByText('Installed')).toBeInTheDocument()
    expect(screen.queryByTestId('hub-items-reload-pending')).not.toBeInTheDocument()
  })

  it('does not offer a retry when remaining entries are reported without not_attempted rows', async () => {
    vi.mocked(api.restoreHubItems).mockResolvedValue(
      result({ failed: [{ ...MISSING[1], reason: 'verify_failed' }, { ...MISSING[2], reason: 'not_found' }], remaining: 0 }),
    )
    renderPanel()

    await userEvent.click(installButton())

    expect(await screen.findByText('Installed but failed verification')).toBeInTheDocument()
    expect(screen.getByText('Not found in the hub')).toBeInTheDocument()
    expect(screen.queryByRole('button')).not.toBeInTheDocument()
  })

  it.each([
    [502, /hub could not be reached.*nothing was installed/i],
    [409, /restore is already running/i],
    [500, /could not be installed/i],
  ])('shows a clear message for HTTP %i and keeps the button enabled', async (status, message) => {
    vi.mocked(api.restoreHubItems).mockRejectedValue(httpError(status))
    renderPanel()

    await userEvent.click(installButton())

    expect(await screen.findByText(message)).toBeInTheDocument()
    expect(installButton()).toBeEnabled()
    expect(screen.getByText(MISSING[0].name, { exact: true })).toBeInTheDocument()
  })

  it('shows the busy message, not the restore conflict, when an install refuses the restore', async () => {
    vi.mocked(api.restoreHubItems).mockRejectedValue(busyError())
    renderPanel()

    await userEvent.click(installButton())

    expect(await screen.findByText(BUSY_TEXT)).toBeInTheDocument()
    expect(screen.queryByText(/restore is already running/i)).not.toBeInTheDocument()
    expect(installButton()).toBeEnabled()
  })

  it('shows the other-change message when a refusal names no install job', async () => {
    vi.mocked(api.restoreHubItems).mockRejectedValue(
      new AxiosError('failed', undefined, undefined, undefined, {
        status: 409,
        statusText: '409',
        headers: {},
        config: {},
        data: { error: 'busy', error_code: 'crowdsec_busy' },
      } as never),
    )
    renderPanel()

    await userEvent.click(installButton())

    expect(await screen.findByText('CrowdSec is finishing another change. Try again in a moment.')).toBeInTheDocument()
    expect(screen.queryByText(BUSY_TEXT)).not.toBeInTheDocument()
  })

  it('shows the generic message for a non-HTTP error', async () => {
    vi.mocked(api.restoreHubItems).mockRejectedValue(new Error('network down'))
    renderPanel()

    await userEvent.click(installButton())

    expect(await screen.findByText(/could not be installed/i)).toBeInTheDocument()
  })
})
