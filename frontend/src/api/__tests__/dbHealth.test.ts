import { describe, it, expect, vi, beforeEach } from 'vitest'

import client from '../../api/client'
import { getDbHealth } from '../dbHealth'

describe('dbHealth api', () => {
  beforeEach(() => {
    vi.restoreAllMocks()
  })

  it('getDbHealth fetches the healthy status', async () => {
    const mockData = {
      status: 'healthy' as const,
      integrity_ok: true,
      integrity_result: 'ok',
      wal_mode: true,
      journal_mode: 'wal',
      last_backup: '2026-07-16T03:00:00Z',
      checked_at: '2026-07-16T10:00:00Z',
    }
    const spy = vi.spyOn(client, 'get').mockResolvedValueOnce({ data: mockData })
    const res = await getDbHealth()
    expect(spy).toHaveBeenCalledWith('/health/db')
    expect(res).toEqual(mockData)
  })

  it('getDbHealth surfaces a corrupted status', async () => {
    const mockData = {
      status: 'corrupted' as const,
      integrity_ok: false,
      integrity_result: 'database disk image is malformed',
      wal_mode: true,
      journal_mode: 'wal',
      last_backup: null,
      checked_at: '2026-07-16T10:00:00Z',
    }
    vi.spyOn(client, 'get').mockResolvedValueOnce({ data: mockData })
    const res = await getDbHealth()
    expect(res.status).toBe('corrupted')
    expect(res.last_backup).toBeNull()
  })
})
