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
  next_version: 3,
  legacy_key_count: 1,
  legacy_key_versions: [1],
  keys: [
    { slot: 'CHARON_ENCRYPTION_KEY', version: 2, key_id: '9f2c41aa', in_use: 5, removable: false, loaded: true },
    { slot: 'CHARON_ENCRYPTION_KEY_V1', version: 1, key_id: 'c0ffee12', in_use: 0, removable: true, loaded: true },
  ],
  stores: [{ store: 'dns_providers', total: 5, by_version: { '2': 5 }, unreadable: 0 }],
  unreadable_total: 0,
  rotation_needed: false,
  providers_on_current_version: 5,
  providers_on_older_versions: 0,
  providers_by_version: { '2': 5 },
}

const mockRotationResult: RotationResult = {
  target_version: 3,
  stores: [{ store: 'dns_providers', total: 5, rotated: 5, already_current: 0, unreadable: 0 }],
  total_records: 5,
  rotated_count: 5,
  already_current_count: 0,
  unreadable_count: 0,
  new_key_version: 3,
  duration: '2.5s',
  started_at: '2025-01-01T00:00:00Z',
  completed_at: '2025-01-01T00:00:03Z',
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

  it('returns per-key and per-store usage from the status endpoint', async () => {
    vi.mocked(client.get).mockResolvedValue({ data: mockRotationStatus })

    const result = await getEncryptionStatus()

    expect(result.keys).toHaveLength(2)
    expect(result.keys[1]).toMatchObject({ slot: 'CHARON_ENCRYPTION_KEY_V1', removable: true, loaded: true })
    expect(result.stores[0]?.by_version).toEqual({ '2': 5 })
    expect(result.rotation_needed).toBe(false)
    expect(result.providers_on_current_version).toBe(5)
  })

  it('should call rotateEncryptionKey with correct endpoint', async () => {
    vi.mocked(client.post).mockResolvedValue({ data: mockRotationResult })

    const result = await rotateEncryptionKey()

    expect(client.post).toHaveBeenCalledWith('/admin/encryption/rotate')
    expect(result).toEqual(mockRotationResult)
    expect(result.new_key_version).toBe(3)
    expect(result.rotated_count).toBe(5)
    expect(result.stores[0]?.store).toBe('dns_providers')
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
