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

const MISSING = [{ type: 'collections', name: 'crowdsecurity/caddy' }]

const importWith = async (response: crowdsecApi.ImportCrowdsecResult) => {
  vi.mocked(crowdsecApi.importCrowdsecConfig).mockResolvedValue(response)
  renderWithQueryClient(<CrowdSecConfig />)
  await screen.findByText('CrowdSec Configuration')
  await userEvent.upload(screen.getByTestId('import-file'), new File(['data'], 'cfg.tar.gz'))
  await userEvent.click(screen.getByTestId('import-btn'))
  await waitFor(() => expect(crowdsecApi.importCrowdsecConfig).toHaveBeenCalled())
}

describe('CrowdSecConfig hub items panel', () => {
  beforeEach(() => {
    vi.clearAllMocks()
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

  it('offers the install action after an import that reports missing items', async () => {
    await importWith({ status: 'imported', hub_items: { in_export: 2, skipped_invalid: 0, missing: MISSING } })

    expect(await screen.findByRole('button', { name: 'Install missing hub items' })).toBeEnabled()
    expect(screen.getByText('crowdsecurity/caddy', { exact: true })).toBeInTheDocument()
  })

  it.each([
    ['no hub_items field', { status: 'imported' }],
    ['nothing missing', { status: 'imported', hub_items: { in_export: 2, skipped_invalid: 0, missing: [] } }],
  ])('shows no panel with %s', async (_label, response) => {
    await importWith(response)

    await waitFor(() => expect(crowdsecApi.importCrowdsecConfig).toHaveBeenCalled())
    expect(screen.queryByTestId('hub-items-panel')).not.toBeInTheDocument()
  })

  it('clears the panel when another file is selected', async () => {
    await importWith({ status: 'imported', hub_items: { in_export: 2, skipped_invalid: 0, missing: MISSING } })
    await screen.findByTestId('hub-items-panel')

    await userEvent.upload(screen.getByTestId('import-file'), new File(['other'], 'other.tar.gz'))

    expect(screen.queryByTestId('hub-items-panel')).not.toBeInTheDocument()
  })

  it('shows the generic error and allows a retry when a restore answers 500 with a partial body', async () => {
    await importWith({ status: 'imported', hub_items: { in_export: 2, skipped_invalid: 0, missing: MISSING } })
    const partialBody = {
      error: 'the installed hub items could not be read; some items may already have been installed',
      installed: [{ type: 'collections', name: 'crowdsecurity/caddy' }],
      already_installed: [],
      failed: [],
    }
    vi.mocked(crowdsecApi.restoreHubItems).mockRejectedValue(
      new AxiosError('failed', 'ERR_BAD_RESPONSE', undefined, undefined, {
        status: 500,
        data: partialBody,
      } as AxiosResponse),
    )

    await userEvent.click(await screen.findByRole('button', { name: 'Install missing hub items' }))

    expect(await screen.findByText('The hub items could not be installed. Try again.')).toBeInTheDocument()
    expect(screen.queryByText(/installed, /i)).not.toBeInTheDocument()
    expect(screen.getByTestId('hub-items-panel')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Install missing hub items' })).toBeEnabled()
  })
})

describe('CrowdSecConfig kept files note', () => {
  beforeEach(() => {
    vi.clearAllMocks()
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

  it('lists the kept files as plain text in the import card', async () => {
    await importWith({ status: 'imported', kept_files: ['acquis.yaml', 'notifications/x.yaml'] })

    const note = await screen.findByTestId('import-kept-files')
    expect(note).toHaveTextContent(
      'These files were not in the archive, so the current ones were kept: acquis.yaml, notifications/x.yaml',
    )
  })

  it('sanitises archive-derived names and does not render markup', async () => {
    await importWith({ status: 'imported', kept_files: ['notifications/<b>x</b>\u202e\n.yaml'] })

    const note = await screen.findByTestId('import-kept-files')
    expect(note).toHaveTextContent('notifications/<b>x</b>.yaml')
    expect(within(note).queryByText('x')).not.toBeInTheDocument()
  })

  it.each([
    ['no kept_files field', { status: 'imported' }],
    ['an empty kept_files list', { status: 'imported', kept_files: [] }],
  ])('shows no note with %s', async (_label, response) => {
    await importWith(response)

    expect(screen.queryByTestId('import-kept-files')).not.toBeInTheDocument()
  })

  it('clears the note when another file is selected', async () => {
    await importWith({ status: 'imported', kept_files: ['profiles.yaml'] })
    await screen.findByTestId('import-kept-files')

    await userEvent.upload(screen.getByTestId('import-file'), new File(['other'], 'other.tar.gz'))

    expect(screen.queryByTestId('import-kept-files')).not.toBeInTheDocument()
  })
})

describe('CrowdSecConfig needs-values note', () => {
  beforeEach(() => {
    vi.clearAllMocks()
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

  it('lists the files that still need values as one inline line without a new toast', async () => {
    await importWith({ status: 'imported', needs_values: ['notifications/http_default.yaml', 'notifications/slack.yaml'] })

    const note = await screen.findByTestId('import-needs-values')
    expect(note).toHaveTextContent(
      'Some values in these notification files were blanked when the configuration was exported, so they need to be entered before they work: notifications/http_default.yaml, notifications/slack.yaml',
    )
    expect(toast.success).toHaveBeenCalledTimes(1)
    expect(toast.error).not.toHaveBeenCalled()
    expect(toast.info).not.toHaveBeenCalled()
  })

  it('shows the first 10 names and the number of the remaining ones', async () => {
    const names = Array.from({ length: 13 }, (_, i) => `notifications/n${i + 1}.yaml`)
    await importWith({ status: 'imported', needs_values: names })

    const note = await screen.findByTestId('import-needs-values')
    expect(note).toHaveTextContent('notifications/n10.yaml, and 3 more')
    expect(note).not.toHaveTextContent('notifications/n11.yaml')
  })

  it('lists exactly 10 names without a remainder', async () => {
    const names = Array.from({ length: 10 }, (_, i) => `notifications/n${i + 1}.yaml`)
    await importWith({ status: 'imported', needs_values: names })

    const note = await screen.findByTestId('import-needs-values')
    expect(note).toHaveTextContent('notifications/n10.yaml')
    expect(note).not.toHaveTextContent('more')
  })

  it('sanitises archive-derived names and does not render markup', async () => {
    await importWith({ status: 'imported', needs_values: ['notifications/<b>x</b>\u202e\n.yaml'] })

    const note = await screen.findByTestId('import-needs-values')
    expect(note).toHaveTextContent('notifications/<b>x</b>.yaml')
    expect(within(note).queryByText('x')).not.toBeInTheDocument()
  })

  it.each([
    ['no needs_values field', { status: 'imported' }],
    ['an empty needs_values list', { status: 'imported', needs_values: [] }],
  ])('shows no line with %s', async (_label, response) => {
    await importWith(response)

    await waitFor(() => expect(crowdsecApi.importCrowdsecConfig).toHaveBeenCalled())
    expect(screen.queryByTestId('import-needs-values')).not.toBeInTheDocument()
  })

  it('clears the line when another file is selected', async () => {
    await importWith({ status: 'imported', needs_values: ['notifications/http_default.yaml'] })
    await screen.findByTestId('import-needs-values')

    await userEvent.upload(screen.getByTestId('import-file'), new File(['other'], 'other.tar.gz'))

    expect(screen.queryByTestId('import-needs-values')).not.toBeInTheDocument()
  })
})
