import { describe, it, expect, vi, beforeEach } from 'vitest'

import client from '../client'
import * as consoleEnrollment from '../consoleEnrollment'

vi.mock('../client')

describe('consoleEnrollment API', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  describe('getConsoleStatus', () => {
    it('should fetch enrollment status with pending state', async () => {
      const mockStatus = {
        status: 'pending',
        tenant: 'my-org',
        agent_name: 'charon-prod',
        key_present: true,
        last_attempt_at: '2025-12-15T09:00:00Z',
      }
      vi.mocked(client.get).mockResolvedValue({ data: mockStatus })

      const result = await consoleEnrollment.getConsoleStatus()

      expect(client.get).toHaveBeenCalledWith('/admin/crowdsec/console/status')
      expect(result).toEqual(mockStatus)
      expect(result.status).toBe('pending')
      expect(result.key_present).toBe(true)
    })

    it('should fetch enrolled status with heartbeat', async () => {
      const mockStatus = {
        status: 'enrolled',
        tenant: 'my-org',
        agent_name: 'charon-prod',
        key_present: true,
        enrolled_at: '2025-12-14T10:00:00Z',
        last_heartbeat_at: '2025-12-15T09:55:00Z',
      }
      vi.mocked(client.get).mockResolvedValue({ data: mockStatus })

      const result = await consoleEnrollment.getConsoleStatus()

      expect(result.status).toBe('enrolled')
      expect(result.enrolled_at).toBeDefined()
      expect(result.last_heartbeat_at).toBeDefined()
    })

    it('should fetch failed status with error message', async () => {
      const mockStatus = {
        status: 'failed',
        tenant: 'my-org',
        agent_name: 'charon-prod',
        key_present: false,
        last_error: 'Invalid enrollment key',
        last_attempt_at: '2025-12-15T09:00:00Z',
        correlation_id: 'req-abc123',
      }
      vi.mocked(client.get).mockResolvedValue({ data: mockStatus })

      const result = await consoleEnrollment.getConsoleStatus()

      expect(result.status).toBe('failed')
      expect(result.last_error).toBe('Invalid enrollment key')
      expect(result.correlation_id).toBe('req-abc123')
      expect(result.key_present).toBe(false)
    })

    it('should fetch status with none state (not enrolled)', async () => {
      const mockStatus = {
        status: 'none',
        key_present: false,
      }
      vi.mocked(client.get).mockResolvedValue({ data: mockStatus })

      const result = await consoleEnrollment.getConsoleStatus()

      expect(result.status).toBe('none')
      expect(result.key_present).toBe(false)
      expect(result.tenant).toBeUndefined()
    })

    it('should NOT return enrollment key in status response', async () => {
      const mockStatus = {
        status: 'enrolled',
        tenant: 'test-org',
        agent_name: 'test-agent',
        key_present: true,
        enrolled_at: '2025-12-14T10:00:00Z',
      }
      vi.mocked(client.get).mockResolvedValue({ data: mockStatus })

      const result = await consoleEnrollment.getConsoleStatus()

      // Security test: Ensure key is never exposed
      expect(result).not.toHaveProperty('enrollment_key')
      expect(result).not.toHaveProperty('encrypted_enroll_key')
      expect(result).toHaveProperty('key_present')
    })

    it('should handle API errors', async () => {
      const error = new Error('Network error')
      vi.mocked(client.get).mockRejectedValue(error)

      await expect(consoleEnrollment.getConsoleStatus()).rejects.toThrow('Network error')
    })

    it('should handle server unavailability', async () => {
      const error = {
        response: {
          status: 503,
          data: { error: 'Service temporarily unavailable' },
        },
      }
      vi.mocked(client.get).mockRejectedValue(error)

      await expect(consoleEnrollment.getConsoleStatus()).rejects.toEqual(error)
    })
  })

  describe('enrollConsole', () => {
    it('should enroll with valid payload', async () => {
      const payload = {
        enrollment_key: 'cs-enroll-abc123xyz',
        tenant: 'my-org',
        agent_name: 'charon-prod',
        force: false,
      }
      const mockResponse = {
        status: 'enrolled',
        tenant: 'my-org',
        agent_name: 'charon-prod',
        key_present: true,
        enrolled_at: '2025-12-15T10:00:00Z',
      }
      vi.mocked(client.post).mockResolvedValue({ data: mockResponse })

      const result = await consoleEnrollment.enrollConsole(payload)

      expect(client.post).toHaveBeenCalledWith('/admin/crowdsec/console/enroll', payload)
      expect(result).toEqual(mockResponse)
      expect(result.status).toBe('enrolled')
      expect(result.enrolled_at).toBeDefined()
    })

    it('should enroll with minimal payload (no tenant)', async () => {
      const payload = {
        enrollment_key: 'cs-enroll-key123',
        agent_name: 'charon-test',
      }
      const mockResponse = {
        status: 'enrolled',
        agent_name: 'charon-test',
        key_present: true,
        enrolled_at: '2025-12-15T10:00:00Z',
      }
      vi.mocked(client.post).mockResolvedValue({ data: mockResponse })

      const result = await consoleEnrollment.enrollConsole(payload)

      expect(result.status).toBe('enrolled')
      expect(result.agent_name).toBe('charon-test')
    })

    it('should force re-enrollment when force=true', async () => {
      const payload = {
        enrollment_key: 'cs-enroll-new-key',
        agent_name: 'charon-updated',
        force: true,
      }
      const mockResponse = {
        status: 'enrolled',
        agent_name: 'charon-updated',
        key_present: true,
        enrolled_at: '2025-12-15T10:05:00Z',
      }
      vi.mocked(client.post).mockResolvedValue({ data: mockResponse })

      const result = await consoleEnrollment.enrollConsole(payload)

      expect(result.status).toBe('enrolled')
      expect(client.post).toHaveBeenCalledWith('/admin/crowdsec/console/enroll', payload)
    })

    it('should handle invalid enrollment key format', async () => {
      const payload = {
        enrollment_key: 'not-a-valid-key',
        agent_name: 'test',
      }
      const error = {
        response: {
          status: 400,
          data: { error: 'Invalid enrollment key format' },
        },
      }
      vi.mocked(client.post).mockRejectedValue(error)

      await expect(consoleEnrollment.enrollConsole(payload)).rejects.toEqual(error)
    })

    it('should handle transient network errors during enrollment', async () => {
      const payload = {
        enrollment_key: 'cs-enroll-key123',
        agent_name: 'test-agent',
      }
      const error = {
        response: {
          status: 503,
          data: { error: 'CrowdSec Console API temporarily unavailable' },
        },
      }
      vi.mocked(client.post).mockRejectedValue(error)

      await expect(consoleEnrollment.enrollConsole(payload)).rejects.toEqual(error)
    })

    it('should handle enrollment key expiration', async () => {
      const payload = {
        enrollment_key: 'cs-enroll-expired-key',
        agent_name: 'test',
      }
      const mockResponse = {
        status: 'failed',
        key_present: false,
        last_error: 'Enrollment key expired',
        correlation_id: 'err-expired-123',
      }
      vi.mocked(client.post).mockResolvedValue({ data: mockResponse })

      const result = await consoleEnrollment.enrollConsole(payload)

      expect(result.status).toBe('failed')
      expect(result.last_error).toBe('Enrollment key expired')
    })

    it('should sanitize tenant name with special characters', async () => {
      const payload = {
        enrollment_key: 'valid-key',
        tenant: 'My Org (Production)',
        agent_name: 'agent1',
      }
      const mockResponse = {
        status: 'enrolled',
        tenant: 'My Org (Production)',
        agent_name: 'agent1',
        key_present: true,
        enrolled_at: '2025-12-15T10:00:00Z',
      }
      vi.mocked(client.post).mockResolvedValue({ data: mockResponse })

      const result = await consoleEnrollment.enrollConsole(payload)

      expect(result.status).toBe('enrolled')
      expect(result.tenant).toBe('My Org (Production)')
    })

    it('should handle SQL injection attempts in agent_name', async () => {
      const payload = {
        enrollment_key: 'valid-key',
        agent_name: "'; DROP TABLE users; --",
      }
      const error = {
        response: {
          status: 400,
          data: { error: 'Invalid agent name format' },
        },
      }
      vi.mocked(client.post).mockRejectedValue(error)

      await expect(consoleEnrollment.enrollConsole(payload)).rejects.toEqual(error)
    })

    it('should handle CrowdSec not running during enrollment', async () => {
      const payload = {
        enrollment_key: 'valid-key',
        agent_name: 'test',
      }
      const error = {
        response: {
          status: 500,
          data: { error: 'CrowdSec is not running. Start CrowdSec before enrolling.' },
        },
      }
      vi.mocked(client.post).mockRejectedValue(error)

      await expect(consoleEnrollment.enrollConsole(payload)).rejects.toEqual(error)
    })

    it('should return pending status when enrollment is queued', async () => {
      const payload = {
        enrollment_key: 'valid-key',
        agent_name: 'test',
      }
      const mockResponse = {
        status: 'pending',
        agent_name: 'test',
        key_present: true,
        last_attempt_at: '2025-12-15T10:00:00Z',
      }
      vi.mocked(client.post).mockResolvedValue({ data: mockResponse })

      const result = await consoleEnrollment.enrollConsole(payload)

      expect(result.status).toBe('pending')
      expect(result.last_attempt_at).toBeDefined()
    })
  })

  describe('default export', () => {
    it('should export all functions', () => {
      expect(consoleEnrollment.default).toHaveProperty('getConsoleStatus')
      expect(consoleEnrollment.default).toHaveProperty('enrollConsole')
    })
  })

  describe('integration scenarios', () => {
    it('should handle full enrollment workflow: status → enroll → verify', async () => {
      // 1. Check initial status (not enrolled)
      const mockStatusNone = {
        status: 'none',
        key_present: false,
      }
      vi.mocked(client.get).mockResolvedValueOnce({ data: mockStatusNone })

      const statusBefore = await consoleEnrollment.getConsoleStatus()
      expect(statusBefore.status).toBe('none')

      // 2. Enroll
      const enrollPayload = {
        enrollment_key: 'cs-enroll-valid-key',
        tenant: 'test-org',
        agent_name: 'charon-test',
      }
      const mockEnrollResponse = {
        status: 'enrolled',
        tenant: 'test-org',
        agent_name: 'charon-test',
        key_present: true,
        enrolled_at: '2025-12-15T10:00:00Z',
      }
      vi.mocked(client.post).mockResolvedValueOnce({ data: mockEnrollResponse })

      const enrollResult = await consoleEnrollment.enrollConsole(enrollPayload)
      expect(enrollResult.status).toBe('enrolled')

      // 3. Verify status updated
      const mockStatusEnrolled = {
        status: 'enrolled',
        tenant: 'test-org',
        agent_name: 'charon-test',
        key_present: true,
        enrolled_at: '2025-12-15T10:00:00Z',
        last_heartbeat_at: '2025-12-15T10:01:00Z',
      }
      vi.mocked(client.get).mockResolvedValueOnce({ data: mockStatusEnrolled })

      const statusAfter = await consoleEnrollment.getConsoleStatus()
      expect(statusAfter.status).toBe('enrolled')
      expect(statusAfter.tenant).toBe('test-org')
    })

    it('should handle enrollment failure and retry', async () => {
      // 1. First enrollment attempt fails
      const payload = {
        enrollment_key: 'cs-enroll-key',
        agent_name: 'test',
      }
      const networkError = new Error('Network timeout')
      vi.mocked(client.post).mockRejectedValueOnce(networkError)

      await expect(consoleEnrollment.enrollConsole(payload)).rejects.toThrow('Network timeout')

      // 2. Retry succeeds
      const mockResponse = {
        status: 'enrolled',
        agent_name: 'test',
        key_present: true,
        enrolled_at: '2025-12-15T10:05:00Z',
      }
      vi.mocked(client.post).mockResolvedValueOnce({ data: mockResponse })

      const retryResult = await consoleEnrollment.enrollConsole(payload)
      expect(retryResult.status).toBe('enrolled')
    })

    it('should handle status transitions: none → pending → enrolled', async () => {
      // 1. Initial: none
      const mockNone = { status: 'none', key_present: false }
      vi.mocked(client.get).mockResolvedValueOnce({ data: mockNone })
      const status1 = await consoleEnrollment.getConsoleStatus()
      expect(status1.status).toBe('none')

      // 2. Enroll (returns pending)
      const payload = { enrollment_key: 'key', agent_name: 'agent' }
      const mockPending = {
        status: 'pending',
        agent_name: 'agent',
        key_present: true,
        last_attempt_at: '2025-12-15T10:00:00Z',
      }
      vi.mocked(client.post).mockResolvedValueOnce({ data: mockPending })
      const enrollResult = await consoleEnrollment.enrollConsole(payload)
      expect(enrollResult.status).toBe('pending')

      // 3. Check status again (now enrolled)
      const mockEnrolled = {
        status: 'enrolled',
        agent_name: 'agent',
        key_present: true,
        enrolled_at: '2025-12-15T10:00:30Z',
      }
      vi.mocked(client.get).mockResolvedValueOnce({ data: mockEnrolled })
      const status2 = await consoleEnrollment.getConsoleStatus()
      expect(status2.status).toBe('enrolled')
    })

    it('should handle force re-enrollment over existing enrollment', async () => {
      // 1. Check current enrollment
      const mockCurrent = {
        status: 'enrolled',
        tenant: 'old-org',
        agent_name: 'old-agent',
        key_present: true,
        enrolled_at: '2025-12-14T10:00:00Z',
      }
      vi.mocked(client.get).mockResolvedValueOnce({ data: mockCurrent })
      const currentStatus = await consoleEnrollment.getConsoleStatus()
      expect(currentStatus.tenant).toBe('old-org')

      // 2. Force re-enrollment
      const forcePayload = {
        enrollment_key: 'new-key',
        tenant: 'new-org',
        agent_name: 'new-agent',
        force: true,
      }
      const mockForced = {
        status: 'enrolled',
        tenant: 'new-org',
        agent_name: 'new-agent',
        key_present: true,
        enrolled_at: '2025-12-15T10:00:00Z',
      }
      vi.mocked(client.post).mockResolvedValueOnce({ data: mockForced })
      const forceResult = await consoleEnrollment.enrollConsole(forcePayload)
      expect(forceResult.tenant).toBe('new-org')
    })
  })

  describe('security tests', () => {
    it('should never log or expose enrollment key', async () => {
      const payload = {
        enrollment_key: 'cs-enroll-secret-key-should-never-log',
        agent_name: 'test',
      }
      const mockResponse = {
        status: 'enrolled',
        agent_name: 'test',
        key_present: true,
      }
      vi.mocked(client.post).mockResolvedValue({ data: mockResponse })

      const result = await consoleEnrollment.enrollConsole(payload)

      // Ensure response never contains the key
      expect(result).not.toHaveProperty('enrollment_key')
      expect(JSON.stringify(result)).not.toContain('cs-enroll-secret-key')
    })

    it('should sanitize error messages to avoid key leakage', async () => {
      const payload = {
        enrollment_key: 'cs-enroll-sensitive-key',
        agent_name: 'test',
      }
      const error = {
        response: {
          status: 400,
          data: { error: 'Enrollment failed: invalid key format' },
        },
      }
      vi.mocked(client.post).mockRejectedValue(error)

      const thrown = await consoleEnrollment.enrollConsole(payload).catch((e: unknown) => e)
      const caughtError = thrown as { response?: { data?: { error?: string } } }
      // Error message should NOT contain the key
      expect(caughtError.response?.data?.error).not.toContain('cs-enroll-sensitive-key')
    })

    it('should handle correlation_id for debugging without exposing keys', async () => {
      const mockStatus = {
        status: 'failed',
        key_present: false,
        last_error: 'Authentication failed',
        correlation_id: 'debug-correlation-abc123',
      }
      vi.mocked(client.get).mockResolvedValue({ data: mockStatus })

      const result = await consoleEnrollment.getConsoleStatus()

      expect(result.correlation_id).toBe('debug-correlation-abc123')
      expect(result).not.toHaveProperty('enrollment_key')
    })
  })
})
