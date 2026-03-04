import client from './client'

/** Summary of an available CrowdSec preset. */
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

/** Response from pulling a CrowdSec preset. */
export interface PullCrowdsecPresetResponse {
  status: string
  slug: string
  preview: string
  cache_key: string
  etag?: string
  retrieved_at?: string
  source?: string
}

/** Response from applying a CrowdSec preset. */
export interface ApplyCrowdsecPresetResponse {
  status: string
  backup?: string
  reload_hint?: boolean
  used_cscli?: boolean
  cache_key?: string
  slug?: string
}

/** Cached CrowdSec preset preview data. */
export interface CachedCrowdsecPresetPreview {
  preview: string
  cache_key: string
  etag?: string
}

/**
 * Lists all available CrowdSec presets.
 * @returns Promise resolving to object containing presets array
 * @throws {AxiosError} If the request fails
 */
export async function listCrowdsecPresets() {
  const resp = await client.get<{ presets: CrowdsecPresetSummary[] }>('/admin/crowdsec/presets')
  return resp.data
}

/**
 * Gets all CrowdSec presets (alias for listCrowdsecPresets).
 * @returns Promise resolving to object containing presets array
 * @throws {AxiosError} If the request fails
 */
export async function getCrowdsecPresets() {
  return listCrowdsecPresets()
}

/**
 * Pulls a CrowdSec preset from the remote source.
 * @param slug - The preset slug identifier
 * @returns Promise resolving to PullCrowdsecPresetResponse with preview
 * @throws {AxiosError} If pull fails or preset not found
 */
export async function pullCrowdsecPreset(slug: string) {
  const resp = await client.post<PullCrowdsecPresetResponse>('/admin/crowdsec/presets/pull', { slug })
  return resp.data
}

/**
 * Applies a CrowdSec preset to the configuration.
 * @param payload - Object with preset slug and optional cache_key
 * @returns Promise resolving to ApplyCrowdsecPresetResponse
 * @throws {AxiosError} If application fails
 */
export async function applyCrowdsecPreset(payload: { slug: string; cache_key?: string }) {
  const resp = await client.post<ApplyCrowdsecPresetResponse>('/admin/crowdsec/presets/apply', payload)
  return resp.data
}

/**
 * Gets a cached CrowdSec preset preview.
 * @param slug - The preset slug identifier
 * @returns Promise resolving to CachedCrowdsecPresetPreview
 * @throws {AxiosError} If not cached or request fails
 */
export async function getCrowdsecPresetCache(slug: string) {
  const resp = await client.get<CachedCrowdsecPresetPreview>(`/admin/crowdsec/presets/cache/${encodeURIComponent(slug)}`)
  return resp.data
}

export default {
  listCrowdsecPresets,
  getCrowdsecPresets,
  pullCrowdsecPreset,
  applyCrowdsecPreset,
  getCrowdsecPresetCache,
}
