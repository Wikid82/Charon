import client from './client'

export interface Domain {
  id: number
  uuid: string
  name: string
  created_at: string
}

export const getDomains = async (): Promise<Domain[]> => {
  const { data } = await client.get<Domain[]>('/domains')
  return data
}

export const createDomain = async (name: string): Promise<Domain> => {
  const { data } = await client.post<Domain>('/domains', { name })
  return data
}

export const deleteDomain = async (uuid: string): Promise<void> => {
  await client.delete(`/domains/${uuid}`)
}
