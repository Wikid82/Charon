import { screen, waitFor } from '@testing-library/react'
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

/** Builds the error the shared client produces: the server's English text becomes the message. */
const refusal = (status: number, body: { error: string; error_code?: string; names?: string[] }): AxiosError => {
  return new AxiosError(body.error, 'ERR_BAD_REQUEST', undefined, undefined, {
    status,
    data: body,
  } as AxiosResponse)
}

const importFailing = async (err: unknown) => {
  vi.mocked(crowdsecApi.importCrowdsecConfig).mockRejectedValue(err)
  renderWithQueryClient(<CrowdSecConfig />)
  await screen.findByText('CrowdSec Configuration')
  await userEvent.upload(screen.getByTestId('import-file'), new File(['data'], 'cfg.tar.gz'))
  await userEvent.click(screen.getByTestId('import-btn'))
  await waitFor(() => expect(toast.error).toHaveBeenCalledTimes(1))
  return vi.mocked(toast.error).mock.calls[0][0]
}

describe('CrowdSecConfig import refusals', () => {
  beforeEach(async () => {
    vi.clearAllMocks()
    const actual = await vi.importActual<typeof crowdsecApi>('../../api/crowdsec')
    vi.mocked(crowdsecApi.getCrowdsecImportError).mockImplementation(actual.getCrowdsecImportError)
    vi.mocked(securityApi.getSecurityStatus).mockResolvedValue({
      cerberus: { enabled: true },
      crowdsec: { enabled: true, mode: 'local', api_url: '' },
      waf: { enabled: true, mode: 'enabled' },
      rate_limit: { enabled: true },
      acl: { enabled: true },
    })
    vi.mocked(crowdsecApi.statusCrowdsec).mockResolvedValue({ running: true, pid: 1, lapi_ready: true })
    vi.mocked(crowdsecApi.listCrowdsecFiles).mockResolvedValue({ files: [] })
    vi.mocked(crowdsecApi.listCrowdsecDecisions).mockResolvedValue({ decisions: [] })
    vi.mocked(presetsApi.listCrowdsecPresets).mockResolvedValue({ presets: [] })
    vi.mocked(backupsApi.createBackup).mockResolvedValue({ job_id: 'j', type: 'create', status: 'pending' })
    vi.mocked(featureFlagsApi.getFeatureFlags).mockResolvedValue({ 'feature.crowdsec.console_enrollment': false })
  })

  it.each([
    [422, 'import_flat_archive', 'The archive keeps its configuration at the top level; CrowdSec reads it from the config/ folder. Export a configuration from this page, or put the files in a config/ folder inside the archive. Nothing was changed.'],
    [422, 'import_kept_profile_duplicate', 'The archive has no profiles.yaml, so the current one was kept, but a notification file it needs defines a name the archive already defines.'],
    [500, 'import_keep_current_failed', 'The current CrowdSec files could not be kept, so the import was stopped. See the server logs for details.'],
    [422, 'import_config_not_found', 'config.yaml was not found at the top level or in the config/ folder of the archive.'],
  ])('translates %i %s', async (status, code, text) => {
    expect(await importFailing(refusal(status, { error: 'english server text', error_code: code }))).toBe(text)
  })

  it('shows the busy text while a preset install runs', async () => {
    const busyWithJob = new AxiosError('busy', 'ERR_BAD_REQUEST', undefined, undefined, {
      status: 409,
      data: { error: 'busy', error_code: 'crowdsec_busy', job_id: 'job-1' },
    } as AxiosResponse)
    expect(await importFailing(busyWithJob)).toBe('CrowdSec is busy installing a preset. Try again when it finishes.')
  })

  it('shows the other-change text when a refused import names no install job', async () => {
    expect(await importFailing(refusal(409, { error: 'busy', error_code: 'crowdsec_busy' }))).toBe(
      'CrowdSec is finishing another change. Try again in a moment.',
    )
  })

  it('lists the undefined notification names of a profiles.yaml in the archive', async () => {
    const message = await importFailing(
      refusal(422, { error: 'server text', error_code: 'import_profile_names_undefined', names: ['slack', 'http'] }),
    )
    expect(message).toBe("The archive's profiles.yaml names notifications that no file in the archive defines: slack, http")
  })

  it('lists the undefined names of a kept profile', async () => {
    const message = await importFailing(
      refusal(422, { error: 'server text', error_code: 'import_kept_profile_undefined', names: ['slack'] }),
    )
    expect(message).toBe(
      'The archive has no profiles.yaml, so the current one was kept, but it names notifications the archive has no files for: slack',
    )
  })

  it('shows the first 10 names then the number of the rest, as sanitised plain text', async () => {
    const names = Array.from({ length: 12 }, (_, i) => `n${i + 1}`)
    const override = String.fromCharCode(0x202e)
    names[0] = `a${override}b\u0007c`
    const message = await importFailing(
      refusal(422, { error: 'server text', error_code: 'import_kept_profile_undefined', names }),
    )
    expect(message).toContain(': abc, n2, n3, n4, n5, n6, n7, n8, n9, n10, and 2 more')
    expect(message).not.toContain('n11')
    expect(message).not.toContain(override)
  })

  it('keeps the server text when a names refusal carries no names', async () => {
    expect(
      await importFailing(refusal(422, { error: 'server text', error_code: 'import_kept_profile_undefined' })),
    ).toBe('server text')
  })

  it('keeps the server text for a refusal without a code', async () => {
    expect(await importFailing(refusal(422, { error: 'archive validation failed' }))).toBe('archive validation failed')
  })

  it('keeps the server text for a code it does not know', async () => {
    expect(await importFailing(refusal(422, { error: 'newer text', error_code: 'import_future_code' }))).toBe('newer text')
  })

  it('uses the generic message when the error is not an Error', async () => {
    expect(await importFailing('boom')).toBe('Failed to import')
  })
})
