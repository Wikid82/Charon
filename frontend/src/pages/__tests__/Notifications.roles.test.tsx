import { screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, it, expect, vi, beforeEach } from 'vitest'

import * as notificationsApi from '../../api/notifications'
import { renderWithQueryClient } from '../../test-utils/renderWithQueryClient'
import Notifications from '../Notifications'

import type { NotificationProvider } from '../../api/notifications'

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string) => (key === 'permissions.hiddenForRole' ? 'Hidden for your role.' : key),
  }),
}))

vi.mock('../../api/notifications', () => ({
  SUPPORTED_NOTIFICATION_PROVIDER_TYPES: ['discord', 'gotify', 'webhook', 'email', 'telegram', 'slack', 'pushover', 'ntfy', 'webpush'],
  getProviders: vi.fn(),
  createProvider: vi.fn(),
  updateProvider: vi.fn(),
  deleteProvider: vi.fn(),
  testProvider: vi.fn(),
  getTemplates: vi.fn(),
  previewProvider: vi.fn(),
  getExternalTemplates: vi.fn(),
  previewExternalTemplate: vi.fn(),
  createExternalTemplate: vi.fn(),
  updateExternalTemplate: vi.fn(),
  deleteExternalTemplate: vi.fn(),
  provisionWebPush: vi.fn(),
  getWebPushVapidPublicKey: vi.fn(),
  subscribeWebPush: vi.fn(),
  listWebPushSubscriptions: vi.fn(),
  unsubscribeWebPush: vi.fn(),
}))

vi.mock('../../utils/toast', () => ({
  toast: { success: vi.fn(), error: vi.fn() },
}))

const roleState = vi.hoisted(() => ({ role: 'admin' as 'admin' | 'user' }))
vi.mock('../../hooks/useAuth', () => ({
  useAuth: () => ({ user: { user_id: 1, role: roleState.role } }),
}))

// As returned to a standard user: url and config are redacted and has_url says whether a URL is set.
const redactedProvider: NotificationProvider = {
  id: 'provider-1',
  name: 'Discord Alerts',
  type: 'discord',
  has_url: true,
  template: 'minimal',
  enabled: true,
  notify_proxy_hosts: true,
  notify_remote_servers: true,
  notify_domains: true,
  notify_certs: true,
  notify_uptime: true,
  notify_security_waf_blocks: false,
  notify_security_acl_denies: false,
  notify_security_rate_limit_hits: false,
  created_at: '2024-01-01T00:00:00Z',
}

const setupProviders = (providers: NotificationProvider[]) => {
  vi.mocked(notificationsApi.getProviders).mockResolvedValue(providers)
  vi.mocked(notificationsApi.getTemplates).mockResolvedValue([])
  vi.mocked(notificationsApi.getExternalTemplates).mockResolvedValue([])
  vi.mocked(notificationsApi.updateProvider).mockResolvedValue(redactedProvider)
  vi.mocked(notificationsApi.listWebPushSubscriptions).mockResolvedValue([])
}

const openEditFor = async (user: ReturnType<typeof userEvent.setup>, providerId: string) => {
  const row = await screen.findByTestId(`provider-row-${providerId}`)
  await user.click(within(row).getByRole('button', { name: 'common.edit' }))
}

describe('Notifications page roles', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    roleState.role = 'user'
  })

  describe('as a standard user', () => {
    it('shows a hidden-for-role note instead of the URL on each provider row', async () => {
      setupProviders([redactedProvider])
      renderWithQueryClient(<Notifications />)

      const row = await screen.findByTestId('provider-row-provider-1')
      expect(within(row).getByText('Hidden for your role.')).toBeInTheDocument()
    })

    it('hides the send-test button on provider rows', async () => {
      setupProviders([redactedProvider])
      renderWithQueryClient(<Notifications />)

      const row = await screen.findByTestId('provider-row-provider-1')
      expect(within(row).queryByRole('button', { name: 'notificationProviders.sendTest' })).not.toBeInTheDocument()
    })

    it('hides the add, edit and delete provider controls', async () => {
      setupProviders([redactedProvider])
      renderWithQueryClient(<Notifications />)

      const row = await screen.findByTestId('provider-row-provider-1')
      expect(screen.queryByTestId('add-provider-btn')).not.toBeInTheDocument()
      expect(within(row).queryByRole('button', { name: 'common.edit' })).not.toBeInTheDocument()
      expect(within(row).queryByRole('button', { name: 'common.delete' })).not.toBeInTheDocument()
    })

    it('hides the template create, manage and edit/delete controls but keeps the template list readable', async () => {
      setupProviders([redactedProvider])
      vi.mocked(notificationsApi.getExternalTemplates).mockResolvedValue([
        { id: 'tpl-1', name: 'Team Template', description: 'Shared layout', config: '{"message":"hi"}' },
      ])
      renderWithQueryClient(<Notifications />)

      const list = await screen.findByTestId('external-templates-list')
      expect(within(list).getByText('Team Template')).toBeInTheDocument()
      expect(screen.queryByRole('button', { name: 'notificationProviders.newTemplate' })).not.toBeInTheDocument()
      expect(screen.queryByRole('button', { name: 'notificationProviders.manageTemplates' })).not.toBeInTheDocument()
      expect(screen.queryByTestId('external-template-edit-tpl-1')).not.toBeInTheDocument()
      expect(screen.queryByTestId('external-template-delete-tpl-1')).not.toBeInTheDocument()
      expect(screen.queryByText('notificationProviders.createTemplate')).not.toBeInTheDocument()
    })
  })

  describe('as an administrator', () => {
    beforeEach(() => {
      roleState.role = 'admin'
    })

    it('shows the stored URL on the row and keeps the test and preview buttons', async () => {
      const user = userEvent.setup({ delay: null })
      setupProviders([{ ...redactedProvider, url: 'https://discord.com/api/webhooks/abc', has_url: undefined }])
      renderWithQueryClient(<Notifications />)

      const row = await screen.findByTestId('provider-row-provider-1')
      expect(within(row).getByText('https://discord.com/api/webhooks/abc')).toBeInTheDocument()
      expect(within(row).getByRole('button', { name: 'notificationProviders.sendTest' })).toBeInTheDocument()

      await openEditFor(user, 'provider-1')
      expect(screen.getByTestId('provider-url')).not.toBeDisabled()
      expect(screen.getByTestId('provider-test-btn')).toBeInTheDocument()
      expect(screen.getByTestId('provider-preview-btn')).toBeInTheDocument()
    })
  })
})
