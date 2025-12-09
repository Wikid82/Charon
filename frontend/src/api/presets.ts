import client from './client'

export interface CrowdsecPresetSummary {
  slug: string
  title: string
  summary: string
  source: string
  tags?: string[]
  requires_hub: boolean
  available: boolean
  cached: boolean
  cache_key?: string
  etag?: string
  retrieved_at?: string
}

export interface PullCrowdsecPresetResponse {
  status: string
  slug: string
  preview: string
  cache_key: string
  etag?: string
  retrieved_at?: string
  source?: string
}

export interface ApplyCrowdsecPresetResponse {
  status: string
  backup?: string
  reload_hint?: string
  used_cscli?: boolean
  cache_key?: string
  slug?: string
}

export interface CachedCrowdsecPresetPreview {
  preview: string
  cache_key: string
  etag?: string
}

export async function listCrowdsecPresets() {
  const resp = await client.get<{ presets: CrowdsecPresetSummary[] }>('/admin/crowdsec/presets')
  return resp.data
}

export async function pullCrowdsecPreset(slug: string) {
  const resp = await client.post<PullCrowdsecPresetResponse>('/admin/crowdsec/presets/pull', { slug })
  return resp.data
}

export async function applyCrowdsecPreset(payload: { slug: string; cache_key?: string }) {
  const resp = await client.post<ApplyCrowdsecPresetResponse>('/admin/crowdsec/presets/apply', payload)
  return resp.data
}

export async function getCrowdsecPresetCache(slug: string) {
  const resp = await client.get<CachedCrowdsecPresetPreview>(`/admin/crowdsec/presets/cache/${encodeURIComponent(slug)}`)
  return resp.data
}

export default {
  listCrowdsecPresets,
  pullCrowdsecPreset,
  applyCrowdsecPreset,
  getCrowdsecPresetCache,
}
