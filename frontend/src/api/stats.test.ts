import { describe, it, expect, vi, beforeEach } from 'vitest'

import {
  getStatsSummary,
  getTopHosts,
  getStatusDistribution,
  getTrafficVolume,
  getCertExpiry,
  getRequests,
  getStatsHealth,
  connectStatsWebSocket,
  type StatsSummary,
  type HostStat,
  type StatusStat,
  type TrafficBucket,
  type CertExpiry,
  type StatsHealth,
  type StatsPushMessage,
} from './stats'
import client from './client'

vi.mock('./client', () => ({
  default: {
    get: vi.fn(),
  },
}))

const mockedClient = client as unknown as {
  get: ReturnType<typeof vi.fn>
}

describe('stats api', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  describe('getStatsSummary', () => {
    it('calls the correct endpoint', async () => {
      const mockSummary: StatsSummary = {
        requests_last_24h: 100,
        requests_last_7d: 700,
        requests_last_30d: 3000,
      }
      mockedClient.get.mockResolvedValueOnce({ data: mockSummary })

      const result = await getStatsSummary()

      expect(mockedClient.get).toHaveBeenCalledWith('/stats/summary')
      expect(result).toEqual(mockSummary)
      expect(result.requests_last_24h).toBe(100)
      expect(result.requests_last_7d).toBe(700)
      expect(result.requests_last_30d).toBe(3000)
    })

    it('propagates errors', async () => {
      mockedClient.get.mockRejectedValueOnce(new Error('Network error'))

      await expect(getStatsSummary()).rejects.toThrow('Network error')
    })
  })

  describe('getTopHosts', () => {
    it('calls the correct endpoint with provided period and limit', async () => {
      const mockHosts: HostStat[] = [
        { host_id: 'uuid-1', hostname: 'example.com', count: 500 },
        { host_id: 'uuid-2', hostname: 'api.example.com', count: 250 },
      ]
      mockedClient.get.mockResolvedValueOnce({ data: mockHosts })

      const result = await getTopHosts('7d', 5)

      expect(mockedClient.get).toHaveBeenCalledWith('/stats/top-hosts?period=7d&limit=5')
      expect(result).toEqual(mockHosts)
      expect(result[0].hostname).toBe('example.com')
    })

    it('uses default limit of 10 when limit is omitted', async () => {
      mockedClient.get.mockResolvedValueOnce({ data: [] })

      await getTopHosts('24h')

      expect(mockedClient.get).toHaveBeenCalledWith('/stats/top-hosts?period=24h&limit=10')
    })

    it('supports all valid periods', async () => {
      mockedClient.get.mockResolvedValue({ data: [] })

      await getTopHosts('30d')
      expect(mockedClient.get).toHaveBeenCalledWith('/stats/top-hosts?period=30d&limit=10')
    })

    it('propagates errors', async () => {
      mockedClient.get.mockRejectedValueOnce(new Error('Fetch failed'))

      await expect(getTopHosts('24h')).rejects.toThrow('Fetch failed')
    })
  })

  describe('getStatusDistribution', () => {
    it('calls the correct endpoint with the given period', async () => {
      const mockStats: StatusStat[] = [
        { code: 200, count: 900 },
        { code: 404, count: 50 },
        { code: 500, count: 10 },
      ]
      mockedClient.get.mockResolvedValueOnce({ data: mockStats })

      const result = await getStatusDistribution('7d')

      expect(mockedClient.get).toHaveBeenCalledWith('/stats/status-distribution?period=7d')
      expect(result).toEqual(mockStats)
      expect(result[0].code).toBe(200)
      expect(result[1].count).toBe(50)
    })

    it('supports all valid periods', async () => {
      mockedClient.get.mockResolvedValue({ data: [] })

      await getStatusDistribution('24h')
      expect(mockedClient.get).toHaveBeenCalledWith('/stats/status-distribution?period=24h')

      await getStatusDistribution('30d')
      expect(mockedClient.get).toHaveBeenCalledWith('/stats/status-distribution?period=30d')
    })

    it('propagates errors', async () => {
      mockedClient.get.mockRejectedValueOnce(new Error('Server error'))

      await expect(getStatusDistribution('7d')).rejects.toThrow('Server error')
    })
  })

  describe('getTrafficVolume', () => {
    it('calls the correct endpoint with the given bucket', async () => {
      const mockBuckets: TrafficBucket[] = [
        { bucket: '2024-01-01T00:00:00Z', bytes_sent: 1024000 },
        { bucket: '2024-01-01T01:00:00Z', bytes_sent: 2048000 },
      ]
      mockedClient.get.mockResolvedValueOnce({ data: mockBuckets })

      const result = await getTrafficVolume('1h')

      expect(mockedClient.get).toHaveBeenCalledWith('/stats/traffic-volume?bucket=1h')
      expect(result).toEqual(mockBuckets)
      expect(result[0].bytes_sent).toBe(1024000)
    })

    it('supports 6h bucket', async () => {
      mockedClient.get.mockResolvedValueOnce({ data: [] })

      await getTrafficVolume('6h')

      expect(mockedClient.get).toHaveBeenCalledWith('/stats/traffic-volume?bucket=6h')
    })

    it('supports 1d bucket', async () => {
      mockedClient.get.mockResolvedValueOnce({ data: [] })

      await getTrafficVolume('1d')

      expect(mockedClient.get).toHaveBeenCalledWith('/stats/traffic-volume?bucket=1d')
    })

    it('propagates errors', async () => {
      mockedClient.get.mockRejectedValueOnce(new Error('Timeout'))

      await expect(getTrafficVolume('1h')).rejects.toThrow('Timeout')
    })
  })

  describe('getCertExpiry', () => {
    it('calls the correct endpoint with provided withinDays', async () => {
      const mockCerts: CertExpiry[] = [
        {
          host_id: 'host-uuid-1',
          hostname: 'secure.example.com',
          expires_at: '2024-02-15T00:00:00Z',
          days_left: 7,
        },
      ]
      mockedClient.get.mockResolvedValueOnce({ data: mockCerts })

      const result = await getCertExpiry(14)

      expect(mockedClient.get).toHaveBeenCalledWith('/stats/cert-expiry?within_days=14')
      expect(result).toEqual(mockCerts)
      expect(result[0].days_left).toBe(7)
      expect(result[0].hostname).toBe('secure.example.com')
    })

    it('uses default withinDays of 30 when omitted', async () => {
      mockedClient.get.mockResolvedValueOnce({ data: [] })

      await getCertExpiry()

      expect(mockedClient.get).toHaveBeenCalledWith('/stats/cert-expiry?within_days=30')
    })

    it('propagates errors', async () => {
      mockedClient.get.mockRejectedValueOnce(new Error('Not found'))

      await expect(getCertExpiry()).rejects.toThrow('Not found')
    })
  })

  describe('getRequests', () => {
    it('calls the correct endpoint with the given bucket', async () => {
      const mockBuckets: TrafficBucket[] = [
        { bucket: '2024-01-01T00:00:00Z', bytes_sent: 320 },
        { bucket: '2024-01-01T06:00:00Z', bytes_sent: 480 },
      ]
      mockedClient.get.mockResolvedValueOnce({ data: mockBuckets })

      const result = await getRequests('6h')

      expect(mockedClient.get).toHaveBeenCalledWith('/stats/requests?bucket=6h')
      expect(result).toEqual(mockBuckets)
      expect(result[1].bucket).toBe('2024-01-01T06:00:00Z')
    })

    it('supports 1h bucket', async () => {
      mockedClient.get.mockResolvedValueOnce({ data: [] })

      await getRequests('1h')

      expect(mockedClient.get).toHaveBeenCalledWith('/stats/requests?bucket=1h')
    })

    it('supports 1d bucket', async () => {
      mockedClient.get.mockResolvedValueOnce({ data: [] })

      await getRequests('1d')

      expect(mockedClient.get).toHaveBeenCalledWith('/stats/requests?bucket=1d')
    })

    it('propagates errors', async () => {
      mockedClient.get.mockRejectedValueOnce(new Error('Unauthorized'))

      await expect(getRequests('1h')).rejects.toThrow('Unauthorized')
    })
  })

  describe('getStatsHealth', () => {
    it('calls the correct endpoint', async () => {
      const mockHealth: StatsHealth = { dropped_count: 3 }
      mockedClient.get.mockResolvedValueOnce({ data: mockHealth })

      const result = await getStatsHealth()

      expect(mockedClient.get).toHaveBeenCalledWith('/stats/health')
      expect(result).toEqual(mockHealth)
      expect(result.dropped_count).toBe(3)
    })

    it('returns zero dropped_count when healthy', async () => {
      const mockHealth: StatsHealth = { dropped_count: 0 }
      mockedClient.get.mockResolvedValueOnce({ data: mockHealth })

      const result = await getStatsHealth()

      expect(result.dropped_count).toBe(0)
    })

    it('propagates errors', async () => {
      mockedClient.get.mockRejectedValueOnce(new Error('Service unavailable'))

      await expect(getStatsHealth()).rejects.toThrow('Service unavailable')
    })
  })

  describe('connectStatsWebSocket', () => {
    let mockWs: {
      onopen: (() => void) | null
      onmessage: ((event: MessageEvent) => void) | null
      onerror: ((event: Event) => void) | null
      onclose: ((event: CloseEvent) => void) | null
      close: ReturnType<typeof vi.fn>
      readyState: number
    }

    beforeEach(() => {
      mockWs = {
        onopen: null,
        onmessage: null,
        onerror: null,
        onclose: null,
        close: vi.fn(),
        readyState: WebSocket.OPEN,
      }
      // Must be a vi.fn() spy wrapping a constructor so `new WebSocket(...)` works
      // and vitest `toHaveBeenCalledWith` assertions succeed.
      const MockWebSocket = vi.fn().mockImplementation(function () {
        return mockWs
      }) as ReturnType<typeof vi.fn> & { OPEN: number; CONNECTING: number; CLOSED: number }
      // Attach the readyState constants used by connectStatsWebSocket
      MockWebSocket.OPEN = 1
      MockWebSocket.CONNECTING = 0
      MockWebSocket.CLOSED = 3
      vi.stubGlobal('WebSocket', MockWebSocket)
    })

    it('connects to the correct WebSocket URL using ws: for http:', () => {
      vi.stubGlobal('location', {
        protocol: 'http:',
        host: 'localhost:3000',
      })

      connectStatsWebSocket(vi.fn())

      expect(WebSocket).toHaveBeenCalledWith('ws://localhost:3000/api/v1/stats/ws')
    })

    it('uses wss: when page is served over https:', () => {
      vi.stubGlobal('location', {
        protocol: 'https:',
        host: 'proxy.example.com',
      })

      connectStatsWebSocket(vi.fn())

      expect(WebSocket).toHaveBeenCalledWith('wss://proxy.example.com/api/v1/stats/ws')
    })

    it('invokes onMessage callback with parsed StatsPushMessage', () => {
      vi.stubGlobal('location', { protocol: 'http:', host: 'localhost' })
      const onMessage = vi.fn()

      connectStatsWebSocket(onMessage)

      const msg: StatsPushMessage = {
        type: 'stats_update',
        data: { requests_last_24h: 10, requests_last_7d: 70, requests_last_30d: 300 },
      }
      const event = { data: JSON.stringify(msg) } as MessageEvent
      mockWs.onmessage?.(event)

      expect(onMessage).toHaveBeenCalledWith(msg)
    })

    it('invokes onOpen callback when connection opens', () => {
      vi.stubGlobal('location', { protocol: 'http:', host: 'localhost' })
      const onOpen = vi.fn()

      connectStatsWebSocket(vi.fn(), onOpen)
      mockWs.onopen?.()

      expect(onOpen).toHaveBeenCalledOnce()
    })

    it('invokes onError callback and logs on WebSocket error', () => {
      vi.stubGlobal('location', { protocol: 'http:', host: 'localhost' })
      const onError = vi.fn()
      const consoleSpy = vi.spyOn(console, 'error').mockImplementation(() => undefined)

      connectStatsWebSocket(vi.fn(), undefined, onError)
      const event = new Event('error')
      mockWs.onerror?.(event)

      expect(onError).toHaveBeenCalledWith(event)
      consoleSpy.mockRestore()
    })

    it('invokes onClose callback when connection closes', () => {
      vi.stubGlobal('location', { protocol: 'http:', host: 'localhost' })
      const onClose = vi.fn()
      const consoleSpy = vi.spyOn(console, 'log').mockImplementation(() => undefined)

      connectStatsWebSocket(vi.fn(), undefined, undefined, onClose)
      const event = new CloseEvent('close', { code: 1000, reason: 'Normal closure', wasClean: true })
      mockWs.onclose?.(event)

      expect(onClose).toHaveBeenCalledOnce()
      consoleSpy.mockRestore()
    })

    it('returns a cleanup function that closes an open connection', () => {
      vi.stubGlobal('location', { protocol: 'http:', host: 'localhost' })
      mockWs.readyState = WebSocket.OPEN

      const cleanup = connectStatsWebSocket(vi.fn())
      cleanup()

      expect(mockWs.close).toHaveBeenCalledOnce()
    })

    it('cleanup function closes a connecting WebSocket', () => {
      vi.stubGlobal('location', { protocol: 'http:', host: 'localhost' })
      mockWs.readyState = WebSocket.CONNECTING

      const cleanup = connectStatsWebSocket(vi.fn())
      cleanup()

      expect(mockWs.close).toHaveBeenCalledOnce()
    })

    it('cleanup function does not close an already closed WebSocket', () => {
      vi.stubGlobal('location', { protocol: 'http:', host: 'localhost' })
      mockWs.readyState = WebSocket.CLOSED

      const cleanup = connectStatsWebSocket(vi.fn())
      cleanup()

      expect(mockWs.close).not.toHaveBeenCalled()
    })

    it('does not invoke onMessage for malformed JSON', () => {
      vi.stubGlobal('location', { protocol: 'http:', host: 'localhost' })
      const onMessage = vi.fn()
      const consoleSpy = vi.spyOn(console, 'error').mockImplementation(() => undefined)

      connectStatsWebSocket(onMessage)

      const event = { data: 'not-valid-json{{{' } as MessageEvent
      mockWs.onmessage?.(event)

      expect(onMessage).not.toHaveBeenCalled()
      expect(consoleSpy).toHaveBeenCalled()
      consoleSpy.mockRestore()
    })
  })
})
