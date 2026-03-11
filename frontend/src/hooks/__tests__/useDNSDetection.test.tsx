import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { renderHook, waitFor } from '@testing-library/react'
import { describe, it, expect, vi, beforeEach } from 'vitest'

import * as api from '../../api/dnsDetection'
import { useDetectDNSProvider, useCachedDetectionResult, useDetectionPatterns } from '../useDNSDetection'

import type { DetectionResult, NameserverPattern } from '../../api/dnsDetection'

vi.mock('../../api/dnsDetection')

const createWrapper = () => {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false },
    },
  })
  return ({ children }: { children: React.ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  )
}

describe('useDNSDetection hooks', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  describe('useDetectDNSProvider', () => {
    it('should detect DNS provider successfully', async () => {
      const mockResult: DetectionResult = {
        domain: 'example.com',
        detected: true,
        provider_type: 'cloudflare',
        nameservers: ['ns1.cloudflare.com'],
        confidence: 'high',
      }

      vi.mocked(api.detectDNSProvider).mockResolvedValue(mockResult)

      const { result } = renderHook(() => useDetectDNSProvider(), {
        wrapper: createWrapper(),
      })

      result.current.mutate('example.com')

      await waitFor(() => expect(result.current.isSuccess).toBe(true))

      expect(result.current.data).toEqual(mockResult)
      expect(api.detectDNSProvider).toHaveBeenCalledWith('example.com')
    })

    it('should handle detection error', async () => {
      vi.mocked(api.detectDNSProvider).mockRejectedValue(new Error('Network error'))

      const { result } = renderHook(() => useDetectDNSProvider(), {
        wrapper: createWrapper(),
      })

      result.current.mutate('example.com')

      await waitFor(() => expect(result.current.isError).toBe(true))

      expect(result.current.error).toEqual(new Error('Network error'))
    })

    it('should cache detection result for 1 hour', async () => {
      const mockResult: DetectionResult = {
        domain: 'example.com',
        detected: true,
        provider_type: 'cloudflare',
        nameservers: ['ns1.cloudflare.com'],
        confidence: 'high',
      }

      vi.mocked(api.detectDNSProvider).mockResolvedValue(mockResult)

      const { result } = renderHook(() => useDetectDNSProvider(), {
        wrapper: createWrapper(),
      })

      result.current.mutate('example.com')

      await waitFor(() => expect(result.current.isSuccess).toBe(true))

      // Result should be cached
      expect(result.current.data).toEqual(mockResult)
    })

    it('should handle not detected scenario', async () => {
      const mockResult: DetectionResult = {
        domain: 'example.com',
        detected: false,
        nameservers: ['ns1.unknown.com'],
        confidence: 'none',
      }

      vi.mocked(api.detectDNSProvider).mockResolvedValue(mockResult)

      const { result } = renderHook(() => useDetectDNSProvider(), {
        wrapper: createWrapper(),
      })

      result.current.mutate('example.com')

      await waitFor(() => expect(result.current.isSuccess).toBe(true))

      expect(result.current.data?.detected).toBe(false)
    })
  })

  describe('useCachedDetectionResult', () => {
    it('should fetch and cache detection result', async () => {
      const mockResult: DetectionResult = {
        domain: 'example.com',
        detected: true,
        provider_type: 'cloudflare',
        nameservers: ['ns1.cloudflare.com'],
        confidence: 'high',
      }

      vi.mocked(api.detectDNSProvider).mockResolvedValue(mockResult)

      const { result } = renderHook(() => useCachedDetectionResult('example.com', true), {
        wrapper: createWrapper(),
      })

      await waitFor(() => expect(result.current.isSuccess).toBe(true))

      expect(result.current.data).toEqual(mockResult)
    })

    it('should not fetch when disabled', async () => {
      const { result } = renderHook(() => useCachedDetectionResult('example.com', false), {
        wrapper: createWrapper(),
      })

      // Wait a bit to ensure no fetch happens
      await new Promise(resolve => setTimeout(resolve, 100))

      expect(result.current.data).toBeUndefined()
      expect(api.detectDNSProvider).not.toHaveBeenCalled()
    })

    it('should not fetch when domain is empty', async () => {
      const { result } = renderHook(() => useCachedDetectionResult('', true), {
        wrapper: createWrapper(),
      })

      await new Promise(resolve => setTimeout(resolve, 100))

      expect(result.current.data).toBeUndefined()
      expect(api.detectDNSProvider).not.toHaveBeenCalled()
    })
  })

  describe('useDetectionPatterns', () => {
    it('should fetch detection patterns successfully', async () => {
      const mockPatterns: NameserverPattern[] = [
        { pattern: '.ns.cloudflare.com', provider_type: 'cloudflare' },
        { pattern: '.awsdns', provider_type: 'route53' },
      ]

      vi.mocked(api.getDetectionPatterns).mockResolvedValue(mockPatterns)

      const { result } = renderHook(() => useDetectionPatterns(), {
        wrapper: createWrapper(),
      })

      await waitFor(() => expect(result.current.isSuccess).toBe(true))

      expect(result.current.data).toEqual(mockPatterns)
      expect(result.current.data).toHaveLength(2)
    })

    it('should cache patterns for 24 hours', async () => {
      const mockPatterns: NameserverPattern[] = [
        { pattern: '.ns.cloudflare.com', provider_type: 'cloudflare' },
      ]

      vi.mocked(api.getDetectionPatterns).mockResolvedValue(mockPatterns)

      const { result } = renderHook(() => useDetectionPatterns(), {
        wrapper: createWrapper(),
      })

      await waitFor(() => expect(result.current.isSuccess).toBe(true))

      // Should only call API once due to caching
      expect(api.getDetectionPatterns).toHaveBeenCalledTimes(1)
    })

    it('should handle error fetching patterns', async () => {
      vi.mocked(api.getDetectionPatterns).mockRejectedValue(new Error('API error'))

      const { result } = renderHook(() => useDetectionPatterns(), {
        wrapper: createWrapper(),
      })

      await waitFor(() => expect(result.current.isError).toBe(true))

      expect(result.current.error).toEqual(new Error('API error'))
    })
  })
})
