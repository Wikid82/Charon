import { describe, it, expect, vi, beforeEach } from 'vitest'

import client from '../client'
import {
  getEncryptionStatus,
  rotateEncryptionKey,
  getRotationHistory,
  validateKeyConfiguration,
  type RotationStatus,
  type RotationResult,
  type RotationHistoryEntry,
  type KeyValidationResult,
} from '../encryption'

vi.mock('../client')

const mockRotationStatus: RotationStatus = {
  current_version: 2,
  next_key_configured: true,
  legacy_key_count: 1,
  providers_on_current_version: 5,
  providers_on_older_versions: 0,
}

const mockRotationResult: RotationResult = {
  total_providers: 5,
  success_count: 5,
  failure_count: 0,
  duration: '2.5s',
  new_key_version: 3,
}

const mockHistoryEntry: RotationHistoryEntry = {
  id: 1,
  uuid: 'test-uuid-1',
  actor: 'admin@example.com',
  action: 'encryption_key_rotated',
  event_category: 'security',
  details: 'Rotated from version 1 to version 2',
  created_at: '2025-01-01T00:00:00Z',
}

const mockValidationResult: KeyValidationResult = {
  valid: true,
  message: 'Key configuration is valid',
}

describe('encryption API', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('should call getEncryptionStatus with correct endpoint', async () => {
    vi.mocked(client.get).mockResolvedValue({ data: mockRotationStatus })

    const result = await getEncryptionStatus()

    expect(client.get).toHaveBeenCalledWith('/admin/encryption/status')
    expect(result).toEqual(mockRotationStatus)
    expect(result.current_version).toBe(2)
  })

  it('should call rotateEncryptionKey with correct endpoint', async () => {
    vi.mocked(client.post).mockResolvedValue({ data: mockRotationResult })

    const result = await rotateEncryptionKey()

    expect(client.post).toHaveBeenCalledWith('/admin/encryption/rotate')
    expect(result).toEqual(mockRotationResult)
    expect(result.new_key_version).toBe(3)
    expect(result.success_count).toBe(5)
  })

  it('should call getRotationHistory with correct endpoint', async () => {
    const mockHistory = [mockHistoryEntry, { ...mockHistoryEntry, id: 2 }]
    vi.mocked(client.get).mockResolvedValue({
      data: { history: mockHistory, total: 2 },
    })

    const result = await getRotationHistory()

    expect(client.get).toHaveBeenCalledWith('/admin/encryption/history')
    expect(result).toEqual(mockHistory)
    expect(result).toHaveLength(2)
  })

  it('should call validateKeyConfiguration with correct endpoint', async () => {
    vi.mocked(client.post).mockResolvedValue({ data: mockValidationResult })

    const result = await validateKeyConfiguration()

    expect(client.post).toHaveBeenCalledWith('/admin/encryption/validate')
    expect(result).toEqual(mockValidationResult)
    expect(result.valid).toBe(true)
  })
})
