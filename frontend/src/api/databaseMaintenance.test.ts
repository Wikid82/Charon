import { beforeEach, describe, expect, it, vi } from 'vitest'

import client from './client'
import {
  cancelOptimizeOnRestart,
  getDatabaseStatus,
  getMaintenanceStatus,
  requestOptimizeOnRestart,
} from './databaseMaintenance'

vi.mock('./client', () => ({
  default: { get: vi.fn(), post: vi.fn(), delete: vi.fn() },
}))

const mocked = client as unknown as {
  get: ReturnType<typeof vi.fn>
  post: ReturnType<typeof vi.fn>
  delete: ReturnType<typeof vi.fn>
}

describe('databaseMaintenance api', () => {
  beforeEach(() => vi.clearAllMocks())

  it('GETs /system/database', async () => {
    const body = { size_bytes: 1, notice: null }
    mocked.get.mockResolvedValue({ data: body })
    await expect(getDatabaseStatus()).resolves.toBe(body)
    expect(mocked.get).toHaveBeenCalledWith('/system/database')
  })

  it('POSTs the optimize-on-restart flag', async () => {
    mocked.post.mockResolvedValue({ data: { requested: true } })
    await expect(requestOptimizeOnRestart()).resolves.toEqual({ requested: true })
    expect(mocked.post).toHaveBeenCalledWith('/system/database/optimize-on-restart')
  })

  it('DELETEs the optimize-on-restart flag', async () => {
    mocked.delete.mockResolvedValue({ data: { requested: false } })
    await expect(cancelOptimizeOnRestart()).resolves.toEqual({ requested: false })
    expect(mocked.delete).toHaveBeenCalledWith('/system/database/optimize-on-restart')
  })

  it('GETs /maintenance/status', async () => {
    const body = { active: true, phase: 'converting', elapsed_seconds: 3 }
    mocked.get.mockResolvedValue({ data: body })
    await expect(getMaintenanceStatus()).resolves.toBe(body)
    expect(mocked.get).toHaveBeenCalledWith('/maintenance/status')
  })

  it('propagates request errors', async () => {
    mocked.post.mockRejectedValue(new Error('409'))
    await expect(requestOptimizeOnRestart()).rejects.toThrow('409')
  })
})
