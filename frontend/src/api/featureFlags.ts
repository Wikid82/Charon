import client from './client'

export async function getFeatureFlags(): Promise<Record<string, boolean>> {
  const resp = await client.get<Record<string, boolean>>('/feature-flags')
  return resp.data
}

export async function updateFeatureFlags(payload: Record<string, boolean>) {
  const resp = await client.put('/feature-flags', payload)
  return resp.data
}

export default {
  getFeatureFlags,
  updateFeatureFlags,
}
