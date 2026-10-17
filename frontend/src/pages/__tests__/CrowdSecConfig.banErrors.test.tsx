import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { AxiosError, type AxiosResponse } from 'axios'
import { describe, it, expect, vi, beforeEach } from 'vitest'

import * as backupsApi from '../../api/backups'
import * as crowdsecApi from '../../api/crowdsec'
import * as featureFlagsApi from '../../api/featureFlags'
import * as presetsApi from '../../api/presets'
import * as securityApi from '../../api/security'
import { renderWithQueryClient } from '../../test-utils/renderWithQueryClient'
import { toast } from '../../utils/toast'
import CrowdSecConfig from '../CrowdSecConfig'

vi.mock('../../api/security')
vi.mock('../../api/crowdsec')
vi.mock('../../api/presets')
vi.mock('../../api/backups')
vi.mock('../../api/featureFlags')
vi.mock('../../components/CrowdSecBouncerKeyDisplay', () => ({ CrowdSecBouncerKeyDisplay: () => null }))
vi.mock('../../utils/toast', () => ({ toast: { success: vi.fn(), error: vi.fn(), info: vi.fn() } }))

const TEXT = {
  invalid_ip: 'Enter a valid IPv4 or IPv6 address, or a range such as 192.0.2.0/24.',
  invalid_duration: 'Choose a duration between 1 minute and 365 days, for example 4h, 30m or 1d.',
  invalid_reason: 'The reason can be at most 200 characters.',
  decision_not_found: 'No active decision matches this address or range.',
  cscli_timeout: 'CrowdSec did not answer in time. Try again.',
}

/** Builds the error the shared client produces: the server's English text becomes the message. */
const refusal = (status: number, error: string, error_code?: string): AxiosError =>
  new AxiosError(error, 'ERR_BAD_REQUEST', undefined, undefined, {
    status,
    data: { error, error_code },
  } as AxiosResponse)

const openBanDialog = async () => {
  renderWithQueryClient(<CrowdSecConfig />)
  await screen.findByText('CrowdSec Configuration')
  await userEvent.click(await screen.findByRole('button', { name: /Ban IP/ }))
  const dialog = await screen.findByRole('dialog', { name: 'Ban IP Address' })
  await userEvent.type(within(dialog).getByLabelText('IP Address'), '203.0.113.9')
  await userEvent.click(within(dialog).getByRole('button', { name: 'Ban IP' }))
  return dialog
}

const unbanFailing = async (err: unknown) => {
  vi.mocked(crowdsecApi.unbanIP).mockRejectedValue(err)
  renderWithQueryClient(<CrowdSecConfig />)
  await userEvent.click((await screen.findAllByText('Unban'))[0])
  const dialog = await screen.findByRole('dialog', { name: 'Confirm Unban' })
  await userEvent.click(within(dialog).getByRole('button', { name: 'Unban' }))
  await waitFor(() => expect(toast.error).toHaveBeenCalledTimes(1))
  return vi.mocked(toast.error).mock.calls[0][0]
}

