import client from './client'

/** Represents a zone-specific credential set */
export interface DNSProviderCredential {
  uuid: string
  dns_provider_id: number
  label: string
  zone_filter: string
  enabled: boolean
  propagation_timeout: number
  polling_interval: number
  key_version: number
  last_used_at?: string
  success_count: number
  failure_count: number
  last_error?: string
  created_at: string
  updated_at: string
}

/** Request payload for creating/updating credentials */
export interface CredentialRequest {
  label: string
  zone_filter: string
  credentials: Record<string, string>
  propagation_timeout?: number
  polling_interval?: number
  enabled?: boolean
}

/** Credential test result */
export interface CredentialTestResult {
  success: boolean
  message?: string
  error?: string
  propagation_time_ms?: number
}

/** Response for list endpoint */
interface ListCredentialsResponse {
  credentials: DNSProviderCredential[]
  total: number
}

/**
 * Fetches all credentials for a DNS provider.
 * @param providerId - The DNS provider UUID
 * @returns Promise resolving to array of credentials
 * @throws {AxiosError} If the request fails
 */
export async function getCredentials(providerId: string): Promise<DNSProviderCredential[]> {
  const response = await client.get<ListCredentialsResponse>(
    `/dns-providers/${providerId}/credentials`
  )
  return response.data.credentials
}

/**
 * Fetches a single credential by UUID.
 * @param providerId - The DNS provider UUID
 * @param credentialId - The credential UUID
 * @returns Promise resolving to the credential
 * @throws {AxiosError} If not found or request fails
 */
export async function getCredential(
  providerId: string,
  credentialId: string
): Promise<DNSProviderCredential> {
  const response = await client.get<DNSProviderCredential>(
    `/dns-providers/${providerId}/credentials/${credentialId}`
  )
  return response.data
}

/**
 * Creates a new credential for a DNS provider.
 * @param providerId - The DNS provider UUID
 * @param data - Credential configuration
 * @returns Promise resolving to the created credential
 * @throws {AxiosError} If validation fails or request fails
 */
export async function createCredential(
  providerId: string,
  data: CredentialRequest
): Promise<DNSProviderCredential> {
  const response = await client.post<DNSProviderCredential>(
    `/dns-providers/${providerId}/credentials`,
    data
  )
  return response.data
}

/**
 * Updates an existing credential.
 * @param providerId - The DNS provider UUID
 * @param credentialId - The credential UUID
 * @param data - Updated configuration
 * @returns Promise resolving to the updated credential
 * @throws {AxiosError} If not found, validation fails, or request fails
 */
export async function updateCredential(
  providerId: string,
  credentialId: string,
  data: CredentialRequest
): Promise<DNSProviderCredential> {
  const response = await client.put<DNSProviderCredential>(
    `/dns-providers/${providerId}/credentials/${credentialId}`,
    data
  )
  return response.data
}

/**
 * Deletes a credential.
 * @param providerId - The DNS provider UUID
 * @param credentialId - The credential UUID
 * @throws {AxiosError} If not found or in use
 */
export async function deleteCredential(providerId: string, credentialId: string): Promise<void> {
  await client.delete(`/dns-providers/${providerId}/credentials/${credentialId}`)
}

/**
 * Tests a credential's connectivity.
 * @param providerId - The DNS provider UUID
 * @param credentialId - The credential UUID
 * @returns Promise resolving to test result
 * @throws {AxiosError} If not found or request fails
 */
export async function testCredential(
  providerId: string,
  credentialId: string
): Promise<CredentialTestResult> {
  const response = await client.post<CredentialTestResult>(
    `/dns-providers/${providerId}/credentials/${credentialId}/test`
  )
  return response.data
}

/**
 * Enables multi-credential mode for a DNS provider.
 * @param providerId - The DNS provider UUID
 * @throws {AxiosError} If provider not found or already enabled
 */
export async function enableMultiCredentials(providerId: string): Promise<void> {
  await client.post(`/dns-providers/${providerId}/enable-multi-credentials`)
}
