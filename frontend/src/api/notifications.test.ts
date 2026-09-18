import { describe, it, expect, vi, beforeEach } from 'vitest'

import client from './client'
import {
  getProviders,
  createProvider,
  updateProvider,
  deleteProvider,
  testProvider,
  getTemplates,
  previewProvider,
  getExternalTemplates,
  createExternalTemplate,
  updateExternalTemplate,
  deleteExternalTemplate,
  previewExternalTemplate,
  getSecurityNotificationSettings,
  updateSecurityNotificationSettings,
  SUPPORTED_NOTIFICATION_PROVIDER_TYPES,
  provisionWebPush,
  getWebPushVapidPublicKey,
  subscribeWebPush,
  listWebPushSubscriptions,
  unsubscribeWebPush,
} from './notifications'

vi.mock('./client', () => ({
  default: {
    get: vi.fn(),
    post: vi.fn(),
    put: vi.fn(),
    delete: vi.fn(),
  },
}))

const mockedClient = client as unknown as {
  get: ReturnType<typeof vi.fn>
  post: ReturnType<typeof vi.fn>
  put: ReturnType<typeof vi.fn>
  delete: ReturnType<typeof vi.fn>
}

describe('notifications api', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('fetches providers list', async () => {
    mockedClient.get.mockResolvedValue({
      data: [
        {
          id: '1',
          name: 'PagerDuty',
          type: 'webhook',
          url: 'https://hooks.example.com',
          enabled: true,
          notify_proxy_hosts: true,
          notify_remote_servers: false,
          notify_domains: false,
          notify_certs: false,
          notify_uptime: true,
          created_at: '2025-01-01T00:00:00Z',
        },
      ],
    })

    const result = await getProviders()

    expect(mockedClient.get).toHaveBeenCalledWith('/notifications/providers')
    expect(result[0].name).toBe('PagerDuty')
  })

  it('creates, updates, tests, and deletes a provider', async () => {
    mockedClient.post.mockResolvedValue({ data: { id: 'new', name: 'Slack' } })
    mockedClient.put.mockResolvedValue({ data: { id: 'new', name: 'Slack v2' } })

    const created = await createProvider({ name: 'Slack' })
    expect(mockedClient.post).toHaveBeenCalledWith('/notifications/providers', { name: 'Slack', type: 'discord' })
    expect(created.id).toBe('new')

    const updated = await updateProvider('new', { enabled: false })
    expect(mockedClient.put).toHaveBeenCalledWith('/notifications/providers/new', { enabled: false, type: 'discord' })
    expect(updated.name).toBe('Slack v2')

    await testProvider({ id: 'new', name: 'Slack', enabled: true })
    expect(mockedClient.post).toHaveBeenCalledWith('/notifications/providers/test', {
      id: 'new',
      name: 'Slack',
      enabled: true,
      type: 'discord',
    })

    mockedClient.delete.mockResolvedValue({})
    await deleteProvider('new')
    expect(mockedClient.delete).toHaveBeenCalledWith('/notifications/providers/new')
  })

  it('supports discord, gotify, and webhook while enforcing token payload contract', async () => {
    mockedClient.post.mockResolvedValue({ data: { id: 'ok' } })
    mockedClient.put.mockResolvedValue({ data: { id: 'ok' } })

    await createProvider({ name: 'Gotify', type: 'gotify', gotify_token: 'secret-token' })
    expect(mockedClient.post).toHaveBeenCalledWith('/notifications/providers', {
      name: 'Gotify',
      type: 'gotify',
      token: 'secret-token',
    })

    await updateProvider('ok', { type: 'webhook', url: 'https://example.com/webhook', gotify_token: 'should-not-send' })
    expect(mockedClient.put).toHaveBeenCalledWith('/notifications/providers/ok', {
      type: 'webhook',
      url: 'https://example.com/webhook',
    })

    await testProvider({ id: 'ok', type: 'gotify', gotify_token: 'should-not-send' })
    expect(mockedClient.post).toHaveBeenCalledWith('/notifications/providers/test', {
      id: 'ok',
      type: 'gotify',
    })

    await previewProvider({ id: 'ok', type: 'gotify', gotify_token: 'should-not-send' })
    expect(mockedClient.post).toHaveBeenCalledWith('/notifications/providers/preview', {
      id: 'ok',
      type: 'gotify',
    })

    await expect(createProvider({ name: 'Bad', type: 'sms' })).rejects.toThrow('Unsupported notification provider type: sms')
    await expect(updateProvider('bad', { type: 'generic' })).rejects.toThrow('Unsupported notification provider type: generic')
  })

  it('supports telegram provider with token payload contract', async () => {
    mockedClient.post.mockResolvedValue({ data: { id: 'tg1' } })
    mockedClient.put.mockResolvedValue({ data: { id: 'tg1' } })

    await createProvider({ name: 'Telegram', type: 'telegram', gotify_token: 'bot123:ABC' })
    expect(mockedClient.post).toHaveBeenCalledWith('/notifications/providers', {
      name: 'Telegram',
      type: 'telegram',
      token: 'bot123:ABC',
    })

    await updateProvider('tg1', { type: 'telegram', url: '987654321', gotify_token: 'newtoken' })
    expect(mockedClient.put).toHaveBeenCalledWith('/notifications/providers/tg1', {
      type: 'telegram',
      url: '987654321',
      token: 'newtoken',
    })

    await updateProvider('tg1', { type: 'telegram', url: '987654321' })
    expect(mockedClient.put).toHaveBeenCalledWith('/notifications/providers/tg1', {
      type: 'telegram',
      url: '987654321',
    })
  })

  it('telegram preserves token in sanitization and strips gotify_token key', async () => {
    mockedClient.post.mockResolvedValue({ data: { id: 'tg2' } })

    await createProvider({ name: 'TG', type: 'telegram', token: 'direct-token', gotify_token: '' })
    expect(mockedClient.post).toHaveBeenCalledWith('/notifications/providers', {
      name: 'TG',
      type: 'telegram',
      token: 'direct-token',
    })
  })

  it('telegram test/preview strips token from read-like actions', async () => {
    mockedClient.post.mockResolvedValue({ data: { id: 'tg3' } })

    await testProvider({ id: 'tg3', type: 'telegram', gotify_token: 'should-not-send' })
    expect(mockedClient.post).toHaveBeenCalledWith('/notifications/providers/test', {
      id: 'tg3',
      type: 'telegram',
    })
  })

  it('fetches templates and previews provider payloads with data', async () => {
    mockedClient.get.mockResolvedValueOnce({ data: [{ id: 'tpl', name: 'default' }] })
    mockedClient.post.mockResolvedValue({ data: { preview: 'ok' } })

    const templates = await getTemplates()
    expect(mockedClient.get).toHaveBeenCalledWith('/notifications/templates')
    expect(templates[0].id).toBe('tpl')

    const preview = await previewProvider({ id: 'p1', name: 'Provider' }, { foo: 'bar' })
    expect(mockedClient.post).toHaveBeenCalledWith('/notifications/providers/preview', {
      id: 'p1',
      name: 'Provider',
      type: 'discord',
      data: { foo: 'bar' },
    })
    expect(preview).toEqual({ preview: 'ok' })
  })

  it('handles external templates lifecycle and previews', async () => {
    mockedClient.get.mockResolvedValueOnce({ data: [{ id: 'ext', name: 'External' }] })
    mockedClient.post.mockResolvedValueOnce({ data: { id: 'ext', name: 'created' } })
    mockedClient.put.mockResolvedValueOnce({ data: { id: 'ext', name: 'updated' } })
    mockedClient.post.mockResolvedValueOnce({ data: { preview: 'rendered' } })

    const list = await getExternalTemplates()
    expect(mockedClient.get).toHaveBeenCalledWith('/notifications/external-templates')
    expect(list[0].id).toBe('ext')

    const created = await createExternalTemplate({ name: 'External' })
    expect(mockedClient.post).toHaveBeenCalledWith('/notifications/external-templates', { name: 'External' })
    expect(created.name).toBe('created')

    const updated = await updateExternalTemplate('ext', { description: 'desc' })
    expect(mockedClient.put).toHaveBeenCalledWith('/notifications/external-templates/ext', { description: 'desc' })
    expect(updated.name).toBe('updated')

    await deleteExternalTemplate('ext')
    expect(mockedClient.delete).toHaveBeenCalledWith('/notifications/external-templates/ext')

    const preview = await previewExternalTemplate('ext', '<tpl>', { a: 1 })
    expect(mockedClient.post).toHaveBeenCalledWith('/notifications/external-templates/preview', {
      template_id: 'ext',
      template: '<tpl>',
      data: { a: 1 },
    })
    expect(preview).toEqual({ preview: 'rendered' })
  })

  it('reads and updates security notification settings', async () => {
    mockedClient.get.mockResolvedValueOnce({ data: { enabled: true, min_log_level: 'info', security_waf_enabled: true, security_acl_enabled: false, security_rate_limit_enabled: true } })
    mockedClient.put.mockResolvedValueOnce({ data: { enabled: false, min_log_level: 'error', security_waf_enabled: false, security_acl_enabled: true, security_rate_limit_enabled: false } })

    const settings = await getSecurityNotificationSettings()
    expect(settings.enabled).toBe(true)
    expect(mockedClient.get).toHaveBeenCalledWith('/notifications/settings/security')

    const updated = await updateSecurityNotificationSettings({ enabled: false, min_log_level: 'error' })
    expect(mockedClient.put).toHaveBeenCalledWith('/notifications/settings/security', { enabled: false, min_log_level: 'error' })
    expect(updated.enabled).toBe(false)
  })

  it('pushover is in SUPPORTED_NOTIFICATION_PROVIDER_TYPES', () => {
    expect(SUPPORTED_NOTIFICATION_PROVIDER_TYPES).toContain('pushover')
  })

  it('sanitizeProviderForWriteAction preserves token for pushover type', async () => {
    mockedClient.post.mockResolvedValue({ data: { id: 'po1' } })
    mockedClient.put.mockResolvedValue({ data: { id: 'po1' } })

    await createProvider({ name: 'Pushover', type: 'pushover', gotify_token: 'app-api-token', url: 'uQiRzpo4DXghDmr9QzzfQu27cmVRsG' })
    expect(mockedClient.post).toHaveBeenCalledWith('/notifications/providers', {
      name: 'Pushover',
      type: 'pushover',
      token: 'app-api-token',
      url: 'uQiRzpo4DXghDmr9QzzfQu27cmVRsG',
    })

    await updateProvider('po1', { type: 'pushover', url: 'uQiRzpo4DXghDmr9QzzfQu27cmVRsG', gotify_token: 'new-token' })
    expect(mockedClient.put).toHaveBeenCalledWith('/notifications/providers/po1', {
      type: 'pushover',
      url: 'uQiRzpo4DXghDmr9QzzfQu27cmVRsG',
      token: 'new-token',
    })
  })

  it('webpush is in SUPPORTED_NOTIFICATION_PROVIDER_TYPES', () => {
    expect(SUPPORTED_NOTIFICATION_PROVIDER_TYPES).toContain('webpush')
  })

  it('provisions the Web Push provider', async () => {
    mockedClient.post.mockResolvedValue({ data: { id: 'wp1', name: 'Web Push', type: 'webpush', enabled: true, has_token: true } })

    const provider = await provisionWebPush({ name: 'Web Push', vapid_subject: 'mailto:admin@example.com' })

    expect(mockedClient.post).toHaveBeenCalledWith('/notifications/providers/webpush/provision', {
      name: 'Web Push',
      vapid_subject: 'mailto:admin@example.com',
    })
    expect(provider.id).toBe('wp1')
  })

  it('fetches the VAPID public key', async () => {
    mockedClient.get.mockResolvedValue({ data: { vapid_public_key: 'abc123' } })

    const result = await getWebPushVapidPublicKey()

    expect(mockedClient.get).toHaveBeenCalledWith('/notifications/providers/webpush/vapid-public-key')
    expect(result.vapid_public_key).toBe('abc123')
  })

  it('subscribes a device for Web Push', async () => {
    mockedClient.post.mockResolvedValue({ data: { id: 'sub1', endpoint: 'https://push.example.com/abc' } })

    const payload = {
      endpoint: 'https://push.example.com/abc',
      keys: { p256dh: 'p256dh-key', auth: 'auth-key' },
      user_agent: 'test-agent',
    }
    const result = await subscribeWebPush(payload)

    expect(mockedClient.post).toHaveBeenCalledWith('/notifications/providers/webpush/subscriptions', payload)
    expect(result).toEqual({ id: 'sub1', endpoint: 'https://push.example.com/abc' })
  })

  it('lists the caller\'s Web Push subscriptions', async () => {
    mockedClient.get.mockResolvedValue({
      data: [{ id: 'sub1', endpoint: 'https://push.example.com/abc', created_at: '2024-01-01T00:00:00Z', last_seen_at: '2024-01-02T00:00:00Z' }],
    })

    const result = await listWebPushSubscriptions()

    expect(mockedClient.get).toHaveBeenCalledWith('/notifications/providers/webpush/subscriptions')
    expect(result).toHaveLength(1)
    expect(result[0].id).toBe('sub1')
  })

  it('unsubscribes a Web Push subscription', async () => {
    mockedClient.delete.mockResolvedValue({})

    await unsubscribeWebPush('sub1')

    expect(mockedClient.delete).toHaveBeenCalledWith('/notifications/providers/webpush/subscriptions/sub1')
  })
})
