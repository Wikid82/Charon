import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, it, expect, vi, beforeEach } from 'vitest'

import * as backupsApi from '../../api/backups'
import * as crowdsecApi from '../../api/crowdsec'
import * as featureFlagsApi from '../../api/featureFlags'
import * as presetsApi from '../../api/presets'
import * as securityApi from '../../api/security'
import { renderWithQueryClient } from '../../test-utils/renderWithQueryClient'
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
})
