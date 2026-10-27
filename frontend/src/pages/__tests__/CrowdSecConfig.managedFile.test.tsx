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

const MANAGED_FILE_TEXT = 'Charon manages this file. Use the Whitelist tab to change the allow list.'

const refusal = (status: number, body: { error: string; error_code?: string }): AxiosError =>
  new AxiosError(body.error, 'ERR_BAD_REQUEST', undefined, undefined, {
    status,
    data: body,
  } as AxiosResponse)

const renderPage = async () => {
  renderWithQueryClient(<CrowdSecConfig />)
  await screen.findByText('CrowdSec Configuration')
}

describe('CrowdSecConfig managed files and import notes', () => {
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
    vi.mocked(crowdsecApi.listCrowdsecFiles).mockResolvedValue({ files: ['parsers/s02-enrich/charon-whitelist.yaml'] })
    vi.mocked(crowdsecApi.readCrowdsecFile).mockResolvedValue({ content: 'managed: true' })
    vi.mocked(crowdsecApi.listCrowdsecDecisions).mockResolvedValue({ decisions: [] })
    vi.mocked(presetsApi.listCrowdsecPresets).mockResolvedValue({ presets: [] })
    vi.mocked(backupsApi.createBackup).mockResolvedValue({ job_id: 'j', type: 'create', status: 'pending' })
    vi.mocked(featureFlagsApi.getFeatureFlags).mockResolvedValue({ 'feature.crowdsec.console_enrollment': false })
  })

  it('shows the translated text when a save is refused because Charon manages the file', async () => {
    vi.mocked(crowdsecApi.writeCrowdsecFile).mockRejectedValue(
      refusal(409, { error: 'Charon manages this file. Change the allow list on the Whitelist tab.', error_code: 'file_managed_by_charon' }),
    )

    await renderPage()
    await screen.findByText('parsers/s02-enrich/charon-whitelist.yaml')
    await userEvent.selectOptions(screen.getByTestId('crowdsec-file-select'), 'parsers/s02-enrich/charon-whitelist.yaml')
    await waitFor(() => expect(crowdsecApi.readCrowdsecFile).toHaveBeenCalled())
    await userEvent.click(screen.getByText('Save'))

    await waitFor(() => expect(toast.error).toHaveBeenCalledWith(MANAGED_FILE_TEXT))
  })

  it('shows the passive whitelist notes after an import that regenerated the file and found others', async () => {
    vi.mocked(crowdsecApi.importCrowdsecConfig).mockResolvedValue({
      status: 'imported',
      whitelist_regenerated: true,
      other_whitelist_files: ['parsers/s02-enrich/a.yaml', 'parsers/s02-enrich/b.yaml'],
    })

    await renderPage()
    await userEvent.upload(screen.getByTestId('import-file'), new File(['data'], 'cfg.tar.gz'))
    await userEvent.click(screen.getByTestId('import-btn'))

    expect(await screen.findByTestId('import-whitelist-regenerated')).toHaveTextContent(
      "Charon regenerated its whitelist file from this instance's database.",
    )
    expect(screen.getByTestId('import-other-whitelist-files')).toHaveTextContent(
      '2 other whitelist files were found and left unchanged: parsers/s02-enrich/a.yaml, parsers/s02-enrich/b.yaml',
    )
  })

  it('shows neither whitelist note when the import reports neither field', async () => {
    vi.mocked(crowdsecApi.importCrowdsecConfig).mockResolvedValue({ status: 'imported' })

    await renderPage()
    await userEvent.upload(screen.getByTestId('import-file'), new File(['data'], 'cfg.tar.gz'))
    await userEvent.click(screen.getByTestId('import-btn'))

    await waitFor(() => expect(toast.success).toHaveBeenCalled())
    expect(screen.queryByTestId('import-whitelist-regenerated')).not.toBeInTheDocument()
    expect(screen.queryByTestId('import-other-whitelist-files')).not.toBeInTheDocument()
  })

  it('clears the whitelist notes when a different file is picked', async () => {
    vi.mocked(crowdsecApi.importCrowdsecConfig).mockResolvedValue({ status: 'imported', whitelist_regenerated: true })

    await renderPage()
    await userEvent.upload(screen.getByTestId('import-file'), new File(['data'], 'cfg.tar.gz'))
    await userEvent.click(screen.getByTestId('import-btn'))
    expect(await screen.findByTestId('import-whitelist-regenerated')).toBeInTheDocument()

    await userEvent.upload(screen.getByTestId('import-file'), new File(['other'], 'next.tar.gz'))

    expect(screen.queryByTestId('import-whitelist-regenerated')).not.toBeInTheDocument()
  })
})
