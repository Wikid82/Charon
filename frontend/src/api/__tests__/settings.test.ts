import { describe, it, expect, vi, beforeEach } from 'vitest'

import client from '../client'
import * as settings from '../settings'

vi.mock('../client')

describe('settings API', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  describe('getSettings', () => {
    it('should call GET /settings', async () => {
      const mockData: settings.SettingsMap = {
        'ui.theme': 'dark',
        'security.cerberus.enabled': 'true'
      }
      vi.mocked(client.get).mockResolvedValue({ data: mockData })

      const result = await settings.getSettings()

      expect(client.get).toHaveBeenCalledWith('/settings')
      expect(result).toEqual(mockData)
    })
  })

  describe('updateSetting', () => {
    it('should call POST /settings with key and value only', async () => {
      vi.mocked(client.post).mockResolvedValue({ data: {} })

      await settings.updateSetting('ui.theme', 'light')

      expect(client.post).toHaveBeenCalledWith('/settings', {
        key: 'ui.theme',
        value: 'light',
        category: undefined,
        type: undefined
      })
    })

    it('should call POST /settings with all parameters', async () => {
      vi.mocked(client.post).mockResolvedValue({ data: {} })

      await settings.updateSetting('security.cerberus.enabled', 'true', 'security', 'bool')

      expect(client.post).toHaveBeenCalledWith('/settings', {
        key: 'security.cerberus.enabled',
        value: 'true',
        category: 'security',
        type: 'bool'
      })
    })

    it('should call POST /settings with category but no type', async () => {
      vi.mocked(client.post).mockResolvedValue({ data: {} })

      await settings.updateSetting('ui.theme', 'dark', 'ui')

      expect(client.post).toHaveBeenCalledWith('/settings', {
        key: 'ui.theme',
        value: 'dark',
        category: 'ui',
        type: undefined
      })
    })
  })

  describe('validatePublicURL', () => {
    it('should call POST /settings/validate-url with URL', async () => {
      const mockResponse = { valid: true, normalized: 'https://example.com' }
      vi.mocked(client.post).mockResolvedValue({ data: mockResponse })

      const result = await settings.validatePublicURL('https://example.com')

      expect(client.post).toHaveBeenCalledWith('/settings/validate-url', { url: 'https://example.com' })
      expect(result).toEqual(mockResponse)
    })

    it('should return valid: true for valid URL', async () => {
      vi.mocked(client.post).mockResolvedValue({ data: { valid: true } })

      const result = await settings.validatePublicURL('https://valid.com')

      expect(result.valid).toBe(true)
    })

    it('should return valid: false for invalid URL', async () => {
      vi.mocked(client.post).mockResolvedValue({ data: { valid: false, error: 'Invalid URL format' } })

      const result = await settings.validatePublicURL('not-a-url')

      expect(result.valid).toBe(false)
      expect(result.error).toBe('Invalid URL format')
    })

    it('should return normalized URL when provided', async () => {
      vi.mocked(client.post).mockResolvedValue({
        data: { valid: true, normalized: 'https://example.com/' }
      })

      const result = await settings.validatePublicURL('https://example.com')

      expect(result.normalized).toBe('https://example.com/')
    })

    it('should handle validation errors', async () => {
      vi.mocked(client.post).mockRejectedValue(new Error('Network error'))

      await expect(settings.validatePublicURL('https://example.com')).rejects.toThrow('Network error')
    })

    it('should handle empty URL parameter', async () => {
      vi.mocked(client.post).mockResolvedValue({ data: { valid: false } })

      const result = await settings.validatePublicURL('')

      expect(client.post).toHaveBeenCalledWith('/settings/validate-url', { url: '' })
      expect(result.valid).toBe(false)
    })
  })

  describe('testPublicURL', () => {
    it('should call POST /settings/test-url with URL', async () => {
      const mockResponse = { reachable: true, latency: 42 }
      vi.mocked(client.post).mockResolvedValue({ data: mockResponse })

      const result = await settings.testPublicURL('https://example.com')

      expect(client.post).toHaveBeenCalledWith('/settings/test-url', { url: 'https://example.com' })
      expect(result).toEqual(mockResponse)
    })

    it('should return reachable: true with latency for successful test', async () => {
      vi.mocked(client.post).mockResolvedValue({
        data: { reachable: true, latency: 123, message: 'URL is reachable' }
      })

      const result = await settings.testPublicURL('https://example.com')

      expect(result.reachable).toBe(true)
      expect(result.latency).toBe(123)
      expect(result.message).toBe('URL is reachable')
    })

    it('should return reachable: false with error for failed test', async () => {
      vi.mocked(client.post).mockResolvedValue({
        data: { reachable: false, error: 'Connection timeout' }
      })

      const result = await settings.testPublicURL('https://unreachable.com')

      expect(result.reachable).toBe(false)
      expect(result.error).toBe('Connection timeout')
    })

    it('should return message field when provided', async () => {
      vi.mocked(client.post).mockResolvedValue({
        data: { reachable: true, latency: 50, message: 'Custom success message' }
      })

      const result = await settings.testPublicURL('https://example.com')

      expect(result.message).toBe('Custom success message')
    })

    it('should handle request errors', async () => {
      vi.mocked(client.post).mockRejectedValue(new Error('Request failed'))

      await expect(settings.testPublicURL('https://example.com')).rejects.toThrow('Request failed')
    })

    it('should handle empty URL parameter', async () => {
      vi.mocked(client.post).mockResolvedValue({ data: { reachable: false } })

      const result = await settings.testPublicURL('')

      expect(client.post).toHaveBeenCalledWith('/settings/test-url', { url: '' })
      expect(result.reachable).toBe(false)
    })
  })
})
