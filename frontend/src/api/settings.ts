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

/**
 * Validates a URL for use as the application URL.
 * @param url - The URL to validate
 * @returns Promise resolving to validation result
 */
export const validatePublicURL = async (url: string): Promise<{
  valid: boolean
  normalized?: string
  error?: string
}> => {
  const response = await client.post('/settings/validate-url', { url })
  return response.data
}

/**
 * Tests if a URL is reachable from the server with SSRF protection.
 * @param url - The URL to test
 * @returns Promise resolving to test result with reachability status and latency
 */
export const testPublicURL = async (url: string): Promise<{
  reachable: boolean
  latency?: number
  message?: string
  error?: string
}> => {
  const response = await client.post('/settings/test-url', { url })
  return response.data
}

/**
 * Uploads a logo image file.
 * Accepted types: image/png, image/jpeg, image/webp (max 2 MB).
 * @param file - The image file to upload
 * @returns Promise resolving to the served URL of the uploaded logo
 */
export const uploadLogo = async (file: File): Promise<{ url: string }> => {
  const form = new FormData()
  form.append('logo', file)
  const response = await client.post('/settings/logo', form, {
    headers: { 'Content-Type': 'multipart/form-data' },
  })
  return response.data
}

/**
 * Deletes the custom logo, restoring the default Charon logo.
 */
export const deleteLogo = async (): Promise<void> => {
  await client.delete('/settings/logo')
}

/**
 * Uploads a banner image file.
 * Accepted types: image/png, image/jpeg, image/webp (max 2 MB).
 * @param file - The image file to upload
 * @returns Promise resolving to the served URL of the uploaded banner
 */
export const uploadBanner = async (file: File): Promise<{ url: string }> => {
  const form = new FormData()
  form.append('banner', file)
  const response = await client.post('/settings/banner', form, {
    headers: { 'Content-Type': 'multipart/form-data' },
  })
  return response.data
}

/**
 * Deletes the custom banner, restoring the default banner image.
 */
export const deleteBanner = async (): Promise<void> => {
  await client.delete('/settings/banner')
}
