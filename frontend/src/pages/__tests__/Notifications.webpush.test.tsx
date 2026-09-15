import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, it, expect, vi, beforeEach } from 'vitest'

import * as notificationsApi from '../../api/notifications'
import { renderWithQueryClient } from '../../test-utils/renderWithQueryClient'
import { toast } from '../../utils/toast'
import Notifications from '../Notifications'

import type { NotificationProvider, WebPushSubscription } from '../../api/notifications'

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
  toast: {
    success: vi.fn(),
    error: vi.fn(),
  },
}))

const mockUseAuth = vi.fn()
vi.mock('../../hooks/useAuth', () => ({
  useAuth: () => mockUseAuth(),
}))

const notProvisionedError = Object.assign(new Error('Not Found'), {
  isAxiosError: true,
  response: { status: 404, data: { error: 'not provisioned' } },
})

const baseSubscription: WebPushSubscription = {
  id: 'sub-1',
  endpoint: 'https://push.example.com/abc',
  user_agent: 'Mozilla/5.0 Test Browser',
  created_at: '2026-01-01T00:00:00Z',
  last_seen_at: '2026-01-02T00:00:00Z',
}

const mockPushManager = {
  subscribe: vi.fn(),
  getSubscription: vi.fn(),
}

const mockRegistration = {
  pushManager: mockPushManager,
}

const mockServiceWorker = {
  register: vi.fn().mockResolvedValue(mockRegistration),
  getRegistration: vi.fn().mockResolvedValue(mockRegistration),
}

const setSupportsWebPush = (supported: boolean) => {
  if (supported) {
    vi.stubGlobal('PushManager', function PushManager() {})
    Object.defineProperty(navigator, 'serviceWorker', {
      value: mockServiceWorker,
      configurable: true,
      writable: true,
    })
  } else {
    vi.unstubAllGlobals()
    Object.defineProperty(navigator, 'serviceWorker', {
      value: undefined,
      configurable: true,
      writable: true,
    })
  }
}

const setupMocks = (options: { providers?: NotificationProvider[]; role?: 'admin' | 'user'; subscriptions?: WebPushSubscription[] } = {}) => {
  const { providers = [], role = 'admin', subscriptions = [] } = options
  vi.mocked(notificationsApi.getProviders).mockResolvedValue(providers)
  vi.mocked(notificationsApi.getTemplates).mockResolvedValue([])
  vi.mocked(notificationsApi.getExternalTemplates).mockResolvedValue([])
  vi.mocked(notificationsApi.listWebPushSubscriptions).mockResolvedValue(subscriptions)
  mockUseAuth.mockReturnValue({ user: { id: 'u1', username: 'tester', role } })
}

let user: ReturnType<typeof userEvent.setup>

