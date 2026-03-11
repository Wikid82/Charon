import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'

import * as notificationsApi from '../../api/notifications'
import { renderWithQueryClient } from '../../test-utils/renderWithQueryClient'
import { toast } from '../../utils/toast'
import Notifications from '../Notifications'

import type { NotificationProvider } from '../../api/notifications'

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string) => key,
  }),
}))

vi.mock('../../api/notifications', () => ({
  SUPPORTED_NOTIFICATION_PROVIDER_TYPES: ['discord', 'gotify', 'webhook', 'email', 'telegram'],
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
  notify_security_waf_blocks: false,
  notify_security_acl_denies: false,
  notify_security_rate_limit_hits: false,
  created_at: '2024-01-01T00:00:00Z',
}

const setupMocks = (providers: NotificationProvider[] = []) => {
  vi.mocked(notificationsApi.getProviders).mockResolvedValue(providers)
  vi.mocked(notificationsApi.getTemplates).mockResolvedValue([])
  vi.mocked(notificationsApi.getExternalTemplates).mockResolvedValue([])
  vi.mocked(notificationsApi.createProvider).mockResolvedValue(baseProvider)
  vi.mocked(notificationsApi.updateProvider).mockResolvedValue(baseProvider)
}

let user: ReturnType<typeof userEvent.setup>

describe('Notifications', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    setupMocks()
    user = userEvent.setup()
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('rejects invalid protocol URLs', async () => {
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
    expect(payload.type).toBe('discord')
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
    expect(payload.type).toBe('discord')
  })

  it('shows supported provider type options', async () => {
    const user = userEvent.setup()
    renderWithQueryClient(<Notifications />)

    await user.click(await screen.findByTestId('add-provider-btn'))

    const typeSelect = screen.getByTestId('provider-type') as HTMLSelectElement
    const options = Array.from(typeSelect.options)

    expect(options).toHaveLength(5)
    expect(options.map((option) => option.value)).toEqual(['discord', 'gotify', 'webhook', 'email', 'telegram'])
    expect(typeSelect.disabled).toBe(false)
  })

  it('associates provider type label with select control', async () => {
    const user = userEvent.setup()
    renderWithQueryClient(<Notifications />)

    await user.click(await screen.findByTestId('add-provider-btn'))

    const typeSelect = screen.getByTestId('provider-type')
    expect(typeSelect).toHaveAttribute('id', 'provider-type')
    expect(screen.getByLabelText('common.type')).toBe(typeSelect)
  })

  it('submits selected provider type without forcing discord', async () => {
    renderWithQueryClient(<Notifications />)

    await user.click(await screen.findByTestId('add-provider-btn'))
    await user.selectOptions(screen.getByTestId('provider-type'), 'webhook')
    await user.type(screen.getByTestId('provider-name'), 'Normalized Provider')
    await user.type(screen.getByTestId('provider-url'), 'https://example.com/webhook')

    const typeSelect = screen.getByTestId('provider-type') as HTMLSelectElement
    expect(typeSelect.value).toBe('webhook')

    await user.click(screen.getByTestId('provider-save-btn'))

    await waitFor(() => {
      expect(notificationsApi.createProvider).toHaveBeenCalled()
    })

    const payload = vi.mocked(notificationsApi.createProvider).mock.calls[0][0]
    expect(payload.type).toBe('webhook')
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

  it('renders external template loading and rows when templates are present', async () => {
    const template = {
      id: 'template-1',
      name: 'Ops Payload',
      description: 'Template for ops alerts',
      template: 'custom' as const,
      config: '{"text":"{{.Message}}"}',
      created_at: '2024-01-01T00:00:00Z',
      updated_at: '2024-01-01T00:00:00Z',
    }

    vi.mocked(notificationsApi.getExternalTemplates).mockReturnValue(new Promise(() => {}))
    const { unmount } = renderWithQueryClient(<Notifications />)
    await userEvent.click(await screen.findByRole('button', { name: 'notificationProviders.manageTemplates' }))
    expect(screen.getByTestId('external-templates-loading')).toBeInTheDocument()
    unmount()

    vi.mocked(notificationsApi.getExternalTemplates).mockResolvedValue([template])
    renderWithQueryClient(<Notifications />)
    await userEvent.click(await screen.findByRole('button', { name: 'notificationProviders.manageTemplates' }))

    expect(await screen.findByTestId('external-template-row-template-1')).toBeInTheDocument()
    expect(screen.getByText('Ops Payload')).toBeInTheDocument()
  })

  it('opens external template editor and deletes template on confirm', async () => {
    const template = {
      id: 'template-2',
      name: 'Security Payload',
      description: 'Template for security alerts',
      template: 'custom' as const,
      config: '{"text":"{{.Message}}"}',
      created_at: '2024-01-01T00:00:00Z',
      updated_at: '2024-01-01T00:00:00Z',
    }

    vi.mocked(notificationsApi.getExternalTemplates).mockResolvedValue([template])
    vi.mocked(notificationsApi.deleteExternalTemplate).mockResolvedValue()
    const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(true)

    const user = userEvent.setup()
    renderWithQueryClient(<Notifications />)
    await user.click(await screen.findByRole('button', { name: 'notificationProviders.manageTemplates' }))

    const row = await screen.findByTestId('external-template-row-template-2')
    expect(row).toBeInTheDocument()

    await user.click(screen.getByTestId('external-template-edit-template-2'))
    await waitFor(() => {
      expect((screen.getByTestId('template-name') as HTMLInputElement).value).toBe('Security Payload')
    })

    await user.click(screen.getByTestId('external-template-delete-template-2'))
    await waitFor(() => {
      expect(confirmSpy).toHaveBeenCalled()
      expect(notificationsApi.deleteExternalTemplate).toHaveBeenCalledWith('template-2')
    })

    confirmSpy.mockRestore()
  })

  it('does not render a standalone security notifications section', async () => {
    renderWithQueryClient(<Notifications />)
    await screen.findByTestId('add-provider-btn')
    expect(screen.queryByTestId('security-notifications-section')).toBeNull()
    expect(screen.queryByTestId('security-compatibility-banner')).toBeNull()
  })

  it('shows security event subscription controls in provider form', async () => {
    const user = userEvent.setup()
    renderWithQueryClient(<Notifications />)

    await user.click(await screen.findByTestId('add-provider-btn'))

    expect(screen.getByTestId('notify-security-waf-blocks')).toBeInTheDocument()
    expect(screen.getByTestId('notify-security-acl-denies')).toBeInTheDocument()
    expect(screen.getByTestId('notify-security-rate-limit-hits')).toBeInTheDocument()
  })

  it('keeps add-provider guidance aligned with Discord webhook UX', async () => {
    const user = userEvent.setup()
    renderWithQueryClient(<Notifications />)
    await user.click(await screen.findByTestId('add-provider-btn'))

    const typeSelect = screen.getByTestId('provider-type') as HTMLSelectElement
    expect(typeSelect.value).toBe('discord')
    expect(screen.getByTestId('provider-url')).toHaveAttribute('placeholder', 'https://discord.com/api/webhooks/...')
    expect(screen.queryByRole('link')).toBeNull()
  })

  it('submits gotify token on create for gotify provider mode', async () => {
    const user = userEvent.setup()
    renderWithQueryClient(<Notifications />)

    await user.click(await screen.findByTestId('add-provider-btn'))
    await user.selectOptions(screen.getByTestId('provider-type'), 'gotify')
    await user.type(screen.getByTestId('provider-name'), 'Gotify Alerts')
    await user.type(screen.getByTestId('provider-url'), 'https://gotify.example.com/message')
    await user.type(screen.getByTestId('provider-gotify-token'), 'super-secret-token')
    await user.click(screen.getByTestId('provider-save-btn'))

    await waitFor(() => {
      expect(notificationsApi.createProvider).toHaveBeenCalled()
    })

    const payload = vi.mocked(notificationsApi.createProvider).mock.calls[0][0]
    expect(payload.type).toBe('gotify')
    expect(payload.token).toBe('super-secret-token')
  })

  it('uses masked gotify token input and never pre-fills token on edit', async () => {
    const gotifyProvider: NotificationProvider = {
      ...baseProvider,
      id: 'provider-gotify',
      type: 'gotify',
      url: 'https://gotify.example.com/message',
    }

    setupMocks([gotifyProvider])

    const user = userEvent.setup()
    renderWithQueryClient(<Notifications />)

    const row = await screen.findByTestId('provider-row-provider-gotify')
    const buttons = within(row).getAllByRole('button')
    await user.click(buttons[1])

    const tokenInput = screen.getByTestId('provider-gotify-token') as HTMLInputElement
    expect(tokenInput.type).toBe('password')
    expect(tokenInput.value).toBe('')
  })

  it('renders external template action buttons and skips delete when confirm is cancelled', async () => {
    const template = {
      id: 'template-cancel',
      name: 'Cancel Delete Template',
      description: 'Template used for cancel delete branch',
      template: 'custom' as const,
      config: '{"text":"{{.Message}}"}',
      created_at: '2024-01-01T00:00:00Z',
      updated_at: '2024-01-01T00:00:00Z',
    }

    vi.mocked(notificationsApi.getExternalTemplates).mockResolvedValue([template])
    vi.mocked(notificationsApi.deleteExternalTemplate).mockResolvedValue()
    const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(false)

    const user = userEvent.setup()
    renderWithQueryClient(<Notifications />)
    await user.click(await screen.findByRole('button', { name: 'notificationProviders.manageTemplates' }))

    expect(await screen.findByTestId('external-template-row-template-cancel')).toBeInTheDocument()

    const editButton = screen.getByTestId('external-template-edit-template-cancel')
    const deleteButton = screen.getByTestId('external-template-delete-template-cancel')

    await user.click(editButton)
    await waitFor(() => {
      expect((screen.getByTestId('template-name') as HTMLInputElement).value).toBe('Cancel Delete Template')
    })

    await user.click(deleteButton)

    expect(confirmSpy).toHaveBeenCalled()
    expect(notificationsApi.deleteExternalTemplate).not.toHaveBeenCalled()

    confirmSpy.mockRestore()
  })

  it('renders non-discord providers with explicit deprecated and non-dispatch messaging', async () => {
    const legacyProvider: NotificationProvider = {
      ...baseProvider,
      id: 'legacy-provider',
      name: 'Legacy Slack',
      type: 'slack',
      enabled: false,
    }

    setupMocks([legacyProvider])

    renderWithQueryClient(<Notifications />)

    const legacyRow = await screen.findByTestId('provider-row-legacy-provider')
    expect(within(legacyRow).getAllByRole('button')).toHaveLength(1)
    expect(screen.getByTestId('provider-deprecated-status-legacy-provider')).toHaveTextContent('notificationProviders.deprecatedReadOnly')
    expect(screen.getByTestId('provider-nondispatch-status-legacy-provider')).toHaveTextContent('notificationProviders.nonDispatch')
    expect(screen.getByTestId('provider-deprecated-message-legacy-provider')).toHaveTextContent('notificationProviders.deprecatedProviderMessage')
  })

  it('submits provider test action from form using normalized discord type', async () => {
    vi.mocked(notificationsApi.testProvider).mockResolvedValue()
    setupMocks([baseProvider])

    const user = userEvent.setup()
    renderWithQueryClient(<Notifications />)

    const row = await screen.findByTestId(`provider-row-${baseProvider.id}`)
    await user.click(within(row).getByRole('button', { name: /edit/i }))

    await user.click(screen.getByTestId('provider-test-btn'))

    await waitFor(() => {
      expect(notificationsApi.testProvider).toHaveBeenCalled()
    })

    const payload = vi.mocked(notificationsApi.testProvider).mock.calls[0][0]
    expect(payload.type).toBe('discord')
  })

  it('uses previewProvider for non-uuid template selections', async () => {
    vi.mocked(notificationsApi.previewProvider).mockResolvedValue({
      rendered: '{"message":"preview"}',
      parsed: undefined,
    })

    const user = userEvent.setup()
    renderWithQueryClient(<Notifications />)

    await user.click(await screen.findByTestId('add-provider-btn'))
    await user.type(screen.getByTestId('provider-name'), 'Preview Provider')
    await user.type(screen.getByTestId('provider-url'), 'https://example.com/webhook')
    await user.click(screen.getByTestId('provider-preview-btn'))

    await waitFor(() => {
      expect(notificationsApi.previewProvider).toHaveBeenCalled()
    })
  })

  it('treats empty legacy type as unsupported and keeps row read-only', async () => {
    const emptyTypeProvider: NotificationProvider = {
      ...baseProvider,
      id: 'provider-empty-type',
      type: '',
    }

    setupMocks([emptyTypeProvider])

    renderWithQueryClient(<Notifications />)

    const row = await screen.findByTestId('provider-row-provider-empty-type')
    const buttons = within(row).getAllByRole('button')
    expect(buttons).toHaveLength(1)
    expect(screen.getByTestId('provider-deprecated-status-provider-empty-type')).toHaveTextContent('notificationProviders.deprecatedReadOnly')
  })

  it('triggers row-level send test action with discord payload', async () => {
    setupMocks([baseProvider])
    vi.mocked(notificationsApi.testProvider).mockResolvedValue()

    const user = userEvent.setup()
    renderWithQueryClient(<Notifications />)

    const row = await screen.findByTestId(`provider-row-${baseProvider.id}`)
    const buttons = within(row).getAllByRole('button')

    await user.click(buttons[0])

    await waitFor(() => {
      expect(notificationsApi.testProvider).toHaveBeenCalled()
    })

    const payload = vi.mocked(notificationsApi.testProvider).mock.calls[0][0]
    expect(payload.type).toBe('discord')
  })

  it('shows token-stored indicator when editing provider with has_token=true', async () => {
    const gotifyProviderWithToken: NotificationProvider = {
      ...baseProvider,
      id: 'provider-gotify-has-token',
      type: 'gotify',
      url: 'https://gotify.example.com/message',
      has_token: true,
    }

    setupMocks([gotifyProviderWithToken])

    const user = userEvent.setup()
    renderWithQueryClient(<Notifications />)

    const row = await screen.findByTestId('provider-row-provider-gotify-has-token')
    const buttons = within(row).getAllByRole('button')
    await user.click(buttons[1])

    expect(screen.getByTestId('gotify-token-stored-indicator')).toHaveTextContent('notificationProviders.gotifyTokenStored')
    const tokenInput = screen.getByTestId('provider-gotify-token') as HTMLInputElement
    expect(tokenInput.placeholder).toBe('notificationProviders.gotifyTokenKeepPlaceholder')
  })

  it('hides token-stored indicator when has_token is false', async () => {
    const gotifyProviderNoToken: NotificationProvider = {
      ...baseProvider,
      id: 'provider-gotify-no-token',
      type: 'gotify',
      url: 'https://gotify.example.com/message',
      has_token: false,
    }

    setupMocks([gotifyProviderNoToken])

    const user = userEvent.setup()
    renderWithQueryClient(<Notifications />)

    const row = await screen.findByTestId('provider-row-provider-gotify-no-token')
    const buttons = within(row).getAllByRole('button')
    await user.click(buttons[1])

    expect(screen.queryByTestId('gotify-token-stored-indicator')).toBeNull()
    const tokenInput = screen.getByTestId('provider-gotify-token') as HTMLInputElement
    expect(tokenInput.placeholder).toBe('notificationProviders.gotifyTokenPlaceholder')
  })

  it('shows error toast when test mutation fails', async () => {
    vi.mocked(notificationsApi.testProvider).mockRejectedValue(new Error('Connection refused'))
    setupMocks([baseProvider])

    const user = userEvent.setup()
    renderWithQueryClient(<Notifications />)

    const row = await screen.findByTestId(`provider-row-${baseProvider.id}`)
    await user.click(within(row).getByRole('button', { name: /edit/i }))

    await user.click(screen.getByTestId('provider-test-btn'))

    await waitFor(() => {
      expect(toast.error).toHaveBeenCalledWith('Connection refused')
    })
  })

  it('disables test button when provider is new (unsaved) and not email type', async () => {
    const user = userEvent.setup()
    renderWithQueryClient(<Notifications />)

    await user.click(await screen.findByTestId('add-provider-btn'))
    const testBtn = screen.getByTestId('provider-test-btn')
    expect(testBtn).toBeDisabled()
  })

  it('shows JSON template selector for gotify provider', async () => {
    const user = userEvent.setup()
    renderWithQueryClient(<Notifications />)

    await user.click(await screen.findByTestId('add-provider-btn'))
    await user.selectOptions(screen.getByTestId('provider-type'), 'gotify')

    expect(screen.getByTestId('provider-config')).toBeInTheDocument()
  })

  it('shows JSON template selector for webhook provider', async () => {
    const user = userEvent.setup()
    renderWithQueryClient(<Notifications />)

    await user.click(await screen.findByTestId('add-provider-btn'))
    await user.selectOptions(screen.getByTestId('provider-type'), 'webhook')

    expect(screen.getByTestId('provider-config')).toBeInTheDocument()
  })
})
