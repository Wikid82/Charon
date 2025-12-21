import client from './client'

/**
 * Fetches all feature flags and their current states.
 * @returns Promise resolving to a record of flag names to boolean values
 * @throws {AxiosError} If the request fails
 */
export async function getFeatureFlags(): Promise<Record<string, boolean>> {
  const resp = await client.get<Record<string, boolean>>('/feature-flags')
  return resp.data
}

/**
 * Updates one or more feature flags.
 * @param payload - Record of flag names to new boolean values
 * @returns Promise resolving to the update result
 * @throws {AxiosError} If the update fails
 */
export async function updateFeatureFlags(payload: Record<string, boolean>) {
  const resp = await client.put('/feature-flags', payload)
  return resp.data
}

export default {
  getFeatureFlags,
  updateFeatureFlags,
}
