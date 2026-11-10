import { describe, it, expect, vi, afterEach } from 'vitest'

import client from '../client'
import {
  checkUpdates,
  getActiveAlerts,
  getNotifications,
  markNotificationRead,
  markAllNotificationsRead,
} from '../system'

vi.mock('../client', () => ({
  default: {
    get: vi.fn(),
    post: vi.fn(),
  },
}))

describe('System API', () => {
  afterEach(() => {
    vi.clearAllMocks()
  })

  it('checkUpdates calls /system/updates', async () => {
    const mockData = { available: true, latest_version: '1.0.0', changelog_url: 'url' }
    vi.mocked(client.get).mockResolvedValue({ data: mockData })

    const result = await checkUpdates()

    expect(client.get).toHaveBeenCalledWith('/system/updates')
    expect(result).toEqual(mockData)
  })

  it('getNotifications calls /notifications', async () => {
    const mockData = [{ id: '1', title: 'Test' }]
    vi.mocked(client.get).mockResolvedValue({ data: mockData })

    const result = await getNotifications()

    expect(client.get).toHaveBeenCalledWith('/notifications', { params: { unread: false } })
    expect(result).toEqual(mockData)
  })

  it('getNotifications calls /notifications with unreadOnly=true', async () => {
    const mockData = [{ id: '1', title: 'Test' }]
    vi.mocked(client.get).mockResolvedValue({ data: mockData })

    const result = await getNotifications(true)

    expect(client.get).toHaveBeenCalledWith('/notifications', { params: { unread: true } })
    expect(result).toEqual(mockData)
  })

  it('markNotificationRead calls /notifications/:id/read', async () => {
    vi.mocked(client.post).mockResolvedValue({})

    await markNotificationRead('123')

    expect(client.post).toHaveBeenCalledWith('/notifications/123/read')
  })

  it('markAllNotificationsRead calls /notifications/read-all', async () => {
    vi.mocked(client.post).mockResolvedValue({})

    await markAllNotificationsRead()

    expect(client.post).toHaveBeenCalledWith('/notifications/read-all')
  })

  it('getActiveAlerts calls /notifications/alerts and unwraps the alerts array', async () => {
    const alerts = [
      { key: 'geoip_unavailable', severity: 'error', affected_lists: ['Office'], affected_lists_total: 1, database_available: false },
    ]
    vi.mocked(client.get).mockResolvedValue({ data: { alerts } })

    const result = await getActiveAlerts()

    expect(client.get).toHaveBeenCalledWith('/notifications/alerts')
    expect(result).toEqual(alerts)
  })

  it('getActiveAlerts returns an empty array when nothing needs attention', async () => {
    vi.mocked(client.get).mockResolvedValue({ data: { alerts: [] } })

    await expect(getActiveAlerts()).resolves.toEqual([])
  })

  it('getActiveAlerts propagates request failures', async () => {
    vi.mocked(client.get).mockRejectedValue(new Error('network down'))

    await expect(getActiveAlerts()).rejects.toThrow('network down')
  })
})
