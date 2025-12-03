import { describe, it, expect, vi, beforeEach } from 'vitest'
import client from '../../api/client'
import { getBackups, createBackup, restoreBackup, deleteBackup } from '../backups'

describe('backups api', () => {
  beforeEach(() => {
    vi.restoreAllMocks()
  })

  it('getBackups returns list', async () => {
    const mockData = [{ filename: 'b1.zip', size: 123, time: '2025-01-01T00:00:00Z' }]
    vi.spyOn(client, 'get').mockResolvedValueOnce({ data: mockData })
    const res = await getBackups()
    expect(res).toEqual(mockData)
  })

  it('createBackup returns filename', async () => {
    vi.spyOn(client, 'post').mockResolvedValueOnce({ data: { filename: 'b2.zip' } })
    const res = await createBackup()
    expect(res).toEqual({ filename: 'b2.zip' })
  })

  it('restoreBackup posts to restore endpoint', async () => {
    const spy = vi.spyOn(client, 'post').mockResolvedValueOnce({})
    await restoreBackup('b3.zip')
    expect(spy).toHaveBeenCalledWith('/backups/b3.zip/restore')
  })

  it('deleteBackup deletes backup', async () => {
    const spy = vi.spyOn(client, 'delete').mockResolvedValueOnce({})
    await deleteBackup('b3.zip')
    expect(spy).toHaveBeenCalledWith('/backups/b3.zip')
  })
})
