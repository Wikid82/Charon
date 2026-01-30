import { describe, it, expect, vi, beforeEach } from 'vitest'
import {
  getDNSProviders,
  getDNSProvider,
  getDNSProviderTypes,
  createDNSProvider,
  updateDNSProvider,
  deleteDNSProvider,
  testDNSProvider,
  testDNSProviderCredentials,
  type DNSProvider,
  type DNSProviderRequest,
  type DNSProviderTypeInfo,
} from '../dnsProviders'
import client from '../client'

vi.mock('../client')

const mockProvider: DNSProvider = {
  id: 1,
  uuid: 'test-uuid-1',
  name: 'Cloudflare Production',
  provider_type: 'cloudflare',
  enabled: true,
  is_default: true,
  has_credentials: true,
  propagation_timeout: 120,
  polling_interval: 2,
  success_count: 5,
  failure_count: 0,
  created_at: '2025-01-01T00:00:00Z',
  updated_at: '2025-01-01T00:00:00Z',
}

const mockProviderType: DNSProviderTypeInfo = {
  type: 'cloudflare',
  name: 'Cloudflare',
  fields: [
    {
      name: 'api_token',
      label: 'API Token',
      type: 'password',
      required: true,
      hint: 'Cloudflare API token with DNS edit permissions',
    },
  ],
  documentation_url: 'https://developers.cloudflare.com/api/',
}

describe('getDNSProviders', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('fetches all DNS providers successfully', async () => {
    const mockProviders = [mockProvider, { ...mockProvider, id: 2, name: 'Secondary' }]
    vi.mocked(client.get).mockResolvedValue({
      data: { providers: mockProviders, total: 2 },
    })

    const result = await getDNSProviders()

    expect(client.get).toHaveBeenCalledWith('/dns-providers')
    expect(result).toEqual(mockProviders)
    expect(result).toHaveLength(2)
  })

  it('returns empty array when no providers exist', async () => {
    vi.mocked(client.get).mockResolvedValue({
      data: { providers: [], total: 0 },
    })

    const result = await getDNSProviders()

    expect(result).toEqual([])
  })

  it('handles network errors', async () => {
    vi.mocked(client.get).mockRejectedValue(new Error('Network error'))

    await expect(getDNSProviders()).rejects.toThrow('Network error')
  })

  it('handles server errors', async () => {
    vi.mocked(client.get).mockRejectedValue({ response: { status: 500 } })

    await expect(getDNSProviders()).rejects.toMatchObject({
      response: { status: 500 },
    })
  })
})

describe('getDNSProvider', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('fetches single provider by valid ID', async () => {
    vi.mocked(client.get).mockResolvedValue({ data: mockProvider })

    const result = await getDNSProvider(1)

    expect(client.get).toHaveBeenCalledWith('/dns-providers/1')
    expect(result).toEqual(mockProvider)
  })

  it('handles not found error for invalid ID', async () => {
    vi.mocked(client.get).mockRejectedValue({ response: { status: 404 } })

    await expect(getDNSProvider(999)).rejects.toMatchObject({
      response: { status: 404 },
    })
  })

  it('handles server errors', async () => {
    vi.mocked(client.get).mockRejectedValue({ response: { status: 500 } })

    await expect(getDNSProvider(1)).rejects.toMatchObject({
      response: { status: 500 },
    })
  })
})

describe('getDNSProviderTypes', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('fetches supported provider types with field definitions', async () => {
    const mockTypes = [
      mockProviderType,
      {
        type: 'route53',
        name: 'AWS Route 53',
        fields: [
          { name: 'access_key_id', label: 'Access Key ID', type: 'text', required: true },
          { name: 'secret_access_key', label: 'Secret Access Key', type: 'password', required: true },
        ],
        documentation_url: 'https://aws.amazon.com/route53/',
      } as DNSProviderTypeInfo,
    ]
    vi.mocked(client.get).mockResolvedValue({
      data: { types: mockTypes },
    })

    const result = await getDNSProviderTypes()

    expect(client.get).toHaveBeenCalledWith('/dns-providers/types')
    expect(result).toEqual(mockTypes)
    expect(result).toHaveLength(2)
  })

  it('handles errors when fetching types', async () => {
    vi.mocked(client.get).mockRejectedValue(new Error('Failed to fetch types'))

    await expect(getDNSProviderTypes()).rejects.toThrow('Failed to fetch types')
  })
})

