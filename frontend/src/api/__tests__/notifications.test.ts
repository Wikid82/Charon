import { describe, it, expect, vi, beforeEach } from 'vitest'

import client from '../client'
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
} from '../notifications'

vi.mock('../client', () => ({
  default: {
    get: vi.fn(),
    post: vi.fn(),
    put: vi.fn(),
    delete: vi.fn(),
  },
}))

describe('notifications api', () => {
  beforeEach(() => {
    vi.resetAllMocks()
  })

  it('crud for providers uses correct endpoints', async () => {
    vi.mocked(client.get).mockResolvedValue({ data: [{ id: '1', name: 'discord', type: 'discord', url: 'http://', enabled: true } as never] })
    vi.mocked(client.post).mockResolvedValue({ data: { id: '2' } })
    vi.mocked(client.put).mockResolvedValue({ data: { id: '2', name: 'updated' } })

    const providers = await getProviders()
    expect(providers[0].id).toBe('1')
    expect(client.get).toHaveBeenCalledWith('/notifications/providers')

    await createProvider({ name: 'x', type: 'discord' })
    expect(client.post).toHaveBeenCalledWith('/notifications/providers', { name: 'x', type: 'discord' })

    await updateProvider('2', { name: 'updated', type: 'discord' })
    expect(client.put).toHaveBeenCalledWith('/notifications/providers/2', { name: 'updated', type: 'discord' })

    await deleteProvider('2')
    expect(client.delete).toHaveBeenCalledWith('/notifications/providers/2')

    await testProvider({ id: '2', name: 'test', type: 'discord' })
    expect(client.post).toHaveBeenCalledWith('/notifications/providers/test', { id: '2', name: 'test', type: 'discord' })

    await expect(createProvider({ name: 'x', type: 'pagerduty' })).rejects.toThrow('Unsupported notification provider type: pagerduty')
    await expect(updateProvider('2', { name: 'updated', type: 'generic' })).rejects.toThrow('Unsupported notification provider type: generic')
    await testProvider({ id: '2', name: 'test', type: 'telegram' })
    expect(client.post).toHaveBeenCalledWith('/notifications/providers/test', { id: '2', name: 'test', type: 'telegram' })
  })

  it('templates and previews use merged payloads', async () => {
    vi.mocked(client.get).mockResolvedValueOnce({ data: [{ id: 't1', name: 'default' }] })
    const templates = await getTemplates()
    expect(templates[0].name).toBe('default')
    expect(client.get).toHaveBeenCalledWith('/notifications/templates')

    vi.mocked(client.post).mockResolvedValueOnce({ data: { preview: 'ok' } })
    const preview = await previewProvider({ name: 'provider', type: 'discord' }, { user: 'alice' })
    expect(preview).toEqual({ preview: 'ok' })
    expect(client.post).toHaveBeenCalledWith('/notifications/providers/preview', { name: 'provider', type: 'discord', data: { user: 'alice' } })

    vi.mocked(client.post).mockResolvedValueOnce({ data: { preview: 'webhook-ok' } })
    const webhookPreview = await previewProvider({ name: 'provider', type: 'webhook' }, { user: 'alice' })
    expect(webhookPreview).toEqual({ preview: 'webhook-ok' })
    expect(client.post).toHaveBeenCalledWith('/notifications/providers/preview', { name: 'provider', type: 'webhook', data: { user: 'alice' } })
  })

  it('external template endpoints shape payloads', async () => {
    vi.mocked(client.get).mockResolvedValueOnce({ data: [{ id: 'ext', name: 'External' }] })
    const external = await getExternalTemplates()
    expect(external[0].id).toBe('ext')
    expect(client.get).toHaveBeenCalledWith('/notifications/external-templates')

    vi.mocked(client.post).mockResolvedValueOnce({ data: { id: 'ext2' } })
    await createExternalTemplate({ name: 'n' })
    expect(client.post).toHaveBeenCalledWith('/notifications/external-templates', { name: 'n' })

    vi.mocked(client.put).mockResolvedValueOnce({ data: { id: 'ext', name: 'updated' } })
    await updateExternalTemplate('ext', { name: 'updated' })
    expect(client.put).toHaveBeenCalledWith('/notifications/external-templates/ext', { name: 'updated' })

    await deleteExternalTemplate('ext')
    expect(client.delete).toHaveBeenCalledWith('/notifications/external-templates/ext')

    vi.mocked(client.post).mockResolvedValueOnce({ data: { id: 'ext2' } })
    const result = await previewExternalTemplate('ext', 'tpl', { id: 1 })
    expect(result).toEqual({ id: 'ext2' })
    expect(client.post).toHaveBeenCalledWith('/notifications/external-templates/preview', { template_id: 'ext', template: 'tpl', data: { id: 1 } })
  })

  it('reads and updates security notification settings', async () => {
    vi.mocked(client.get).mockResolvedValueOnce({ data: { enabled: true, min_log_level: 'info', security_waf_enabled: true, security_acl_enabled: false, security_rate_limit_enabled: true } })
    const settings = await getSecurityNotificationSettings()
    expect(settings.enabled).toBe(true)
    expect(client.get).toHaveBeenCalledWith('/notifications/settings/security')

    vi.mocked(client.put).mockResolvedValueOnce({ data: { enabled: false } })
    const updated = await updateSecurityNotificationSettings({ enabled: false })
    expect(updated.enabled).toBe(false)
    expect(client.put).toHaveBeenCalledWith('/notifications/settings/security', { enabled: false })
  })
})
