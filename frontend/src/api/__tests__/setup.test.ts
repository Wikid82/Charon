import { describe, it, expect, vi, beforeEach } from 'vitest'
import client from '../../api/client'
import { getSetupStatus, performSetup } from '../setup'

describe('setup api', () => {
  beforeEach(() => {
    vi.restoreAllMocks()
  })

  it('getSetupStatus returns status', async () => {
    const data = { setupRequired: true }
    vi.spyOn(client, 'get').mockResolvedValueOnce({ data })
    const res = await getSetupStatus()
    expect(res).toEqual(data)
  })

  it('performSetup posts data to setup endpoint', async () => {
    const spy = vi.spyOn(client, 'post').mockResolvedValueOnce({ data: {} })
    const payload = { name: 'Admin', email: 'admin@example.com', password: 'secret' }
    await performSetup(payload)
    expect(spy).toHaveBeenCalledWith('/setup', payload)
  })
})
