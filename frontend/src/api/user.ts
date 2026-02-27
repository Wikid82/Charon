import client from './client'

/** Current user profile information. */
export interface UserProfile {
  id: number
  email: string
  name: string
  role: string
  has_api_key: boolean
  api_key_masked: string
}

/**
 * Fetches the current user's profile.
 * @returns Promise resolving to UserProfile
 * @throws {AxiosError} If the request fails or not authenticated
 */
export const getProfile = async (): Promise<UserProfile> => {
  const response = await client.get('/user/profile')
  return response.data
}

/**
 * Regenerates the current user's API key.
 * @returns Promise resolving to object containing the new API key
 * @throws {AxiosError} If regeneration fails
 */
export interface RegenerateApiKeyResponse {
  message: string
  has_api_key: boolean
  api_key_masked: string
  api_key_updated: string
}

export const regenerateApiKey = async (): Promise<RegenerateApiKeyResponse> => {
  const response = await client.post<RegenerateApiKeyResponse>('/user/api-key')
  return response.data
}

/**
 * Updates the current user's profile.
 * @param data - Object with name, email, and optional current_password for verification
 * @returns Promise resolving to success message
 * @throws {AxiosError} If update fails or password verification fails
 */
export const updateProfile = async (data: { name: string; email: string; current_password?: string }): Promise<{ message: string }> => {
  const response = await client.post('/user/profile', data)
  return response.data
}
