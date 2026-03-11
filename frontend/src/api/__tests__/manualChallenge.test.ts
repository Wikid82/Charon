import { describe, it, expect, vi, beforeEach } from 'vitest'

import client from '../client'
import {
  getChallenge,
  createChallenge,
  verifyChallenge,
  pollChallenge,
  deleteChallenge,
} from '../manualChallenge'

vi.mock('../client', () => ({
  default: {
    get: vi.fn(),
    post: vi.fn(),
    delete: vi.fn(),
  },
}))

describe('manualChallenge API', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  describe('getChallenge', () => {
    it('fetches challenge by provider and challenge ID', async () => {
      const mockChallenge = {
        id: 'challenge-uuid',
        status: 'pending',
        fqdn: '_acme-challenge.example.com',
        value: 'test-value',
        ttl: 300,
        created_at: '2026-01-11T00:00:00Z',
        expires_at: '2026-01-11T00:10:00Z',
        dns_propagated: false,
      }

      vi.mocked(client.get).mockResolvedValueOnce({ data: mockChallenge })

      const result = await getChallenge(1, 'challenge-uuid')

      expect(client.get).toHaveBeenCalledWith(
        '/dns-providers/1/manual-challenge/challenge-uuid'
      )
      expect(result).toEqual(mockChallenge)
    })

    it('throws error when challenge not found', async () => {
      vi.mocked(client.get).mockRejectedValueOnce({
        response: { status: 404, data: { error: 'Challenge not found' } },
      })

      await expect(getChallenge(1, 'invalid-uuid')).rejects.toMatchObject({
        response: { status: 404 },
      })
    })
  })

  describe('createChallenge', () => {
    it('creates a new challenge for the provider', async () => {
      const mockChallenge = {
        id: 'new-challenge-uuid',
        status: 'created',
        fqdn: '_acme-challenge.example.com',
        value: 'generated-value',
        ttl: 300,
        created_at: '2026-01-11T00:00:00Z',
        expires_at: '2026-01-11T00:10:00Z',
        dns_propagated: false,
      }

      vi.mocked(client.post).mockResolvedValueOnce({ data: mockChallenge })

      const result = await createChallenge(1, { domain: 'example.com' })

      expect(client.post).toHaveBeenCalledWith('/dns-providers/1/manual-challenge', {
        domain: 'example.com',
      })
      expect(result).toEqual(mockChallenge)
    })

    it('throws error when provider not found', async () => {
      vi.mocked(client.post).mockRejectedValueOnce({
        response: { status: 404, data: { error: 'Provider not found' } },
      })

      await expect(createChallenge(999, { domain: 'example.com' })).rejects.toMatchObject({
        response: { status: 404 },
      })
    })

    it('throws error when challenge already in progress', async () => {
      vi.mocked(client.post).mockRejectedValueOnce({
        response: { status: 409, data: { code: 'CHALLENGE_IN_PROGRESS' } },
      })

      await expect(createChallenge(1, { domain: 'example.com' })).rejects.toMatchObject({
        response: { status: 409 },
      })
    })
  })

  describe('verifyChallenge', () => {
    it('triggers verification for a challenge', async () => {
      const mockResult = {
        success: true,
        dns_found: true,
        message: 'TXT record verified successfully',
      }

      vi.mocked(client.post).mockResolvedValueOnce({ data: mockResult })

      const result = await verifyChallenge(1, 'challenge-uuid')

      expect(client.post).toHaveBeenCalledWith(
        '/dns-providers/1/manual-challenge/challenge-uuid/verify'
      )
      expect(result).toEqual(mockResult)
    })

    it('returns dns_found false when record not propagated', async () => {
      const mockResult = {
        success: false,
        dns_found: false,
        message: 'DNS record not found',
      }

      vi.mocked(client.post).mockResolvedValueOnce({ data: mockResult })

      const result = await verifyChallenge(1, 'challenge-uuid')

      expect(result.success).toBe(false)
      expect(result.dns_found).toBe(false)
    })

    it('throws error when challenge expired', async () => {
      vi.mocked(client.post).mockRejectedValueOnce({
        response: { status: 410, data: { code: 'CHALLENGE_EXPIRED' } },
      })

      await expect(verifyChallenge(1, 'challenge-uuid')).rejects.toMatchObject({
        response: { status: 410 },
      })
    })
  })

  describe('pollChallenge', () => {
    it('returns current challenge status', async () => {
      const mockPoll = {
        status: 'pending',
        dns_propagated: false,
        time_remaining_seconds: 480,
        last_check_at: '2026-01-11T00:02:00Z',
      }

      vi.mocked(client.get).mockResolvedValueOnce({ data: mockPoll })

      const result = await pollChallenge(1, 'challenge-uuid')

      expect(client.get).toHaveBeenCalledWith(
        '/dns-providers/1/manual-challenge/challenge-uuid/poll'
      )
      expect(result).toEqual(mockPoll)
    })

    it('returns verified status when DNS propagated', async () => {
      const mockPoll = {
        status: 'verified',
        dns_propagated: true,
        time_remaining_seconds: 0,
        last_check_at: '2026-01-11T00:05:00Z',
      }

      vi.mocked(client.get).mockResolvedValueOnce({ data: mockPoll })

      const result = await pollChallenge(1, 'challenge-uuid')

      expect(result.status).toBe('verified')
      expect(result.dns_propagated).toBe(true)
    })

    it('includes error message when challenge failed', async () => {
      const mockPoll = {
        status: 'failed',
        dns_propagated: false,
        time_remaining_seconds: 0,
        last_check_at: '2026-01-11T00:05:00Z',
        error_message: 'ACME validation failed',
      }

      vi.mocked(client.get).mockResolvedValueOnce({ data: mockPoll })

      const result = await pollChallenge(1, 'challenge-uuid')

      expect(result.status).toBe('failed')
      expect(result.error_message).toBe('ACME validation failed')
    })
  })

  describe('deleteChallenge', () => {
    it('deletes/cancels a challenge', async () => {
      vi.mocked(client.delete).mockResolvedValueOnce({ data: undefined })

      await deleteChallenge(1, 'challenge-uuid')

      expect(client.delete).toHaveBeenCalledWith(
        '/dns-providers/1/manual-challenge/challenge-uuid'
      )
    })

    it('throws error when challenge not found', async () => {
      vi.mocked(client.delete).mockRejectedValueOnce({
        response: { status: 404, data: { error: 'Challenge not found' } },
      })

      await expect(deleteChallenge(1, 'invalid-uuid')).rejects.toMatchObject({
        response: { status: 404 },
      })
    })

    it('throws error when unauthorized', async () => {
      vi.mocked(client.delete).mockRejectedValueOnce({
        response: { status: 403, data: { error: 'Unauthorized' } },
      })

      await expect(deleteChallenge(1, 'challenge-uuid')).rejects.toMatchObject({
        response: { status: 403 },
      })
    })
  })
})
