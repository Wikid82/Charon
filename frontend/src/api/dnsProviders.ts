import client from './client'

/** Supported DNS provider types */
export type DNSProviderType =
  | 'cloudflare'
  | 'route53'
  | 'digitalocean'
  | 'googleclouddns'
  | 'namecheap'
  | 'godaddy'
  | 'azure'
  | 'hetzner'
  | 'vultr'
  | 'dnsimple'
  // Custom plugin types
  | 'manual'
  | 'webhook'
  | 'rfc2136'
  | 'script'

/** Represents a configured DNS provider */
export interface DNSProvider {
  id: number
  uuid: string
  name: string
  provider_type: DNSProviderType
  enabled: boolean
  is_default: boolean
  has_credentials: boolean
  propagation_timeout: number
  polling_interval: number
  last_used_at?: string
  success_count: number
  failure_count: number
  last_error?: string
  created_at: string
  updated_at: string
}

/** Request payload for creating/updating DNS providers */
export interface DNSProviderRequest {
  name: string
  provider_type: DNSProviderType
  credentials: Record<string, string>
  propagation_timeout?: number
  polling_interval?: number
  is_default?: boolean
}

/** DNS provider test result */
export interface DNSTestResult {
  success: boolean
  message?: string
  error?: string
  code?: string
  propagation_time_ms?: number
}

/** Field definition for DNS provider credentials */
export interface DNSProviderField {
  name: string
  label: string
  type: 'text' | 'password' | 'textarea' | 'select'
  required: boolean
  default?: string
  hint?: string
  placeholder?: string
  options?: Array<{
    value: string
    label: string
  }>
}

/** DNS provider type information with field definitions */
export interface DNSProviderTypeInfo {
  type: DNSProviderType
  name: string
  description?: string
  documentation_url?: string
  is_built_in?: boolean
  fields: DNSProviderField[]
}

/** Response for list endpoint */
interface ListDNSProvidersResponse {
  providers: DNSProvider[]
  total: number
}

/** Response for types endpoint */
interface DNSProviderTypesResponse {
  types: DNSProviderTypeInfo[]
}

/**
 * Fetches all configured DNS providers.
 * @returns Promise resolving to array of DNS providers
 * @throws {AxiosError} If the request fails
 */
export async function getDNSProviders(): Promise<DNSProvider[]> {
  const response = await client.get<ListDNSProvidersResponse>('/dns-providers')
  return response.data.providers
}

/**
 * Fetches a single DNS provider by ID.
 * @param id - The DNS provider ID
 * @returns Promise resolving to the DNS provider
 * @throws {AxiosError} If not found or request fails
 */
export async function getDNSProvider(id: number): Promise<DNSProvider> {
  const response = await client.get<DNSProvider>(`/dns-providers/${id}`)
  return response.data
}

/**
 * Creates a new DNS provider.
 * @param data - DNS provider configuration
 * @returns Promise resolving to the created provider
 * @throws {AxiosError} If validation fails or request fails
 */
export async function createDNSProvider(data: DNSProviderRequest): Promise<DNSProvider> {
  const response = await client.post<DNSProvider>('/dns-providers', data)
  return response.data
}

/**
 * Updates an existing DNS provider.
 * @param id - The DNS provider ID
 * @param data - Updated configuration
 * @returns Promise resolving to the updated provider
 * @throws {AxiosError} If not found, validation fails, or request fails
 */
export async function updateDNSProvider(id: number, data: DNSProviderRequest): Promise<DNSProvider> {
  const response = await client.put<DNSProvider>(`/dns-providers/${id}`, data)
  return response.data
}

/**
 * Deletes a DNS provider.
 * @param id - The DNS provider ID
 * @throws {AxiosError} If not found or in use by proxy hosts
 */
export async function deleteDNSProvider(id: number): Promise<void> {
  await client.delete(`/dns-providers/${id}`)
}

/**
 * Tests connectivity of a saved DNS provider.
 * @param id - The DNS provider ID
 * @returns Promise resolving to test result
 * @throws {AxiosError} If not found or request fails
 */
export async function testDNSProvider(id: number): Promise<DNSTestResult> {
  const response = await client.post<DNSTestResult>(`/dns-providers/${id}/test`)
  return response.data
}

/**
 * Tests DNS provider credentials before saving.
 * @param data - Provider configuration to test
 * @returns Promise resolving to test result
 * @throws {AxiosError} If validation fails or request fails
 */
export async function testDNSProviderCredentials(data: DNSProviderRequest): Promise<DNSTestResult> {
  const response = await client.post<DNSTestResult>('/dns-providers/test', data)
  return response.data
}

/**
 * Fetches supported DNS provider types with field definitions.
 * @returns Promise resolving to array of provider type info
 * @throws {AxiosError} If request fails
 */
export async function getDNSProviderTypes(): Promise<DNSProviderTypeInfo[]> {
  const response = await client.get<DNSProviderTypesResponse>('/dns-providers/types')
  return response.data.types
}
