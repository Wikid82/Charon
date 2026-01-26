import { describe, it, expect, vi, beforeEach } from 'vitest'
import {
  getCredentials,
  getCredential,
  createCredential,
  updateCredential,
  deleteCredential,
  testCredential,
  enableMultiCredentials,
  type DNSProviderCredential,
  type CredentialRequest,
  type CredentialTestResult,
} from '../credentials'
import client from '../client'

vi.mock('../client')

const mockCredential: DNSProviderCredential = {
  id: 1,
  uuid: 'test-uuid-1',
  dns_provider_id: 1,
  label: 'Production Credentials',
  zone_filter: '*.example.com',
  enabled: true,
  propagation_timeout: 120,
  polling_interval: 2,
  key_version: 1,
  success_count: 5,
  failure_count: 0,
  created_at: '2025-01-01T00:00:00Z',
  updated_at: '2025-01-01T00:00:00Z',
}

const mockCredentialRequest: CredentialRequest = {
  label: 'New Credentials',
  zone_filter: '*.example.com',
  credentials: { api_token: 'test-token-123' },
  propagation_timeout: 120,
  polling_interval: 2,
  enabled: true,
}

describe('credentials API', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('should call getCredentials with correct endpoint', async () => {
    const mockData = [mockCredential, { ...mockCredential, id: 2, label: 'Secondary' }]
    vi.mocked(client.get).mockResolvedValue({
      data: { credentials: mockData, total: 2 },
    })

    const result = await getCredentials(1)

    expect(client.get).toHaveBeenCalledWith('/dns-providers/1/credentials')
    expect(result).toEqual(mockData)
    expect(result).toHaveLength(2)
  })

  it('should call getCredential with correct endpoint', async () => {
    vi.mocked(client.get).mockResolvedValue({ data: mockCredential })

    const result = await getCredential(1, 1)

    expect(client.get).toHaveBeenCalledWith('/dns-providers/1/credentials/1')
    expect(result).toEqual(mockCredential)
  })

  it('should call createCredential with correct endpoint and data', async () => {
    vi.mocked(client.post).mockResolvedValue({ data: mockCredential })

    const result = await createCredential(1, mockCredentialRequest)

    expect(client.post).toHaveBeenCalledWith('/dns-providers/1/credentials', mockCredentialRequest)
    expect(result).toEqual(mockCredential)
  })

  it('should call updateCredential with correct endpoint and data', async () => {
    const updatedCredential = { ...mockCredential, label: 'Updated Label' }
    vi.mocked(client.put).mockResolvedValue({ data: updatedCredential })

    const result = await updateCredential(1, 1, mockCredentialRequest)

    expect(client.put).toHaveBeenCalledWith('/dns-providers/1/credentials/1', mockCredentialRequest)
    expect(result).toEqual(updatedCredential)
  })

  it('should call deleteCredential with correct endpoint', async () => {
    vi.mocked(client.delete).mockResolvedValue({ data: undefined })

    await deleteCredential(1, 1)

    expect(client.delete).toHaveBeenCalledWith('/dns-providers/1/credentials/1')
  })

  it('should call testCredential with correct endpoint', async () => {
    const mockTestResult: CredentialTestResult = {
      success: true,
      message: 'Credentials validated successfully',
      propagation_time_ms: 1200,
    }
    vi.mocked(client.post).mockResolvedValue({ data: mockTestResult })

    const result = await testCredential(1, 1)

    expect(client.post).toHaveBeenCalledWith('/dns-providers/1/credentials/1/test')
    expect(result).toEqual(mockTestResult)
    expect(result.success).toBe(true)
  })

  it('should call enableMultiCredentials with correct endpoint', async () => {
    vi.mocked(client.post).mockResolvedValue({ data: undefined })

    await enableMultiCredentials(1)

    expect(client.post).toHaveBeenCalledWith('/dns-providers/1/enable-multi-credentials')
  })
})
