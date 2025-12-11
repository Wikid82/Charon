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

    vi.mocked(client.get).mockResolvedValueOnce({ data: { filename: 'access.log', logs: [], total: 0, limit: 100, offset: 0 } })
    const resp = await getLogContent('access.log', {
      search: 'bot',
      host: 'example.com',
      status: '500',
      level: 'error',
      limit: 50,
      offset: 5,
      sort: 'asc',
    })
    expect(resp.filename).toBe('access.log')
    expect(client.get).toHaveBeenCalledWith('/logs/access.log?search=bot&host=example.com&status=500&level=error&limit=50&offset=5&sort=asc')
  })

  it('downloads log via window location', () => {
    downloadLog('access.log')
    expect(window.location.href).toBe('/api/v1/logs/access.log/download')
  })
})