describe('createDNSProvider', () => {
  const validRequest: DNSProviderRequest = {
    name: 'New Cloudflare',
    provider_type: 'cloudflare',
    credentials: { api_token: 'test-token-123' },
    propagation_timeout: 120,
    polling_interval: 2,
  }

  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('creates provider successfully and returns with ID', async () => {
    const createdProvider = { ...mockProvider, id: 5, name: 'New Cloudflare' }
    vi.mocked(client.post).mockResolvedValue({ data: createdProvider })

    const result = await createDNSProvider(validRequest)

    expect(client.post).toHaveBeenCalledWith('/dns-providers', validRequest)
    expect(result).toEqual(createdProvider)
    expect(result.id).toBe(5)
  })

  it('handles validation error for missing required fields', async () => {
    vi.mocked(client.post).mockRejectedValue({
      response: { status: 400, data: { error: 'Missing required field: api_token' } },
    })

    await expect(
      createDNSProvider({ ...validRequest, credentials: {} })
    ).rejects.toMatchObject({
      response: { status: 400 },
    })
  })

  it('handles validation error for invalid provider type', async () => {
    vi.mocked(client.post).mockRejectedValue({
      response: { status: 400, data: { error: 'Invalid provider type' } },
    })

    await expect(
      createDNSProvider({ ...validRequest, provider_type: 'invalid' as DNSProviderRequest['provider_type'] })
    ).rejects.toMatchObject({
      response: { status: 400 },
    })
  })

  it('handles duplicate name error', async () => {
    vi.mocked(client.post).mockRejectedValue({
      response: { status: 409, data: { error: 'Provider with this name already exists' } },
    })

    await expect(createDNSProvider(validRequest)).rejects.toMatchObject({
      response: { status: 409 },
    })
  })

  it('handles server errors', async () => {
    vi.mocked(client.post).mockRejectedValue({ response: { status: 500 } })

    await expect(createDNSProvider(validRequest)).rejects.toMatchObject({
      response: { status: 500 },
    })
  })
})

describe('updateDNSProvider', () => {
  const updateRequest: DNSProviderRequest = {
    name: 'Updated Name',
    provider_type: 'cloudflare',
    credentials: { api_token: 'new-token' },
  }

  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('updates provider successfully', async () => {
    const updatedProvider = { ...mockProvider, name: 'Updated Name' }
    vi.mocked(client.put).mockResolvedValue({ data: updatedProvider })

    const result = await updateDNSProvider(1, updateRequest)

    expect(client.put).toHaveBeenCalledWith('/dns-providers/1', updateRequest)
    expect(result).toEqual(updatedProvider)
    expect(result.name).toBe('Updated Name')
  })

  it('handles not found error', async () => {
    vi.mocked(client.put).mockRejectedValue({ response: { status: 404 } })

    await expect(updateDNSProvider(999, updateRequest)).rejects.toMatchObject({
      response: { status: 404 },
    })
  })

  it('handles validation errors', async () => {
    vi.mocked(client.put).mockRejectedValue({
      response: { status: 400, data: { error: 'Invalid credentials' } },
    })

    await expect(updateDNSProvider(1, updateRequest)).rejects.toMatchObject({
      response: { status: 400 },
    })
  })

  it('handles server errors', async () => {
    vi.mocked(client.put).mockRejectedValue({ response: { status: 500 } })

    await expect(updateDNSProvider(1, updateRequest)).rejects.toMatchObject({
      response: { status: 500 },
    })
  })
})

