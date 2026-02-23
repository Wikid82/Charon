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

  it('rejects non-discord type before submit for provider mutations and preview', async () => {
    await expect(createProvider({ name: 'Bad', type: 'slack' })).rejects.toThrow('Only discord notification providers are supported')
    await expect(updateProvider('bad', { type: 'generic' })).rejects.toThrow('Only discord notification providers are supported')
    await expect(testProvider({ id: 'bad', type: 'email' })).rejects.toThrow('Only discord notification providers are supported')
    await expect(previewProvider({ id: 'bad', type: 'gotify' })).rejects.toThrow('Only discord notification providers are supported')

    expect(mockedClient.post).not.toHaveBeenCalled()
    expect(mockedClient.put).not.toHaveBeenCalled()
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
})
