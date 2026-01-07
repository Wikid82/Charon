import client from './client'

/** Plugin status types */
export type PluginStatus = 'pending' | 'loaded' | 'error'

/** Plugin information */
export interface PluginInfo {
  id: number
  uuid: string
  name: string
  type: string
  enabled: boolean
  status: PluginStatus
  error?: string
  version?: string
  author?: string
  is_built_in: boolean
  description?: string
  documentation_url?: string
  loaded_at?: string
  created_at: string
  updated_at: string
}

/** Credential field specification */
export interface CredentialFieldSpec {
  name: string
  label: string
  type: 'text' | 'password' | 'textarea' | 'select'
  placeholder?: string
  hint?: string
  required?: boolean
  options?: Array<{
    value: string
    label: string
  }>
}

/** Provider metadata response */
export interface ProviderFieldsResponse {
  type: string
  name: string
  required_fields: CredentialFieldSpec[]
  optional_fields: CredentialFieldSpec[]
}

/**
 * Fetches all plugins (built-in and external).
 * @returns Promise resolving to array of plugin info
 * @throws {AxiosError} If the request fails
 */
export async function getPlugins(): Promise<PluginInfo[]> {
  const response = await client.get<PluginInfo[]>('/admin/plugins')
  return response.data
}

/**
 * Fetches a single plugin by ID.
 * @param id - The plugin ID
 * @returns Promise resolving to the plugin info
 * @throws {AxiosError} If not found or request fails
 */
export async function getPlugin(id: number): Promise<PluginInfo> {
  const response = await client.get<PluginInfo>(`/admin/plugins/${id}`)
  return response.data
}

/**
 * Enables a disabled plugin.
 * @param id - The plugin ID
 * @returns Promise resolving to success message
 * @throws {AxiosError} If not found or request fails
 */
export async function enablePlugin(id: number): Promise<{ message: string }> {
  const response = await client.post<{ message: string }>(`/admin/plugins/${id}/enable`)
  return response.data
}

/**
 * Disables an active plugin.
 * @param id - The plugin ID
 * @returns Promise resolving to success message
 * @throws {AxiosError} If not found, in use, or request fails
 */
export async function disablePlugin(id: number): Promise<{ message: string }> {
  const response = await client.post<{ message: string }>(`/admin/plugins/${id}/disable`)
  return response.data
}

/**
 * Reloads all plugins from the plugin directory.
 * @returns Promise resolving to success message and count
 * @throws {AxiosError} If request fails
 */
export async function reloadPlugins(): Promise<{ message: string; count: number }> {
  const response = await client.post<{ message: string; count: number }>('/admin/plugins/reload')
  return response.data
}

/**
 * Fetches credential field definitions for a DNS provider type.
 * @param providerType - The provider type (e.g., "cloudflare", "powerdns")
 * @returns Promise resolving to field specifications
 * @throws {AxiosError} If provider type not found or request fails
 */
export async function getProviderFields(providerType: string): Promise<ProviderFieldsResponse> {
  const response = await client.get<ProviderFieldsResponse>(`/dns-providers/types/${providerType}/fields`)
  return response.data
}