describe('CrowdSecConfig ban and unban refusals', () => {
  beforeEach(async () => {
    vi.clearAllMocks()
    const actual = await vi.importActual<typeof crowdsecApi>('../../api/crowdsec')
    vi.mocked(crowdsecApi.getCrowdsecBanError).mockImplementation(actual.getCrowdsecBanError)
    vi.mocked(securityApi.getSecurityStatus).mockResolvedValue({
      cerberus: { enabled: true },
      crowdsec: { enabled: true, mode: 'local', api_url: '' },
      waf: { enabled: true, mode: 'enabled' },
      rate_limit: { enabled: true },
      acl: { enabled: true },
    })
    vi.mocked(crowdsecApi.statusCrowdsec).mockResolvedValue({ running: true, pid: 1, lapi_ready: true })
    vi.mocked(crowdsecApi.listCrowdsecFiles).mockResolvedValue({ files: [] })
    vi.mocked(crowdsecApi.listCrowdsecDecisions).mockResolvedValue({
      decisions: [
        { id: '1', ip: '1.1.1.1', reason: 'bot', duration: '24h', created_at: '2024-01-01T00:00:00Z', source: 'manual' },
      ],
    })
    vi.mocked(presetsApi.listCrowdsecPresets).mockResolvedValue({ presets: [] })
    vi.mocked(backupsApi.createBackup).mockResolvedValue({ job_id: 'j', type: 'create', status: 'pending' })
    vi.mocked(featureFlagsApi.getFeatureFlags).mockResolvedValue({ 'feature.crowdsec.console_enrollment': false })
  })

  it.each([
    ['invalid_ip', 'ban-ip-error'],
    ['invalid_duration', 'ban-duration-error'],
    ['invalid_reason', 'ban-reason-error'],
  ])('shows %s beside its field and keeps the dialog open', async (code, errorId) => {
    vi.mocked(crowdsecApi.banIP).mockRejectedValue(refusal(400, 'english server text', code))
    const dialog = await openBanDialog()

    const message = await within(dialog).findByText(TEXT[code as keyof typeof TEXT])
    expect(message).toHaveAttribute('id', errorId)
    expect(toast.error).not.toHaveBeenCalled()
    expect(dialog).toBeInTheDocument()
  })

  it('clears the field message when the field is edited', async () => {
    vi.mocked(crowdsecApi.banIP).mockRejectedValue(refusal(400, 'english', 'invalid_ip'))
    const dialog = await openBanDialog()
    await within(dialog).findByText(TEXT.invalid_ip)

    await userEvent.type(within(dialog).getByLabelText('IP Address'), '1')
    expect(within(dialog).queryByText(TEXT.invalid_ip)).not.toBeInTheDocument()
  })

  it('clears the duration and reason messages when those fields are edited', async () => {
    vi.mocked(crowdsecApi.banIP).mockRejectedValue(refusal(400, 'english', 'invalid_duration'))
    const dialog = await openBanDialog()
    await within(dialog).findByText(TEXT.invalid_duration)
    await userEvent.selectOptions(within(dialog).getByLabelText('Duration'), '4h')
    expect(within(dialog).queryByText(TEXT.invalid_duration)).not.toBeInTheDocument()

    vi.mocked(crowdsecApi.banIP).mockRejectedValue(refusal(400, 'english', 'invalid_reason'))
    await userEvent.click(within(dialog).getByRole('button', { name: 'Ban IP' }))
    await within(dialog).findByText(TEXT.invalid_reason)
    await userEvent.type(within(dialog).getByLabelText('Reason'), 'x')
    expect(within(dialog).queryByText(TEXT.invalid_reason)).not.toBeInTheDocument()
  })

  it('shows a retry message as a toast when CrowdSec times out during a ban', async () => {
    vi.mocked(crowdsecApi.banIP).mockRejectedValue(refusal(504, 'english', 'cscli_timeout'))
    await openBanDialog()

    await waitFor(() => expect(toast.error).toHaveBeenCalledWith(TEXT.cscli_timeout))
  })

  it.each([
    ['decision_not_found', 404],
    ['cscli_timeout', 504],
  ])('translates %s for an unban', async (code, status) => {
    expect(await unbanFailing(refusal(status, 'english server text', code))).toBe(TEXT[code as keyof typeof TEXT])
  })

  it('keeps the server text for an unknown code or no code', async () => {
    vi.mocked(crowdsecApi.banIP).mockRejectedValue(refusal(400, 'newer text', 'future_code'))
    await openBanDialog()
    await waitFor(() => expect(toast.error).toHaveBeenCalledWith('newer text'))

    vi.mocked(toast.error).mockClear()
    vi.mocked(crowdsecApi.banIP).mockRejectedValue(refusal(500, 'failed to ban IP'))
    await userEvent.click(within(screen.getByRole('dialog', { name: 'Ban IP Address' })).getByRole('button', { name: 'Ban IP' }))
    await waitFor(() => expect(toast.error).toHaveBeenCalledWith('failed to ban IP'))
  })

  it('keeps the server text for an uncoded unban failure and falls back to the generic text', async () => {
    expect(await unbanFailing(refusal(500, 'failed to unban IP'))).toBe('failed to unban IP')
  })

  it('uses the generic message when a non-Error is thrown', async () => {
    expect(await unbanFailing('boom')).toBe('Failed to unban IP')
  })

  it('offers a one-year maximum instead of a permanent option and sends 365d', async () => {
    vi.mocked(crowdsecApi.banIP).mockResolvedValue(undefined)
    renderWithQueryClient(<CrowdSecConfig />)
    await screen.findByText('CrowdSec Configuration')
    await userEvent.click(await screen.findByRole('button', { name: /Ban IP/ }))
    const dialog = await screen.findByRole('dialog', { name: 'Ban IP Address' })
    const select = within(dialog).getByLabelText('Duration')

    expect(within(select).getAllByRole('option').map((o) => o.textContent)).toEqual([
      '1 hour',
      '4 hours',
      '24 hours',
      '7 days',
      '30 days',
      'Up to one year (365 days)',
    ])
    expect(within(dialog).queryByText(/permanent/i)).not.toBeInTheDocument()
    expect(within(dialog).getByText('CrowdSec bans are limited to one year.')).toBeInTheDocument()

    await userEvent.selectOptions(select, '365d')
    await userEvent.type(within(dialog).getByLabelText('IP Address'), '203.0.113.9')
    await userEvent.click(within(dialog).getByRole('button', { name: 'Ban IP' }))
    await waitFor(() => expect(crowdsecApi.banIP).toHaveBeenCalledWith('203.0.113.9', '365d', ''))
  })
})
