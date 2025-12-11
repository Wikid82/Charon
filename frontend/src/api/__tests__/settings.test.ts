import { describe, it, expect, vi, beforeEach } from 'vitest'
import * as settings from '../settings'
import client from '../client'

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
})
