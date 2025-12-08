import { describe, it, expect, vi, beforeEach } from 'vitest'
import * as presets from '../presets'
import client from '../client'

vi.mock('../client')

describe('crowdsec presets API', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('lists presets via GET', async () => {
    const mockData = { presets: [{ slug: 'bot', title: 'Bot', summary: 'desc', source: 'hub', requires_hub: true, available: true, cached: false }] }
    vi.mocked(client.get).mockResolvedValue({ data: mockData })

    const result = await presets.listCrowdsecPresets()

    expect(client.get).toHaveBeenCalledWith('/admin/crowdsec/presets')
    expect(result).toEqual(mockData)
  })

  it('pulls a preset via POST', async () => {
    const mockData = { status: 'pulled', slug: 'bot', preview: 'configs: {}', cache_key: 'cache-1' }
    vi.mocked(client.post).mockResolvedValue({ data: mockData })

    const result = await presets.pullCrowdsecPreset('bot')

    expect(client.post).toHaveBeenCalledWith('/admin/crowdsec/presets/pull', { slug: 'bot' })
    expect(result).toEqual(mockData)
  })

  it('applies a preset via POST', async () => {
    const mockData = { status: 'applied', backup: '/tmp/backup', cache_key: 'cache-1' }
    vi.mocked(client.post).mockResolvedValue({ data: mockData })

    const payload = { slug: 'bot', cache_key: 'cache-1' }
    const result = await presets.applyCrowdsecPreset(payload)

    expect(client.post).toHaveBeenCalledWith('/admin/crowdsec/presets/apply', payload)
    expect(result).toEqual(mockData)
  })

  it('fetches cached preview by slug', async () => {
    const mockData = { preview: 'cached', cache_key: 'cache-1', etag: 'etag-1' }
    vi.mocked(client.get).mockResolvedValue({ data: mockData })

    const result = await presets.getCrowdsecPresetCache('bot/collection')

    expect(client.get).toHaveBeenCalledWith('/admin/crowdsec/presets/cache/bot%2Fcollection')
    expect(result).toEqual(mockData)
  })

  it('exports default bundle', () => {
    expect(presets.default).toHaveProperty('listCrowdsecPresets')
    expect(presets.default).toHaveProperty('pullCrowdsecPreset')
    expect(presets.default).toHaveProperty('applyCrowdsecPreset')
    expect(presets.default).toHaveProperty('getCrowdsecPresetCache')
  })
})
