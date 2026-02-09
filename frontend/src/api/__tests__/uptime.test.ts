import { describe, it, expect, vi, beforeEach } from 'vitest'
import * as uptime from '../uptime'
import client from '../client'
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
