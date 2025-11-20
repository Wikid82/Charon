import client from './client'

export interface UserProfile {
  id: number
  email: string
  name: string
  role: string
  api_key: string
}

export const getProfile = async (): Promise<UserProfile> => {
  const response = await client.get('/user/profile')
  return response.data
}

export const regenerateApiKey = async (): Promise<{ api_key: string }> => {
  const response = await client.post('/user/api-key')
  return response.data
}
