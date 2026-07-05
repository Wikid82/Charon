import { describe, it, expect, vi, beforeEach } from 'vitest'

import client from '../client'
import { downloadLog, getLogContent, getLogs } from '../logs'

vi.mock('../client', () => ({
  default: {
    get: vi.fn(),
  },
}))

describe('logs api http helpers', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    Object.defineProperty(window, 'location', {
      value: { href: 'http://localhost' },
      writable: true,
    })
  })

  it('fetches log list and content with filters', async () => {
    vi.mocked(client.get).mockResolvedValueOnce({ data: [{ name: 'access.log', size: 10, mod_time: 'now' }] })
    const logs = await getLogs()
    expect(logs[0].name).toBe('access.log')
    expect(client.get).toHaveBeenCalledWith('/logs')

    vi.mocked(client.get).mockResolvedValueOnce({ data: { filename: 'access.log', logs: [], total: 0, limit: 100, offset: 0, skipped_lines: 0 } })
    const resp = await getLogContent('access.log', {
      search: 'bot',
      host: 'example.com',
      status: '500',
      level: 'error',
      limit: 50,
      offset: 5,
      sort: 'asc',
      sortBy: 'uri',
    })
    expect(resp.filename).toBe('access.log')
    expect(client.get).toHaveBeenCalledWith('/logs/access.log?search=bot&host=example.com&status=500&level=error&limit=50&offset=5&sort=asc&sort_by=uri')
  })

  it('downloads log as a blob without navigating', async () => {
    const originalCreateObjectURL = URL.createObjectURL
    const originalRevokeObjectURL = URL.revokeObjectURL
    URL.createObjectURL = vi.fn().mockReturnValue('blob:mock-url')
    URL.revokeObjectURL = vi.fn()
    const clickSpy = vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => {})

    try {
      vi.mocked(client.get).mockResolvedValueOnce({ data: new Blob(['log content']) })

      await downloadLog('access.log')

      expect(client.get).toHaveBeenCalledWith('/logs/access.log/download', { responseType: 'blob' })
      expect(clickSpy).toHaveBeenCalledTimes(1)
      expect(URL.revokeObjectURL).toHaveBeenCalledWith('blob:mock-url')
      expect(window.location.href).toBe('http://localhost')
    } finally {
      clickSpy.mockRestore()
      URL.createObjectURL = originalCreateObjectURL
      URL.revokeObjectURL = originalRevokeObjectURL
    }
  })
})
