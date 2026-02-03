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