describe('Notifications - Web Push', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.stubGlobal('Notification', { requestPermission: vi.fn().mockResolvedValue('granted') })
    mockPushManager.subscribe.mockReset()
    mockPushManager.getSubscription.mockReset()
    mockServiceWorker.register.mockClear().mockResolvedValue(mockRegistration)
    mockServiceWorker.getRegistration.mockClear().mockResolvedValue(mockRegistration)
    setupMocks()
    user = userEvent.setup()
  })

  it('shows an unsupported-browser message when serviceWorker/PushManager are unavailable', async () => {
    setSupportsWebPush(false)
    vi.mocked(notificationsApi.getWebPushVapidPublicKey).mockRejectedValue(notProvisionedError)

    renderWithQueryClient(<Notifications />)

    expect(await screen.findByTestId('webpush-unsupported')).toBeInTheDocument()
    expect(notificationsApi.getWebPushVapidPublicKey).not.toHaveBeenCalled()
  })

  it('shows a provision form to admins when Web Push has not been provisioned', async () => {
    setSupportsWebPush(true)
    vi.mocked(notificationsApi.getWebPushVapidPublicKey).mockRejectedValue(notProvisionedError)

    renderWithQueryClient(<Notifications />)

    expect(await screen.findByTestId('webpush-provision-form')).toBeInTheDocument()
    expect(screen.queryByTestId('webpush-not-provisioned-message')).not.toBeInTheDocument()
  })

  it('shows a plain message (no provision form) to non-admins when not provisioned', async () => {
    setSupportsWebPush(true)
    setupMocks({ role: 'user' })
    vi.mocked(notificationsApi.getWebPushVapidPublicKey).mockRejectedValue(notProvisionedError)

    renderWithQueryClient(<Notifications />)

    expect(await screen.findByTestId('webpush-not-provisioned-message')).toBeInTheDocument()
    expect(screen.queryByTestId('webpush-provision-form')).not.toBeInTheDocument()
  })

  it('provisions Web Push when the admin submits the form', async () => {
    setSupportsWebPush(true)
    vi.mocked(notificationsApi.getWebPushVapidPublicKey).mockRejectedValue(notProvisionedError)
    vi.mocked(notificationsApi.provisionWebPush).mockResolvedValue({
      id: 'wp-1',
      name: 'Web Push',
      type: 'webpush',
      url: '',
      enabled: true,
      has_token: true,
      notify_proxy_hosts: true,
      notify_remote_servers: true,
      notify_domains: true,
      notify_certs: true,
      notify_uptime: true,
      notify_security_waf_blocks: false,
      notify_security_acl_denies: false,
      notify_security_rate_limit_hits: false,
      created_at: '2026-01-01T00:00:00Z',
    })

    renderWithQueryClient(<Notifications />)

    await screen.findByTestId('webpush-provision-form')
    await user.clear(screen.getByTestId('webpush-provision-name'))
    await user.type(screen.getByTestId('webpush-provision-name'), 'Team Push')
    await user.clear(screen.getByTestId('webpush-vapid-subject'))
    await user.type(screen.getByTestId('webpush-vapid-subject'), 'mailto:admin@example.com')
    await user.click(screen.getByTestId('webpush-provision-btn'))

    await waitFor(() => {
      expect(notificationsApi.provisionWebPush).toHaveBeenCalled()
    })
    expect(vi.mocked(notificationsApi.provisionWebPush).mock.calls[0][0]).toEqual({ name: 'Team Push', vapid_subject: 'mailto:admin@example.com' })
    expect(toast.success).toHaveBeenCalled()
  })

  it('shows an error toast and keeps the form open when provisioning fails', async () => {
    setSupportsWebPush(true)
    vi.mocked(notificationsApi.getWebPushVapidPublicKey).mockRejectedValue(notProvisionedError)
    vi.mocked(notificationsApi.provisionWebPush).mockRejectedValue(new Error('vapid_subject must be a mailto: or https: URI'))

    renderWithQueryClient(<Notifications />)

    await screen.findByTestId('webpush-provision-form')
    await user.type(screen.getByTestId('webpush-vapid-subject'), 'not-a-valid-uri')
    await user.click(screen.getByTestId('webpush-provision-btn'))

    await waitFor(() => {
      expect(toast.error).toHaveBeenCalledWith('vapid_subject must be a mailto: or https: URI')
    })
    expect(screen.getByTestId('webpush-provision-form')).toBeInTheDocument()
  })

  it('shows the subscribe control and an empty subscriptions list once provisioned', async () => {
    setSupportsWebPush(true)
    vi.mocked(notificationsApi.getWebPushVapidPublicKey).mockResolvedValue({ vapid_public_key: 'test-vapid-key' })

    renderWithQueryClient(<Notifications />)

    expect(await screen.findByTestId('webpush-subscribe-btn')).toBeInTheDocument()
    expect(await screen.findByTestId('webpush-no-subscriptions')).toBeInTheDocument()
  })

  it('subscribes this device when clicking the subscribe button', async () => {
    setSupportsWebPush(true)
    vi.mocked(notificationsApi.getWebPushVapidPublicKey).mockResolvedValue({ vapid_public_key: 'test-vapid-key' })
    mockPushManager.subscribe.mockResolvedValue({
      toJSON: () => ({ endpoint: 'https://push.example.com/new', keys: { p256dh: 'p-key', auth: 'a-key' } }),
    })
    vi.mocked(notificationsApi.subscribeWebPush).mockResolvedValue({ id: 'sub-new', endpoint: 'https://push.example.com/new' })

    renderWithQueryClient(<Notifications />)

    await user.click(await screen.findByTestId('webpush-subscribe-btn'))

    await waitFor(() => {
      expect(notificationsApi.subscribeWebPush).toHaveBeenCalled()
    })
    expect(vi.mocked(notificationsApi.subscribeWebPush).mock.calls[0][0]).toEqual({
      endpoint: 'https://push.example.com/new',
      keys: { p256dh: 'p-key', auth: 'a-key' },
      user_agent: navigator.userAgent,
    })
    expect(mockServiceWorker.register).toHaveBeenCalledWith('/sw.js')
  })

  it('shows an inline error and a toast when the browser subscribe call fails', async () => {
    setSupportsWebPush(true)
    vi.mocked(notificationsApi.getWebPushVapidPublicKey).mockResolvedValue({ vapid_public_key: 'test-vapid-key' })
    mockPushManager.subscribe.mockRejectedValue(new Error('AbortError: subscribe failed'))

    renderWithQueryClient(<Notifications />)

    await user.click(await screen.findByTestId('webpush-subscribe-btn'))

    expect(await screen.findByTestId('webpush-subscribe-error')).toHaveTextContent('AbortError: subscribe failed')
    expect(toast.error).toHaveBeenCalledWith('AbortError: subscribe failed')
    expect(notificationsApi.subscribeWebPush).not.toHaveBeenCalled()
  })

  it('shows an inline message and makes no backend call when permission is denied', async () => {
    setSupportsWebPush(true)
    vi.mocked(notificationsApi.getWebPushVapidPublicKey).mockResolvedValue({ vapid_public_key: 'test-vapid-key' })
    vi.stubGlobal('Notification', { requestPermission: vi.fn().mockResolvedValue('denied') })

    renderWithQueryClient(<Notifications />)

    await user.click(await screen.findByTestId('webpush-subscribe-btn'))

    expect(await screen.findByTestId('webpush-subscribe-error')).toBeInTheDocument()
    expect(notificationsApi.subscribeWebPush).not.toHaveBeenCalled()
  })

  it('lists existing subscriptions and unsubscribes a row', async () => {
    setSupportsWebPush(true)
    vi.mocked(notificationsApi.getWebPushVapidPublicKey).mockResolvedValue({ vapid_public_key: 'test-vapid-key' })
    setupMocks({ subscriptions: [baseSubscription] })
    mockPushManager.getSubscription.mockResolvedValue(null)
    vi.mocked(notificationsApi.unsubscribeWebPush).mockResolvedValue(undefined)

    renderWithQueryClient(<Notifications />)

    const row = await screen.findByTestId('webpush-subscription-row-sub-1')
    expect(within(row).getByText('Mozilla/5.0 Test Browser')).toBeInTheDocument()

    await user.click(within(row).getByTestId('webpush-unsubscribe-sub-1'))

    await waitFor(() => {
      expect(notificationsApi.unsubscribeWebPush).toHaveBeenCalled()
    })
    expect(vi.mocked(notificationsApi.unsubscribeWebPush).mock.calls[0][0]).toBe('sub-1')
  })

  it('unsubscribes browser-side when the row matches this device\'s active subscription', async () => {
    setSupportsWebPush(true)
    vi.mocked(notificationsApi.getWebPushVapidPublicKey).mockResolvedValue({ vapid_public_key: 'test-vapid-key' })
    setupMocks({ subscriptions: [baseSubscription] })
    const activeUnsubscribe = vi.fn().mockResolvedValue(true)
    mockPushManager.getSubscription.mockResolvedValue({ endpoint: baseSubscription.endpoint, unsubscribe: activeUnsubscribe })
    vi.mocked(notificationsApi.unsubscribeWebPush).mockResolvedValue(undefined)

    renderWithQueryClient(<Notifications />)

    const row = await screen.findByTestId('webpush-subscription-row-sub-1')
    await user.click(within(row).getByTestId('webpush-unsubscribe-sub-1'))

    await waitFor(() => {
      expect(activeUnsubscribe).toHaveBeenCalled()
      expect(notificationsApi.unsubscribeWebPush).toHaveBeenCalled()
    })
    expect(vi.mocked(notificationsApi.unsubscribeWebPush).mock.calls[0][0]).toBe('sub-1')
  })

  it('shows a toast when removing a subscription fails', async () => {
    setSupportsWebPush(true)
    vi.mocked(notificationsApi.getWebPushVapidPublicKey).mockResolvedValue({ vapid_public_key: 'test-vapid-key' })
    setupMocks({ subscriptions: [baseSubscription] })
    mockPushManager.getSubscription.mockResolvedValue(null)
    vi.mocked(notificationsApi.unsubscribeWebPush).mockRejectedValue(new Error('Subscription not found'))

    renderWithQueryClient(<Notifications />)

    const row = await screen.findByTestId('webpush-subscription-row-sub-1')
    await user.click(within(row).getByTestId('webpush-unsubscribe-sub-1'))

    await waitFor(() => {
      expect(toast.error).toHaveBeenCalledWith('Subscription not found')
    })
  })

  it('still removes the backend row when browser-side cleanup throws', async () => {
    setSupportsWebPush(true)
    vi.mocked(notificationsApi.getWebPushVapidPublicKey).mockResolvedValue({ vapid_public_key: 'test-vapid-key' })
    setupMocks({ subscriptions: [baseSubscription] })
    mockPushManager.getSubscription.mockRejectedValue(new Error('registration lookup failed'))
    vi.mocked(notificationsApi.unsubscribeWebPush).mockResolvedValue(undefined)

    renderWithQueryClient(<Notifications />)

    const row = await screen.findByTestId('webpush-subscription-row-sub-1')
    await user.click(within(row).getByTestId('webpush-unsubscribe-sub-1'))

    await waitFor(() => {
      expect(notificationsApi.unsubscribeWebPush).toHaveBeenCalled()
    })
    expect(vi.mocked(notificationsApi.unsubscribeWebPush).mock.calls[0][0]).toBe('sub-1')
  })
})
