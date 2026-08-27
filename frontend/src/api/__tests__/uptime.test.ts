import { describe, it, expect, vi, beforeEach } from 'vitest'

import client from '../client'
import * as uptime from '../uptime'

import type { UptimeMonitor, UptimeHeartbeat } from '../uptime'

vi.mock('../client')

describe('uptime API', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  describe('getMonitors', () => {
    it('should call GET /uptime/monitors', async () => {
      const mockData: UptimeMonitor[] = [
        {
          id: 'mon-1',
          name: 'Test Monitor',
          type: 'http',
          url: 'https://example.com',
          interval: 60,
          enabled: true,
          status: 'up',
          latency: 100,
          max_retries: 3
        }
      ]
      vi.mocked(client.get).mockResolvedValue({ data: mockData })

      const result = await uptime.getMonitors()

      expect(client.get).toHaveBeenCalledWith('/uptime/monitors')
      expect(result).toEqual(mockData)
    })
  })

  describe('getMonitorHistory', () => {
    it('should call GET /uptime/monitors/:id/history with default limit', async () => {
      const mockData: UptimeHeartbeat[] = [
        {
          id: 1,
          monitor_id: 'mon-1',
          status: 'up',
          latency: 100,
          message: 'OK',
          created_at: '2025-12-04T00:00:00Z'
        }
      ]
      vi.mocked(client.get).mockResolvedValue({ data: mockData })

      const result = await uptime.getMonitorHistory('mon-1')

      expect(client.get).toHaveBeenCalledWith('/uptime/monitors/mon-1/history?limit=50')
      expect(result).toEqual(mockData)
    })

    it('should call GET /uptime/monitors/:id/history with custom limit', async () => {
      const mockData: UptimeHeartbeat[] = []
      vi.mocked(client.get).mockResolvedValue({ data: mockData })

      const result = await uptime.getMonitorHistory('mon-1', 100)

      expect(client.get).toHaveBeenCalledWith('/uptime/monitors/mon-1/history?limit=100')
      expect(result).toEqual(mockData)
    })

    it('should append the before cursor when provided', async () => {
      vi.mocked(client.get).mockResolvedValue({ data: [] })

      await uptime.getMonitorHistory('mon-1', 60, '2026-08-27T12:00:00Z')

      expect(client.get).toHaveBeenCalledWith(
        '/uptime/monitors/mon-1/history?limit=60&before=2026-08-27T12%3A00%3A00Z'
      )
    })
  })

  describe('getMonitorsSummary', () => {
    it('should call GET /uptime/monitors/summary with the default beats window', async () => {
      const mockData: uptime.MonitorSummary[] = [
        {
          id: 'mon-1',
          name: 'Test Monitor',
          type: 'http',
          url: 'https://example.com',
          enabled: true,
          status: 'up',
          latency: 40,
          last_check: '2026-08-27T12:00:00Z',
          interval: 30,
          proxy_host_id: 12,
          remote_server_id: null,
          uptime_24h: 99.9,
          recent_beats: [{ status: 'up', latency: 40, created_at: '2026-08-27T11:59:30Z' }],
        },
      ]
      vi.mocked(client.get).mockResolvedValue({ data: mockData })

      const result = await uptime.getMonitorsSummary()

      expect(client.get).toHaveBeenCalledWith('/uptime/monitors/summary?beats=30')
      expect(result).toEqual(mockData)
    })

    it('should forward a custom beats window', async () => {
      vi.mocked(client.get).mockResolvedValue({ data: [] })

      await uptime.getMonitorsSummary(60)

      expect(client.get).toHaveBeenCalledWith('/uptime/monitors/summary?beats=60')
    })
  })

  describe('getUptimeHealth', () => {
    it('should call GET /uptime/health', async () => {
      const mockData: uptime.UptimeHealth = {
        heartbeats_dropped: 0,
        checks_enqueue_dropped: 0,
        queue_depth: 3,
        worker_pool_size: 30,
      }
      vi.mocked(client.get).mockResolvedValue({ data: mockData })

      const result = await uptime.getUptimeHealth()

      expect(client.get).toHaveBeenCalledWith('/uptime/health')
      expect(result).toEqual(mockData)
    })
  })

  describe('updateMonitor', () => {
    it('should call PUT /uptime/monitors/:id', async () => {
      const mockMonitor: UptimeMonitor = {
        id: 'mon-1',
        name: 'Updated Monitor',
        type: 'http',
        url: 'https://example.com',
        interval: 120,
        enabled: false,
        status: 'down',
        latency: 0,
        max_retries: 5
      }
      vi.mocked(client.put).mockResolvedValue({ data: mockMonitor })

      const result = await uptime.updateMonitor('mon-1', { enabled: false, interval: 120 })

      expect(client.put).toHaveBeenCalledWith('/uptime/monitors/mon-1', { enabled: false, interval: 120 })
      expect(result).toEqual(mockMonitor)
    })
  })

  describe('deleteMonitor', () => {
    it('should call DELETE /uptime/monitors/:id', async () => {
      vi.mocked(client.delete).mockResolvedValue({ data: undefined })

      const result = await uptime.deleteMonitor('mon-1')

      expect(client.delete).toHaveBeenCalledWith('/uptime/monitors/mon-1')
      expect(result).toBeUndefined()
    })
  })

  describe('syncMonitors', () => {
    it('should call POST /uptime/sync with empty body when no params', async () => {
      const mockData = { synced: 5 }
      vi.mocked(client.post).mockResolvedValue({ data: mockData })

      const result = await uptime.syncMonitors()

      expect(client.post).toHaveBeenCalledWith('/uptime/sync', {})
      expect(result).toEqual(mockData)
    })

    it('should call POST /uptime/sync with provided parameters', async () => {
      const mockData = { synced: 5 }
      const body = { interval: 120, max_retries: 5 }
      vi.mocked(client.post).mockResolvedValue({ data: mockData })

      const result = await uptime.syncMonitors(body)

      expect(client.post).toHaveBeenCalledWith('/uptime/sync', body)
      expect(result).toEqual(mockData)
    })
  })

  describe('checkMonitor', () => {
    it('should call POST /uptime/monitors/:id/check', async () => {
      const mockData = { message: 'Check initiated' }
      vi.mocked(client.post).mockResolvedValue({ data: mockData })

      const result = await uptime.checkMonitor('mon-1')

      expect(client.post).toHaveBeenCalledWith('/uptime/monitors/mon-1/check')
      expect(result).toEqual(mockData)
    })
  })
})
