import { screen, waitFor, within } from '@testing-library/react'
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

const BAN_DIALOG = 'Ban IP Address'
const UNBAN_DIALOG = 'Confirm Unban'
const WHITELIST_DIALOG = 'Remove Whitelist Entry'
const CLOSE_BACKDROP = 'Close'

const decision = {
  id: '1',
  ip: '1.1.1.1',
  reason: 'bot',
  duration: '24h',
  created_at: '2024-01-01T00:00:00Z',
  source: 'manual',
}

const whitelistEntry = {
  uuid: 'uuid-1',
  ip_or_cidr: '192.168.1.1',
  reason: 'Home IP',
  created_at: '2024-01-01T00:00:00Z',
  updated_at: '2024-01-01T00:00:00Z',
}

const renderPage = async () => {
  renderWithQueryClient(<CrowdSecConfig />)
  await screen.findByText('CrowdSec Configuration')
}

const openBanDialog = async () => {
  await userEvent.click(await screen.findByRole('button', { name: /Ban IP/ }))
  return screen.findByRole('dialog', { name: BAN_DIALOG })
}

const openUnbanDialog = async () => {
  await userEvent.click((await screen.findAllByText('Unban'))[0])
  return screen.findByRole('dialog', { name: UNBAN_DIALOG })
}

const openWhitelistDeleteDialog = async () => {
  await userEvent.click(screen.getByRole('tab', { name: 'Whitelist' }))
  await screen.findByTestId('whitelist-ip-input')
  await userEvent.click((await screen.findAllByTestId('whitelist-delete-btn'))[0])
  return screen.findByRole('dialog', { name: WHITELIST_DIALOG })
}