describe('deleteDNSProvider', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('deletes provider successfully', async () => {
    vi.mocked(client.delete).mockResolvedValue({ data: undefined })

    await deleteDNSProvider(1)

    expect(client.delete).toHaveBeenCalledWith('/dns-providers/1')
  })

  it('handles not found error', async () => {
    vi.mocked(client.delete).mockRejectedValue({ response: { status: 404 } })

    await expect(deleteDNSProvider(999)).rejects.toMatchObject({
      response: { status: 404 },
    })
  })

  it('handles in-use error when provider used by proxy hosts', async () => {
    vi.mocked(client.delete).mockRejectedValue({
      response: {
        status: 409,
        data: { error: 'Cannot delete provider in use by proxy hosts' },
      },
    })

    await expect(deleteDNSProvider(1)).rejects.toMatchObject({
      response: { status: 409 },
    })
  })

  it('handles server errors', async () => {
    vi.mocked(client.delete).mockRejectedValue({ response: { status: 500 } })

    await expect(deleteDNSProvider(1)).rejects.toMatchObject({
      response: { status: 500 },
    })
  })
})

describe('testDNSProvider', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('returns success result with propagation time', async () => {
    const successResult = {
      success: true,
      message: 'DNS challenge completed successfully',
      propagation_time_ms: 1500,
    }
    vi.mocked(client.post).mockResolvedValue({ data: successResult })

    const result = await testDNSProvider(1)

    expect(client.post).toHaveBeenCalledWith('/dns-providers/1/test')
    expect(result).toEqual(successResult)
    expect(result.success).toBe(true)
    expect(result.propagation_time_ms).toBe(1500)
  })

  it('returns failure result with error message', async () => {
    const failureResult = {
      success: false,
      error: 'Invalid API token',
      code: 'AUTH_FAILED',
    }
    vi.mocked(client.post).mockResolvedValue({ data: failureResult })

    const result = await testDNSProvider(1)

    expect(result).toEqual(failureResult)
    expect(result.success).toBe(false)
    expect(result.error).toBe('Invalid API token')
  })

  it('handles not found error', async () => {
    vi.mocked(client.post).mockRejectedValue({ response: { status: 404 } })

    await expect(testDNSProvider(999)).rejects.toMatchObject({
      response: { status: 404 },
    })
  })

  it('handles server errors', async () => {
    vi.mocked(client.post).mockRejectedValue({ response: { status: 500 } })

    await expect(testDNSProvider(1)).rejects.toMatchObject({
      response: { status: 500 },
    })
  })
})

describe('testDNSProviderCredentials', () => {
  const testRequest: DNSProviderRequest = {
    name: 'Test Provider',
    provider_type: 'cloudflare',
    credentials: { api_token: 'test-token' },
  }

  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('returns success for valid credentials', async () => {
    const successResult = {
      success: true,
      message: 'Credentials validated successfully',
      propagation_time_ms: 800,
    }
    vi.mocked(client.post).mockResolvedValue({ data: successResult })

    const result = await testDNSProviderCredentials(testRequest)

    expect(client.post).toHaveBeenCalledWith('/dns-providers/test', testRequest)
    expect(result).toEqual(successResult)
    expect(result.success).toBe(true)
  })

  it('returns failure for invalid credentials', async () => {
    const failureResult = {
      success: false,
      error: 'Authentication failed',
      code: 'INVALID_CREDENTIALS',
    }
    vi.mocked(client.post).mockResolvedValue({ data: failureResult })

    const result = await testDNSProviderCredentials(testRequest)

    expect(result).toEqual(failureResult)
    expect(result.success).toBe(false)
  })

  it('handles validation errors for missing credentials', async () => {
    vi.mocked(client.post).mockRejectedValue({
      response: { status: 400, data: { error: 'Missing required field: api_token' } },
    })

    await expect(
      testDNSProviderCredentials({ ...testRequest, credentials: {} })
    ).rejects.toMatchObject({
      response: { status: 400 },
    })
  })

  it('handles server errors', async () => {
    vi.mocked(client.post).mockRejectedValue({ response: { status: 500 } })

    await expect(testDNSProviderCredentials(testRequest)).rejects.toMatchObject({
      response: { status: 500 },
    })
  })
})
