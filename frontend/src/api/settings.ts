import client from './client'

export interface SettingsMap {
  [key: string]: string
}

export const getSettings = async (): Promise<SettingsMap> => {
  const response = await client.get('/settings')
  return response.data
}

export const updateSetting = async (key: string, value: string, category?: string, type?: string): Promise<void> => {
  await client.post('/settings', { key, value, category, type })
}
