import { beforeEach, describe, expect, it, vi } from 'vitest'

import client from '../client'
import { getProfile, regenerateApiKey, updateProfile } from '../users'

vi.mock('../client', () => ({
  default: {
    get: vi.fn(),
    post: vi.fn(),
  },
}))

describe('user api', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('fetches profile using masked API key fields', async () => {
    vi.mocked(client.get).mockResolvedValueOnce({
      data: {
        id: 1,
        email: 'admin@example.com',
        name: 'Admin',
        role: 'admin',
        has_api_key: true,
        api_key_masked: '********',
      },
    })

    const profile = await getProfile()

    expect(client.get).toHaveBeenCalledWith('/user/profile')
    expect(profile.has_api_key).toBe(true)
    expect(profile.api_key_masked).toBe('********')
  })

  it('regenerates API key and returns metadata-only response', async () => {
    vi.mocked(client.post).mockResolvedValueOnce({
      data: {
        message: 'API key regenerated successfully',
        has_api_key: true,
        api_key_masked: '********',
        api_key_updated: '2026-02-25T00:00:00Z',
      },
    })

    const result = await regenerateApiKey()

    expect(client.post).toHaveBeenCalledWith('/user/api-key')
    expect(result.has_api_key).toBe(true)
    expect(result.api_key_masked).toBe('********')
    expect(result.api_key_updated).toBe('2026-02-25T00:00:00Z')
  })

  it('updates profile with optional current password', async () => {
    vi.mocked(client.post).mockResolvedValueOnce({ data: { message: 'ok' } })

    await updateProfile({
      name: 'Updated Name',
      email: 'updated@example.com',
      current_password: 'current-password',
    })

    expect(client.post).toHaveBeenCalledWith('/user/profile', {
      name: 'Updated Name',
      email: 'updated@example.com',
      current_password: 'current-password',
    })
  })
})
