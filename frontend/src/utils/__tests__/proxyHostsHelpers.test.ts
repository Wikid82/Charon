import { vi } from 'vitest'

import {
  formatSettingLabel,
  settingHelpText,
  settingKeyToField,
  applyBulkSettingsToHosts,
} from '../proxyHostsHelpers'

import type { ProxyHost } from '../../api/proxyHosts'

describe('proxyHostsHelpers', () => {
  describe('formatSettingLabel', () => {
    it('returns correct labels for known keys', () => {
      expect(formatSettingLabel('ssl_forced')).toBe('Force SSL')
      expect(formatSettingLabel('http2_support')).toBe('HTTP/2 Support')
      expect(formatSettingLabel('hsts_enabled')).toBe('HSTS Enabled')
      expect(formatSettingLabel('hsts_subdomains')).toBe('HSTS Subdomains')
      expect(formatSettingLabel('block_exploits')).toBe('Block Exploits')
      expect(formatSettingLabel('websocket_support')).toBe('Websockets Support')
      expect(formatSettingLabel('enable_standard_headers')).toBe('Standard Proxy Headers')
    })
    it('returns key for unknown keys', () => {
      expect(formatSettingLabel('unknown_key')).toBe('unknown_key')
    })
  })

  describe('settingHelpText', () => {
    it('returns correct help text for known keys', () => {
      expect(settingHelpText('ssl_forced')).toContain('Redirect all HTTP traffic')
      expect(settingHelpText('http2_support')).toContain('Enable HTTP/2')
      expect(settingHelpText('block_exploits')).toContain('Add common exploit-mitigation')
    })
    it('returns empty string for unknown keys', () => {
      expect(settingHelpText('unknown_key')).toBe('')
    })
  })

  describe('settingKeyToField', () => {
    it('returns correct field for known keys', () => {
      expect(settingKeyToField('ssl_forced')).toBe('ssl_forced')
      expect(settingKeyToField('websocket_support')).toBe('websocket_support')
    })
    it('returns key for unknown keys', () => {
      expect(settingKeyToField('unknown_key')).toBe('unknown_key')
    })
  })

  describe('applyBulkSettingsToHosts', () => {
    const mockHosts: ProxyHost[] = [
      { uuid: 'h1', is_enabled: true } as unknown as ProxyHost,
      { uuid: 'h2', is_enabled: false } as unknown as ProxyHost
    ]
    const mockUpdateHost = vi.fn()
    const mockSetProgress = vi.fn()

    beforeEach(() => {
      vi.clearAllMocks()
    })

    it('applies settings to specified hosts', async () => {
      mockUpdateHost.mockResolvedValue({} as ProxyHost)

      const result = await applyBulkSettingsToHosts({
        hosts: mockHosts,
        hostUUIDs: ['h1'],
        keysToApply: ['ssl_forced'],
        bulkApplySettings: {
            ssl_forced: { apply: true, value: true }
        },
        updateHost: mockUpdateHost,
        setApplyProgress: mockSetProgress
      })

      expect(result).toEqual({ errors: 0, completed: 1 })
      expect(mockUpdateHost).toHaveBeenCalledWith('h1', expect.objectContaining({
          uuid: 'h1',
          ssl_forced: true
      }))
      expect(mockUpdateHost).toHaveBeenCalledTimes(1)
      expect(mockSetProgress).toHaveBeenCalled()
    })

    it('handles errors during update', async () => {
      mockUpdateHost.mockRejectedValue(new Error('Update failed'))

      const result = await applyBulkSettingsToHosts({
        hosts: mockHosts,
        hostUUIDs: ['h1'],
        keysToApply: ['ssl_forced'],
        bulkApplySettings: {
            ssl_forced: { apply: true, value: true }
        },
        updateHost: mockUpdateHost
      })

      expect(result).toEqual({ errors: 1, completed: 1 })
    })

    it('handles missing hosts', async () => {
        const result = await applyBulkSettingsToHosts({
            hosts: mockHosts,
            // h3 doesn't exist
            hostUUIDs: ['h3'],
            keysToApply: ['ssl_forced'],
            bulkApplySettings: {
                ssl_forced: { apply: true, value: true }
            },
            updateHost: mockUpdateHost
        })

        expect(result).toEqual({ errors: 1, completed: 1 })
        expect(mockUpdateHost).not.toHaveBeenCalled()
    })

    it('handles multiple hosts and settings', async () => {
        mockUpdateHost.mockResolvedValue({} as ProxyHost)

        const result = await applyBulkSettingsToHosts({
          hosts: mockHosts,
          hostUUIDs: ['h1', 'h2'],
          keysToApply: ['ssl_forced', 'http2_support'],
          bulkApplySettings: {
              ssl_forced: { apply: true, value: true },
              http2_support: { apply: true, value: false }
          },
          updateHost: mockUpdateHost,
          setApplyProgress: mockSetProgress
        })

        expect(result).toEqual({ errors: 0, completed: 2 })
        expect(mockUpdateHost).toHaveBeenCalledWith('h1', expect.objectContaining({
            uuid: 'h1',
            ssl_forced: true,
            http2_support: false
        }))
        expect(mockUpdateHost).toHaveBeenCalledWith('h2', expect.objectContaining({
            uuid: 'h2',
            ssl_forced: true,
            http2_support: false
        }))
        expect(mockUpdateHost).toHaveBeenCalledTimes(2)
      })
  })
})
