import { describe, it, expect, vi, beforeEach } from 'vitest'
import { detectDNSProvider, getDetectionPatterns } from '../dnsDetection'
import client from '../client'
import type { DetectionResult, NameserverPattern } from '../dnsDetection'

vi.mock('../client')

describe('dnsDetection API', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  describe('detectDNSProvider', () => {
    it('should detect DNS provider successfully', async () => {
      const mockResponse: DetectionResult = {
        domain: 'example.com',
        detected: true,
        provider_type: 'cloudflare',
        nameservers: ['ns1.cloudflare.com', 'ns2.cloudflare.com'],
        confidence: 'high',
        suggested_provider: {
          id: 1,
          uuid: 'test-uuid',
          name: 'Production Cloudflare',
          provider_type: 'cloudflare',
          enabled: true,
          is_default: true,
          has_credentials: true,
          propagation_timeout: 120,
          polling_interval: 5,
          success_count: 10,
          failure_count: 0,
          created_at: '2026-01-01T00:00:00Z',
          updated_at: '2026-01-01T00:00:00Z',
        },
      }

      vi.mocked(client.post).mockResolvedValue({ data: mockResponse })

      const result = await detectDNSProvider('example.com')

      expect(client.post).toHaveBeenCalledWith('/dns-providers/detect', { domain: 'example.com' })
      expect(result).toEqual(mockResponse)
      expect(result.detected).toBe(true)
      expect(result.provider_type).toBe('cloudflare')
      expect(result.confidence).toBe('high')
    })

    it('should handle detection failure (no provider found)', async () => {
      const mockResponse: DetectionResult = {
        domain: 'example.com',
        detected: false,
        nameservers: ['ns1.unknown.com', 'ns2.unknown.com'],
        confidence: 'none',
      }

      vi.mocked(client.post).mockResolvedValue({ data: mockResponse })

      const result = await detectDNSProvider('example.com')

      expect(result.detected).toBe(false)
      expect(result.confidence).toBe('none')
      expect(result.nameservers).toHaveLength(2)
    })

    it('should handle detection error', async () => {
      const mockResponse: DetectionResult = {
        domain: 'invalid.domain',
        detected: false,
        nameservers: [],
        confidence: 'none',
        error: 'Failed to lookup nameservers: domain not found',
      }

      vi.mocked(client.post).mockResolvedValue({ data: mockResponse })

      const result = await detectDNSProvider('invalid.domain')

      expect(result.detected).toBe(false)
      expect(result.error).toContain('domain not found')
    })

    it('should handle network error', async () => {
      vi.mocked(client.post).mockRejectedValue(new Error('Network error'))

      await expect(detectDNSProvider('example.com')).rejects.toThrow('Network error')
    })

    it('should handle medium confidence detection', async () => {
      const mockResponse: DetectionResult = {
        domain: 'example.com',
        detected: true,
        provider_type: 'route53',
        nameservers: ['ns-123.awsdns-12.com'],
        confidence: 'medium',
      }

      vi.mocked(client.post).mockResolvedValue({ data: mockResponse })

      const result = await detectDNSProvider('example.com')

      expect(result.confidence).toBe('medium')
      expect(result.detected).toBe(true)
    })
  })

  describe('getDetectionPatterns', () => {
    it('should fetch detection patterns successfully', async () => {
      const mockPatterns: NameserverPattern[] = [
        { pattern: '.ns.cloudflare.com', provider_type: 'cloudflare' },
        { pattern: '.awsdns', provider_type: 'route53' },
        { pattern: '.digitalocean.com', provider_type: 'digitalocean' },
      ]

      vi.mocked(client.get).mockResolvedValue({ data: { patterns: mockPatterns } })

      const result = await getDetectionPatterns()

      expect(client.get).toHaveBeenCalledWith('/dns-providers/patterns')
      expect(result).toEqual(mockPatterns)
      expect(result).toHaveLength(3)
    })

    it('should handle empty patterns list', async () => {
      vi.mocked(client.get).mockResolvedValue({ data: { patterns: [] } })

      const result = await getDetectionPatterns()

      expect(result).toEqual([])
    })

    it('should handle network error when fetching patterns', async () => {
      vi.mocked(client.get).mockRejectedValue(new Error('Network error'))

      await expect(getDetectionPatterns()).rejects.toThrow('Network error')
    })
  })
})
