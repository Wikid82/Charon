import { describe, it, expect, vi, beforeEach } from 'vitest'
import * as security from '../security'
import client from '../client'

vi.mock('../client')

describe('security API', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  describe('getSecurityStatus', () => {
    it('should call GET /security/status', async () => {
      const mockData: security.SecurityStatus = {
        cerberus: { enabled: true },
        crowdsec: { mode: 'local', api_url: 'http://localhost:8080', enabled: true },
        waf: { mode: 'enabled', enabled: true },
        rate_limit: { mode: 'enabled', enabled: true },
        acl: { enabled: true }
      }
      vi.mocked(client.get).mockResolvedValue({ data: mockData })

      const result = await security.getSecurityStatus()

      expect(client.get).toHaveBeenCalledWith('/security/status')
      expect(result).toEqual(mockData)
    })
  })

  describe('getSecurityConfig', () => {
    it('should call GET /security/config', async () => {
      const mockData = { config: { admin_whitelist: '10.0.0.0/8' } }
      vi.mocked(client.get).mockResolvedValue({ data: mockData })

      const result = await security.getSecurityConfig()

      expect(client.get).toHaveBeenCalledWith('/security/config')
      expect(result).toEqual(mockData)
    })
  })

  describe('updateSecurityConfig', () => {
    it('should call POST /security/config with payload', async () => {
      const payload: security.SecurityConfigPayload = {
        name: 'test',
        enabled: true,
        admin_whitelist: '10.0.0.0/8'
      }
      const mockData = { success: true }
      vi.mocked(client.post).mockResolvedValue({ data: mockData })

      const result = await security.updateSecurityConfig(payload)

      expect(client.post).toHaveBeenCalledWith('/security/config', payload)
      expect(result).toEqual(mockData)
    })

    it('should handle all payload fields', async () => {
      const payload: security.SecurityConfigPayload = {
        name: 'test',
        enabled: true,
        admin_whitelist: '10.0.0.0/8',
        crowdsec_mode: 'local',
        crowdsec_api_url: 'http://localhost:8080',
        waf_mode: 'enabled',
        waf_rules_source: 'coreruleset',
        waf_learning: true,
        rate_limit_enable: true,
        rate_limit_burst: 10,
        rate_limit_requests: 100,
        rate_limit_window_sec: 60
      }
      const mockData = { success: true }
      vi.mocked(client.post).mockResolvedValue({ data: mockData })

      const result = await security.updateSecurityConfig(payload)

      expect(client.post).toHaveBeenCalledWith('/security/config', payload)
      expect(result).toEqual(mockData)
    })
  })

  describe('generateBreakGlassToken', () => {
    it('should call POST /security/breakglass/generate', async () => {
      const mockData = { token: 'abc123' }
      vi.mocked(client.post).mockResolvedValue({ data: mockData })

      const result = await security.generateBreakGlassToken()

      expect(client.post).toHaveBeenCalledWith('/security/breakglass/generate')
      expect(result).toEqual(mockData)
    })
  })

  describe('enableCerberus', () => {
    it('should call POST /security/enable with payload', async () => {
      const payload = { mode: 'full' }
      const mockData = { success: true }
      vi.mocked(client.post).mockResolvedValue({ data: mockData })

      const result = await security.enableCerberus(payload)

      expect(client.post).toHaveBeenCalledWith('/security/enable', payload)
      expect(result).toEqual(mockData)
    })

    it('should call POST /security/enable with empty object when no payload', async () => {
      const mockData = { success: true }
      vi.mocked(client.post).mockResolvedValue({ data: mockData })

      const result = await security.enableCerberus()

      expect(client.post).toHaveBeenCalledWith('/security/enable', {})
      expect(result).toEqual(mockData)
    })
  })

  describe('disableCerberus', () => {
    it('should call POST /security/disable with payload', async () => {
      const payload = { reason: 'maintenance' }
      const mockData = { success: true }
      vi.mocked(client.post).mockResolvedValue({ data: mockData })

      const result = await security.disableCerberus(payload)

      expect(client.post).toHaveBeenCalledWith('/security/disable', payload)
      expect(result).toEqual(mockData)
    })

    it('should call POST /security/disable with empty object when no payload', async () => {
      const mockData = { success: true }
      vi.mocked(client.post).mockResolvedValue({ data: mockData })

      const result = await security.disableCerberus()

      expect(client.post).toHaveBeenCalledWith('/security/disable', {})
      expect(result).toEqual(mockData)
    })
  })

  describe('getDecisions', () => {
    it('should call GET /security/decisions with default limit', async () => {
      const mockData = { decisions: [] }
      vi.mocked(client.get).mockResolvedValue({ data: mockData })

      const result = await security.getDecisions()

      expect(client.get).toHaveBeenCalledWith('/security/decisions?limit=50')
      expect(result).toEqual(mockData)
    })

    it('should call GET /security/decisions with custom limit', async () => {
      const mockData = { decisions: [] }
      vi.mocked(client.get).mockResolvedValue({ data: mockData })

      const result = await security.getDecisions(100)

      expect(client.get).toHaveBeenCalledWith('/security/decisions?limit=100')
      expect(result).toEqual(mockData)
    })
  })

  describe('createDecision', () => {
    it('should call POST /security/decisions with payload', async () => {
      const payload = { ip: '1.2.3.4', duration: '4h', type: 'ban' }
      const mockData = { success: true }
      vi.mocked(client.post).mockResolvedValue({ data: mockData })

      const result = await security.createDecision(payload)

      expect(client.post).toHaveBeenCalledWith('/security/decisions', payload)
      expect(result).toEqual(mockData)
    })
  })

  describe('getRuleSets', () => {
    it('should call GET /security/rulesets', async () => {
      const mockData: security.RuleSetsResponse = {
        rulesets: [
          {
            id: 1,
            uuid: 'abc-123',
            name: 'OWASP CRS',
            source_url: 'https://example.com/rules',
            mode: 'blocking',
            last_updated: '2025-12-04T00:00:00Z',
            content: 'rule content'
          }
        ]
      }
      vi.mocked(client.get).mockResolvedValue({ data: mockData })

      const result = await security.getRuleSets()

      expect(client.get).toHaveBeenCalledWith('/security/rulesets')
      expect(result).toEqual(mockData)
    })
  })

  describe('upsertRuleSet', () => {
    it('should call POST /security/rulesets with create payload', async () => {
      const payload: security.UpsertRuleSetPayload = {
        name: 'Custom Rules',
        content: 'rule content',
        mode: 'blocking'
      }
      const mockData = { success: true }
      vi.mocked(client.post).mockResolvedValue({ data: mockData })

      const result = await security.upsertRuleSet(payload)

      expect(client.post).toHaveBeenCalledWith('/security/rulesets', payload)
      expect(result).toEqual(mockData)
    })

    it('should call POST /security/rulesets with update payload', async () => {
      const payload: security.UpsertRuleSetPayload = {
        id: 1,
        name: 'Updated Rules',
        source_url: 'https://example.com/rules',
        mode: 'detection'
      }
      const mockData = { success: true }
      vi.mocked(client.post).mockResolvedValue({ data: mockData })

      const result = await security.upsertRuleSet(payload)

      expect(client.post).toHaveBeenCalledWith('/security/rulesets', payload)
      expect(result).toEqual(mockData)
    })
  })

  describe('deleteRuleSet', () => {
    it('should call DELETE /security/rulesets/:id', async () => {
      const mockData = { success: true }
      vi.mocked(client.delete).mockResolvedValue({ data: mockData })

      const result = await security.deleteRuleSet(1)

      expect(client.delete).toHaveBeenCalledWith('/security/rulesets/1')
      expect(result).toEqual(mockData)
    })
  })
})
