import client from './client'

/** Map of setting keys to string values. */
export interface SettingsMap {
  [key: string]: string
}

/**
 * Fetches all application settings.
 * @returns Promise resolving to SettingsMap
 * @throws {AxiosError} If the request fails
 */
export const getSettings = async (): Promise<SettingsMap> => {
  const response = await client.get('/settings')
  return response.data
}

/**
 * Updates a single application setting.
 * @param key - The setting key to update
 * @param value - The new value for the setting
 * @param category - Optional category for organization
 * @param type - Optional type hint for the setting
 * @throws {AxiosError} If the update fails
 */
export const updateSetting = async (key: string, value: string, category?: string, type?: string): Promise<void> => {
  await client.post('/settings', { key, value, category, type })
}
