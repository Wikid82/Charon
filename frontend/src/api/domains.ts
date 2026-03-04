import client from './client'

/** Represents a managed domain. */
export interface Domain {
  id: number
  uuid: string
  name: string
  created_at: string
}

/**
 * Fetches all managed domains.
 * @returns Promise resolving to array of Domain objects
 * @throws {AxiosError} If the request fails
 */
export const getDomains = async (): Promise<Domain[]> => {
  const { data } = await client.get<Domain[]>('/domains')
  return data
}

/**
 * Creates a new managed domain.
 * @param name - The domain name to create
 * @returns Promise resolving to the created Domain
 * @throws {AxiosError} If creation fails or domain is invalid
 */
export const createDomain = async (name: string): Promise<Domain> => {
  const { data } = await client.post<Domain>('/domains', { name })
  return data
}

/**
 * Deletes a managed domain.
 * @param uuid - The unique identifier of the domain to delete
 * @throws {AxiosError} If deletion fails or domain not found
 */
export const deleteDomain = async (uuid: string): Promise<void> => {
  await client.delete(`/domains/${uuid}`)
}