describe('CrowdSecConfig modal keyboard and backdrop behaviour', () => {
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
    vi.mocked(crowdsecApi.listCrowdsecFiles).mockResolvedValue({ files: ['acquis.yaml'] })
    vi.mocked(crowdsecApi.readCrowdsecFile).mockResolvedValue({ content: '' })
    vi.mocked(crowdsecApi.listCrowdsecDecisions).mockResolvedValue({ decisions: [decision] })
    vi.mocked(crowdsecApi.listWhitelists).mockResolvedValue([whitelistEntry])
    vi.mocked(crowdsecApi.banIP).mockResolvedValue(undefined)
    vi.mocked(crowdsecApi.unbanIP).mockResolvedValue(undefined)
    vi.mocked(presetsApi.listCrowdsecPresets).mockResolvedValue({ presets: [] })
    vi.mocked(backupsApi.createBackup).mockResolvedValue({ job_id: 'j', type: 'create', status: 'pending' })
    vi.mocked(featureFlagsApi.getFeatureFlags).mockResolvedValue({ 'feature.crowdsec.console_enrollment': false })
  })

  describe('ban IP dialog', () => {
    it('moves focus to the IP field on open', async () => {
      await renderPage()
      const dialog = await openBanDialog()

      expect(within(dialog).getByLabelText('IP Address')).toHaveFocus()
    })

    it('closes on Escape from the IP field', async () => {
      await renderPage()
      await openBanDialog()

      await userEvent.keyboard('{Escape}')

      await waitFor(() => expect(screen.queryByRole('dialog', { name: BAN_DIALOG })).not.toBeInTheDocument())
      expect(crowdsecApi.banIP).not.toHaveBeenCalled()
    })

    it('submits on Ctrl+Enter when focus is outside the IP field', async () => {
      await renderPage()
      const dialog = await openBanDialog()
      await userEvent.type(within(dialog).getByLabelText('IP Address'), '203.0.113.9')
      within(dialog).getByLabelText('Duration').focus()

      await userEvent.keyboard('{Control>}{Enter}{/Control}')

      await waitFor(() => expect(crowdsecApi.banIP).toHaveBeenCalledWith('203.0.113.9', '24h', ''))
    })

    it('submits once on plain Enter in the IP field', async () => {
      await renderPage()
      const dialog = await openBanDialog()
      await userEvent.type(within(dialog).getByLabelText('IP Address'), '203.0.113.10{Enter}')

      await waitFor(() => expect(crowdsecApi.banIP).toHaveBeenCalledTimes(1))
      expect(crowdsecApi.banIP).toHaveBeenCalledWith('203.0.113.10', '24h', '')
    })

    it('submits once on Ctrl+Enter in the IP field', async () => {
      await renderPage()
      const dialog = await openBanDialog()
      await userEvent.type(within(dialog).getByLabelText('IP Address'), '203.0.113.11')

      await userEvent.keyboard('{Control>}{Enter}{/Control}')

      await waitFor(() => expect(crowdsecApi.banIP).toHaveBeenCalledTimes(1))
      expect(crowdsecApi.banIP).toHaveBeenCalledWith('203.0.113.11', '24h', '')
    })

    it('does not submit on Ctrl+Enter when the IP field is empty', async () => {
      await renderPage()
      const dialog = await openBanDialog()
      within(dialog).getByLabelText('IP Address').focus()

      await userEvent.keyboard('{Control>}{Enter}{/Control}')

      expect(crowdsecApi.banIP).not.toHaveBeenCalled()
      expect(screen.getByRole('dialog', { name: BAN_DIALOG })).toBeInTheDocument()
    })

    it('does not submit on a whitespace-only IP with Ctrl+Enter', async () => {
      await renderPage()
      const dialog = await openBanDialog()
      await userEvent.type(within(dialog).getByLabelText('IP Address'), '   ')

      await userEvent.keyboard('{Control>}{Enter}{/Control}')

      expect(crowdsecApi.banIP).not.toHaveBeenCalled()
    })

    it('submits once on Ctrl+Enter from the duration select', async () => {
      await renderPage()
      const dialog = await openBanDialog()
      await userEvent.type(within(dialog).getByLabelText('IP Address'), '203.0.113.12')
      within(dialog).getByLabelText('Duration').focus()

      await userEvent.keyboard('{Control>}{Enter}{/Control}')

      await waitFor(() => expect(crowdsecApi.banIP).toHaveBeenCalledTimes(1))
      expect(crowdsecApi.banIP).toHaveBeenCalledWith('203.0.113.12', '24h', '')
    })

    it('does not start a second ban while the first is still in flight', async () => {
      vi.mocked(crowdsecApi.banIP).mockReturnValue(new Promise<void>(() => {}))
      await renderPage()
      const dialog = await openBanDialog()
      await userEvent.type(within(dialog).getByLabelText('IP Address'), '203.0.113.13')

      await userEvent.keyboard('{Control>}{Enter}{/Control}')
      await userEvent.keyboard('{Control>}{Enter}{/Control}')

      await waitFor(() => expect(crowdsecApi.banIP).toHaveBeenCalledTimes(1))
    })

    it('closes from the backdrop close control', async () => {
      await renderPage()
      const dialog = await openBanDialog()

      await userEvent.click(within(dialog).getByRole('button', { name: CLOSE_BACKDROP }))

      await waitFor(() => expect(screen.queryByRole('dialog', { name: BAN_DIALOG })).not.toBeInTheDocument())
    })

    it.each(['{Enter}', ' '])('activates the backdrop close control with the %j key', async (key) => {
      await renderPage()
      const dialog = await openBanDialog()
      const backdrop = within(dialog).getByRole('button', { name: CLOSE_BACKDROP })
      backdrop.focus()

      await userEvent.keyboard(key)

      await waitFor(() => expect(screen.queryByRole('dialog', { name: BAN_DIALOG })).not.toBeInTheDocument())
    })
  })

  describe('unban confirmation dialog', () => {
    it('moves focus to Cancel on open', async () => {
      await renderPage()
      const dialog = await openUnbanDialog()

      expect(within(dialog).getByRole('button', { name: 'Cancel' })).toHaveFocus()
    })

    it('closes on Escape without unbanning', async () => {
      await renderPage()
      await openUnbanDialog()

      await userEvent.keyboard('{Escape}')

      await waitFor(() => expect(screen.queryByRole('dialog', { name: UNBAN_DIALOG })).not.toBeInTheDocument())
      expect(crowdsecApi.unbanIP).not.toHaveBeenCalled()
    })

    it('closes from the backdrop close control', async () => {
      await renderPage()
      const dialog = await openUnbanDialog()

      await userEvent.click(within(dialog).getByRole('button', { name: CLOSE_BACKDROP }))

      await waitFor(() => expect(screen.queryByRole('dialog', { name: UNBAN_DIALOG })).not.toBeInTheDocument())
      expect(crowdsecApi.unbanIP).not.toHaveBeenCalled()
    })
  })

  describe('whitelist remove dialog', () => {
    it('moves focus to Cancel on open', async () => {
      await renderPage()
      const dialog = await openWhitelistDeleteDialog()

      expect(within(dialog).getByRole('button', { name: 'Cancel' })).toHaveFocus()
    })

    it('closes on Escape without deleting', async () => {
      await renderPage()
      await openWhitelistDeleteDialog()

      await userEvent.keyboard('{Escape}')

      await waitFor(() => expect(screen.queryByRole('dialog', { name: WHITELIST_DIALOG })).not.toBeInTheDocument())
      expect(crowdsecApi.deleteWhitelist).not.toHaveBeenCalled()
    })
  })
})
