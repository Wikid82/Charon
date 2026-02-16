import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import Notifications from '../Notifications'
import { renderWithQueryClient } from '../../test-utils/renderWithQueryClient'
import * as notificationsApi from '../../api/notifications'
import { toast } from '../../utils/toast'
import type { NotificationProvider } from '../../api/notifications'

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string) => key,
  }),
}))

vi.mock('../../api/notifications', () => ({
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
}))

vi.mock('../../utils/toast', () => ({
  toast: {
    success: vi.fn(),
    error: vi.fn(),
  },
}))

const baseProvider: NotificationProvider = {
  id: 'provider-1',
  name: 'Discord Alerts',
  type: 'discord',
  url: 'https://discord.com/api/webhooks/abc',
  config: '{"message":"test"}',
  template: 'minimal',
  enabled: true,
  notify_proxy_hosts: true,
  notify_remote_servers: true,
  notify_domains: true,
  notify_certs: true,
  notify_uptime: true,
  created_at: '2024-01-01T00:00:00Z',
}

const setupMocks = (providers: NotificationProvider[] = []) => {
  vi.mocked(notificationsApi.getProviders).mockResolvedValue(providers)
  vi.mocked(notificationsApi.getTemplates).mockResolvedValue([])
  vi.mocked(notificationsApi.getExternalTemplates).mockResolvedValue([])
  vi.mocked(notificationsApi.createProvider).mockResolvedValue(baseProvider)
  vi.mocked(notificationsApi.updateProvider).mockResolvedValue(baseProvider)
}

describe('Notifications', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    setupMocks()
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('rejects invalid protocol URLs', async () => {
    const user = userEvent.setup()
    renderWithQueryClient(<Notifications />)

    await user.click(await screen.findByTestId('add-provider-btn'))
    await user.type(screen.getByTestId('provider-name'), 'Webhook')
    await user.type(screen.getByTestId('provider-url'), 'ftp://example.com/hook')
    await user.click(screen.getByTestId('provider-save-btn'))

    expect(screen.getByTestId('provider-url-error')).toHaveTextContent('notificationProviders.invalidUrl')
    expect(notificationsApi.createProvider).not.toHaveBeenCalled()
  })

  it('rejects malformed URLs', async () => {
    const user = userEvent.setup()
    renderWithQueryClient(<Notifications />)

    await user.click(await screen.findByTestId('add-provider-btn'))
    await user.type(screen.getByTestId('provider-name'), 'Webhook')
    await user.type(screen.getByTestId('provider-url'), 'not-a-url')
    await user.click(screen.getByTestId('provider-save-btn'))

    expect(screen.getByTestId('provider-url-error')).toHaveTextContent('notificationProviders.invalidUrl')
    expect(notificationsApi.createProvider).not.toHaveBeenCalled()
  })

  it('accepts a valid https URL', async () => {
    const user = userEvent.setup()
    renderWithQueryClient(<Notifications />)

    await user.click(await screen.findByTestId('add-provider-btn'))
    await user.type(screen.getByTestId('provider-name'), 'Webhook')
    await user.type(screen.getByTestId('provider-url'), 'https://example.com/webhook')
    await user.click(screen.getByTestId('provider-save-btn'))

    await waitFor(() => {
      expect(notificationsApi.createProvider).toHaveBeenCalled()
    })

    const payload = vi.mocked(notificationsApi.createProvider).mock.calls[0][0]
    expect(payload.url).toBe('https://example.com/webhook')
  })

  it('accepts a valid http URL', async () => {
    const user = userEvent.setup()
    renderWithQueryClient(<Notifications />)

    await user.click(await screen.findByTestId('add-provider-btn'))
    await user.type(screen.getByTestId('provider-name'), 'Webhook')
    await user.type(screen.getByTestId('provider-url'), 'http://example.com/webhook')
    await user.click(screen.getByTestId('provider-save-btn'))

    await waitFor(() => {
      expect(notificationsApi.createProvider).toHaveBeenCalled()
    })

    const payload = vi.mocked(notificationsApi.createProvider).mock.calls[0][0]
    expect(payload.url).toBe('http://example.com/webhook')
  })

  it('shows and hides the update indicator after save', async () => {
    setupMocks([baseProvider])

    const user = userEvent.setup()
    renderWithQueryClient(<Notifications />)

    const row = await screen.findByTestId(`provider-row-${baseProvider.id}`)
    const buttons = within(row).getAllByRole('button')
    await user.click(buttons[1])

    await user.click(screen.getByTestId('provider-save-btn'))

    expect(notificationsApi.updateProvider).toHaveBeenCalled()

    expect(screen.getByTestId(`provider-update-indicator-${baseProvider.id}`)).toBeInTheDocument()
    expect(toast.success).toHaveBeenCalledWith('common.saved')

    await waitFor(
      () => {
        expect(screen.queryByTestId(`provider-update-indicator-${baseProvider.id}`)).toBeNull()
      },
      { timeout: 4000 },
    )
  })

  it('cleans up the update indicator timer on unmount', async () => {
    setupMocks([baseProvider])

    const clearTimeoutSpy = vi.spyOn(window, 'clearTimeout')
    const user = userEvent.setup()
    const { unmount } = renderWithQueryClient(<Notifications />)

    const row = await screen.findByTestId(`provider-row-${baseProvider.id}`)
    const buttons = within(row).getAllByRole('button')
    await user.click(buttons[1])

    await user.click(screen.getByTestId('provider-save-btn'))

    expect(notificationsApi.updateProvider).toHaveBeenCalled()
    expect(screen.getByTestId(`provider-update-indicator-${baseProvider.id}`)).toBeInTheDocument()
    unmount()

    expect(clearTimeoutSpy).toHaveBeenCalled()
  })

  it('resets event checkboxes when switching from edit to add', async () => {
    const providerWithDisabledEvents: NotificationProvider = {
      ...baseProvider,
      notify_proxy_hosts: false,
      notify_remote_servers: false,
    }

    setupMocks([providerWithDisabledEvents])
    const user = userEvent.setup()
    renderWithQueryClient(<Notifications />)

    const row = await screen.findByTestId(`provider-row-${providerWithDisabledEvents.id}`)
    const buttons = within(row).getAllByRole('button')
    await user.click(buttons[1])

    const notifyProxyHosts = screen.getByTestId('notify-proxy-hosts') as HTMLInputElement
    expect(notifyProxyHosts.checked).toBe(false)

    await user.click(screen.getByRole('button', { name: 'common.cancel' }))
    await user.click(await screen.findByTestId('add-provider-btn'))

    const resetNotifyProxyHosts = screen.getByTestId('notify-proxy-hosts') as HTMLInputElement
    expect(resetNotifyProxyHosts.checked).toBe(true)
  })
})
