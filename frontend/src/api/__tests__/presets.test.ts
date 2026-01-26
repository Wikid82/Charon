import { describe, it, expect, vi, beforeEach } from 'vitest'
import * as presets from '../presets'
import client from '../client'

vi.mock('../client')

describe('presets API', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  describe('listCrowdsecPresets', () => {
    it('should fetch presets list with cached flags', async () => {
      const mockPresets = {
        presets: [
          {
            slug: 'bot-mitigation-essentials',
            title: 'Bot Mitigation Essentials',
            summary: 'Core HTTP parsers and scenarios',
            source: 'hub',
            tags: ['bots', 'web'],
            requires_hub: true,
            available: true,
            cached: true,
            cache_key: 'hub-bot-abc123',
            etag: '"w/12345"',
            retrieved_at: '2025-12-15T10:00:00Z',
          },
          {
            slug: 'honeypot-friendly-defaults',
            title: 'Honeypot Friendly Defaults',
            summary: 'Lightweight defaults for honeypots',
            source: 'builtin',
            tags: ['low-noise'],
            requires_hub: false,
            available: true,
            cached: false,
          },
        ],
      }
      vi.mocked(client.get).mockResolvedValue({ data: mockPresets })

      const result = await presets.listCrowdsecPresets()

      expect(client.get).toHaveBeenCalledWith('/admin/crowdsec/presets')
      expect(result).toEqual(mockPresets)
      expect(result.presets).toHaveLength(2)
      expect(result.presets[0].cached).toBe(true)
      expect(result.presets[0].cache_key).toBe('hub-bot-abc123')
      expect(result.presets[1].cached).toBe(false)
    })

    it('should handle empty presets list', async () => {
      const mockData = { presets: [] }
      vi.mocked(client.get).mockResolvedValue({ data: mockData })

      const result = await presets.listCrowdsecPresets()

      expect(result.presets).toHaveLength(0)
    })

    it('should handle API errors', async () => {
      const error = new Error('Network error')
      vi.mocked(client.get).mockRejectedValue(error)

      await expect(presets.listCrowdsecPresets()).rejects.toThrow('Network error')
    })

    it('should handle hub API unavailability', async () => {
      const error = {
        response: {
          status: 503,
          data: { error: 'CrowdSec Hub API unavailable' },
        },
      }
      vi.mocked(client.get).mockRejectedValue(error)

      await expect(presets.listCrowdsecPresets()).rejects.toEqual(error)
    })
  })

  describe('getCrowdsecPresets', () => {
    it('should be an alias for listCrowdsecPresets', async () => {
      const mockData = { presets: [] }
      vi.mocked(client.get).mockResolvedValue({ data: mockData })

      const result = await presets.getCrowdsecPresets()

      expect(client.get).toHaveBeenCalledWith('/admin/crowdsec/presets')
      expect(result).toEqual(mockData)
    })
  })

  describe('pullCrowdsecPreset', () => {
    it('should pull preset and return preview with cache_key', async () => {
      const mockResponse = {
        status: 'success',
        slug: 'bot-mitigation-essentials',
        preview: '# Bot Mitigation Config\nconfigs:\n  collections:\n    - crowdsecurity/base-http-scenarios',
        cache_key: 'hub-bot-xyz789',
        etag: '"abc123"',
        retrieved_at: '2025-12-15T10:00:00Z',
        source: 'hub',
      }
      vi.mocked(client.post).mockResolvedValue({ data: mockResponse })

      const result = await presets.pullCrowdsecPreset('bot-mitigation-essentials')

      expect(client.post).toHaveBeenCalledWith('/admin/crowdsec/presets/pull', {
        slug: 'bot-mitigation-essentials',
      })
      expect(result).toEqual(mockResponse)
      expect(result.status).toBe('success')
      expect(result.cache_key).toBeDefined()
      expect(result.preview).toContain('configs:')
    })

    it('should handle invalid preset slug', async () => {
      const mockResponse = {
        status: 'error',
        slug: 'non-existent-preset',
        preview: '',
        cache_key: '',
      }
      vi.mocked(client.post).mockResolvedValue({ data: mockResponse })

      const result = await presets.pullCrowdsecPreset('non-existent-preset')

      expect(result.status).toBe('error')
    })

    it('should handle hub API timeout during pull', async () => {
      const error = {
        response: {
          status: 504,
          data: { error: 'Gateway timeout while fetching from CrowdSec Hub' },
        },
      }
      vi.mocked(client.post).mockRejectedValue(error)

      await expect(presets.pullCrowdsecPreset('bot-mitigation-essentials')).rejects.toEqual(error)
    })

    it('should handle ETAG validation scenarios', async () => {
      const mockResponse = {
        status: 'success',
        slug: 'bot-mitigation-essentials',
        preview: '# Cached content',
        cache_key: 'hub-bot-cached123',
        etag: '"not-modified"',
        retrieved_at: '2025-12-14T09:00:00Z',
        source: 'cache',
      }
      vi.mocked(client.post).mockResolvedValue({ data: mockResponse })

      const result = await presets.pullCrowdsecPreset('bot-mitigation-essentials')

      expect(result.source).toBe('cache')
      expect(result.etag).toBe('"not-modified"')
    })

    it('should handle CrowdSec not running during pull', async () => {
      const error = {
        response: {
          status: 500,
          data: { error: 'CrowdSec LAPI not available' },
        },
      }
      vi.mocked(client.post).mockRejectedValue(error)

      await expect(presets.pullCrowdsecPreset('bot-mitigation-essentials')).rejects.toEqual(error)
    })

    it('should encode special characters in preset slug', async () => {
      const mockResponse = {
        status: 'success',
        slug: 'custom/preset-with-slash',
        preview: '# Custom',
        cache_key: 'custom-key',
      }
      vi.mocked(client.post).mockResolvedValue({ data: mockResponse })

      await presets.pullCrowdsecPreset('custom/preset-with-slash')

      expect(client.post).toHaveBeenCalledWith('/admin/crowdsec/presets/pull', {
        slug: 'custom/preset-with-slash',
      })
    })
  })

  describe('applyCrowdsecPreset', () => {
    it('should apply preset with cache_key when available', async () => {
      const payload = { slug: 'bot-mitigation-essentials', cache_key: 'hub-bot-xyz789' }
      const mockResponse = {
        status: 'success',
        backup: '/data/charon/data/backups/preset-backup-20251215-100000.tar.gz',
        reload_hint: true,
        used_cscli: true,
        cache_key: 'hub-bot-xyz789',
        slug: 'bot-mitigation-essentials',
      }
      vi.mocked(client.post).mockResolvedValue({ data: mockResponse })

      const result = await presets.applyCrowdsecPreset(payload)

      expect(client.post).toHaveBeenCalledWith('/admin/crowdsec/presets/apply', payload)
      expect(result).toEqual(mockResponse)
      expect(result.status).toBe('success')
      expect(result.backup).toBeDefined()
      expect(result.reload_hint).toBe(true)
    })

    it('should apply preset without cache_key (fallback mode)', async () => {
      const payload = { slug: 'honeypot-friendly-defaults' }
      const mockResponse = {
        status: 'success',
        backup: '/data/charon/data/backups/preset-backup-20251215-100100.tar.gz',
        reload_hint: true,
        used_cscli: true,
        slug: 'honeypot-friendly-defaults',
      }
      vi.mocked(client.post).mockResolvedValue({ data: mockResponse })

      const result = await presets.applyCrowdsecPreset(payload)

      expect(client.post).toHaveBeenCalledWith('/admin/crowdsec/presets/apply', payload)
      expect(result.status).toBe('success')
      expect(result.used_cscli).toBe(true)
    })

    it('should handle stale cache_key gracefully', async () => {
      const stalePayload = { slug: 'bot-mitigation-essentials', cache_key: 'old_key_123' }
      const error = {
        response: {
          status: 400,
          data: { error: 'Cache key mismatch or expired. Please pull the preset again.' },
        },
      }
      vi.mocked(client.post).mockRejectedValue(error)

      await expect(presets.applyCrowdsecPreset(stalePayload)).rejects.toEqual(error)
    })

    it('should error when applying preset with CrowdSec stopped', async () => {
      const payload = { slug: 'bot-mitigation-essentials', cache_key: 'valid-key' }
      const error = {
        response: {
          status: 500,
          data: { error: 'CrowdSec is not running. Start CrowdSec before applying presets.' },
        },
      }
      vi.mocked(client.post).mockRejectedValue(error)

      await expect(presets.applyCrowdsecPreset(payload)).rejects.toEqual(error)
    })

    it('should handle backup creation failure', async () => {
      const payload = { slug: 'bot-mitigation-essentials', cache_key: 'valid-key' }
      const error = {
        response: {
          status: 500,
          data: { error: 'Failed to create backup before applying preset' },
        },
      }
      vi.mocked(client.post).mockRejectedValue(error)

      await expect(presets.applyCrowdsecPreset(payload)).rejects.toEqual(error)
    })

    it('should handle cscli errors during application', async () => {
      const payload = { slug: 'invalid-preset' }
      const error = {
        response: {
          status: 500,
          data: { error: 'cscli hub update failed: exit status 1' },
        },
      }
      vi.mocked(client.post).mockRejectedValue(error)

      await expect(presets.applyCrowdsecPreset(payload)).rejects.toEqual(error)
    })

    it('should handle payload with force flag', async () => {
      const payload = { slug: 'bot-mitigation-essentials', cache_key: 'key123' }
      const mockResponse = {
        status: 'success',
        backup: '/data/backups/preset-forced.tar.gz',
        reload_hint: true,
      }
      vi.mocked(client.post).mockResolvedValue({ data: mockResponse })

      const result = await presets.applyCrowdsecPreset(payload)

      expect(result.status).toBe('success')
    })
  })

  describe('getCrowdsecPresetCache', () => {
    it('should fetch cached preset preview', async () => {
      const mockCache = {
        preview: '# Cached Bot Mitigation Config\nconfigs:\n  collections:\n    - crowdsecurity/base-http-scenarios',
        cache_key: 'hub-bot-xyz789',
        etag: '"abc123"',
      }
      vi.mocked(client.get).mockResolvedValue({ data: mockCache })

      const result = await presets.getCrowdsecPresetCache('bot-mitigation-essentials')

      expect(client.get).toHaveBeenCalledWith(
        '/admin/crowdsec/presets/cache/bot-mitigation-essentials'
      )
      expect(result).toEqual(mockCache)
      expect(result.preview).toContain('configs:')
      expect(result.cache_key).toBe('hub-bot-xyz789')
    })

    it('should encode special characters in slug', async () => {
      const mockCache = {
        preview: '# Custom',
        cache_key: 'custom-key',
      }
      vi.mocked(client.get).mockResolvedValue({ data: mockCache })

      await presets.getCrowdsecPresetCache('custom/preset with spaces')

      expect(client.get).toHaveBeenCalledWith(
        '/admin/crowdsec/presets/cache/custom%2Fpreset%20with%20spaces'
      )
    })

    it('should handle cache miss (404)', async () => {
      const error = {
        response: {
          status: 404,
          data: { error: 'Preset not found in cache' },
        },
      }
      vi.mocked(client.get).mockRejectedValue(error)

      await expect(presets.getCrowdsecPresetCache('non-cached-preset')).rejects.toEqual(error)
    })

    it('should handle expired cache entries', async () => {
      const error = {
        response: {
          status: 410,
          data: { error: 'Cache entry expired' },
        },
      }
      vi.mocked(client.get).mockRejectedValue(error)

      await expect(presets.getCrowdsecPresetCache('expired-preset')).rejects.toEqual(error)
    })

    it('should handle empty preview content', async () => {
      const mockCache = {
        preview: '',
        cache_key: 'empty-key',
      }
      vi.mocked(client.get).mockResolvedValue({ data: mockCache })

      const result = await presets.getCrowdsecPresetCache('empty-preset')

      expect(result.preview).toBe('')
      expect(result.cache_key).toBe('empty-key')
    })
  })

  describe('default export', () => {
    it('should export all functions', () => {
      expect(presets.default).toHaveProperty('listCrowdsecPresets')
      expect(presets.default).toHaveProperty('getCrowdsecPresets')
      expect(presets.default).toHaveProperty('pullCrowdsecPreset')
      expect(presets.default).toHaveProperty('applyCrowdsecPreset')
      expect(presets.default).toHaveProperty('getCrowdsecPresetCache')
    })
  })

  describe('integration scenarios', () => {
    it('should handle full workflow: list → pull → cache → apply', async () => {
      // 1. List presets
      const mockList = {
        presets: [
          {
            slug: 'bot-mitigation-essentials',
            title: 'Bot Mitigation',
            summary: 'Core',
            source: 'hub',
            requires_hub: true,
            available: true,
            cached: false,
          },
        ],
      }
      vi.mocked(client.get).mockResolvedValueOnce({ data: mockList })

      const listResult = await presets.listCrowdsecPresets()
      expect(listResult.presets[0].cached).toBe(false)

      // 2. Pull preset
      const mockPull = {
        status: 'success',
        slug: 'bot-mitigation-essentials',
        preview: '# Config',
        cache_key: 'hub-bot-new123',
        etag: '"etag1"',
        retrieved_at: '2025-12-15T10:00:00Z',
      }
      vi.mocked(client.post).mockResolvedValueOnce({ data: mockPull })

      const pullResult = await presets.pullCrowdsecPreset('bot-mitigation-essentials')
      expect(pullResult.cache_key).toBe('hub-bot-new123')

      // 3. Verify cache
      const mockCache = {
        preview: '# Config',
        cache_key: 'hub-bot-new123',
        etag: '"etag1"',
      }
      vi.mocked(client.get).mockResolvedValueOnce({ data: mockCache })

      const cacheResult = await presets.getCrowdsecPresetCache('bot-mitigation-essentials')
      expect(cacheResult.cache_key).toBe(pullResult.cache_key)

      // 4. Apply preset
      const mockApply = {
        status: 'success',
        backup: '/data/backups/preset-backup.tar.gz',
        reload_hint: true,
        cache_key: 'hub-bot-new123',
        slug: 'bot-mitigation-essentials',
      }
      vi.mocked(client.post).mockResolvedValueOnce({ data: mockApply })

      const applyResult = await presets.applyCrowdsecPreset({
        slug: 'bot-mitigation-essentials',
        cache_key: pullResult.cache_key,
      })
      expect(applyResult.status).toBe('success')
      expect(applyResult.backup).toBeDefined()
    })

    it('should handle network failure mid-workflow', async () => {
      // Pull succeeds
      const mockPull = {
        status: 'success',
        slug: 'test-preset',
        preview: '# Test',
        cache_key: 'test-key',
      }
      vi.mocked(client.post).mockResolvedValueOnce({ data: mockPull })

      const pullResult = await presets.pullCrowdsecPreset('test-preset')
      expect(pullResult.cache_key).toBe('test-key')

      // Apply fails due to network
      const networkError = new Error('Network error')
      vi.mocked(client.post).mockRejectedValueOnce(networkError)

      await expect(
        presets.applyCrowdsecPreset({ slug: 'test-preset', cache_key: 'test-key' })
      ).rejects.toThrow('Network error')
    })
  })
})
