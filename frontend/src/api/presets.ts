import { isAxiosError } from 'axios'

import client from './client'
import { getApiErrorCode } from './errorBody'

/**
 * Per-request timeout for applying a preset. The server allows up to 120 s for
 * the install, which is longer than the default client timeout (30 s).
 */
export const APPLY_CROWDSEC_PRESET_TIMEOUT_MS = 130_000

/** Machine-readable outcomes the server reports when a preset apply does not succeed. */
export const PRESET_ERROR_CODES = [
  'preset_cache_key_required',
  'preset_cache_stale',
  'preset_hub_updating',
  'preset_local_modified',
  'preset_apply_timeout',
  'preset_type_ambiguous',
  'cscli_unavailable',
] as const

export type CrowdsecPresetErrorCode = (typeof PRESET_ERROR_CODES)[number]

/** Error body returned by the preset endpoints. */
export interface CrowdsecPresetApiError {
  error: string
  error_code?: CrowdsecPresetErrorCode
  /** `type/name` of each edited item behind a `preset_local_modified` outcome. */
  tainted_items?: string[]
  /** Hub sections that hold the name behind a `preset_type_ambiguous` outcome. */
  types?: string[]
}

/** Summary of an available CrowdSec preset. */
export interface CrowdsecPresetSummary {
  slug: string
  title: string
  summary: string
  source: string
  /** Hub section of the item. One hub name can have an entry per section. */
  type?: string
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
  /** Hub section of the pulled item. */
  type?: string
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
  /** Hub section of the installed item. */
  type?: string
  /** Passive notes, e.g. a dependency that was edited on this server. */
  warnings?: string[]
}

/** Cached CrowdSec preset preview data. */
export interface CachedCrowdsecPresetPreview {
  preview: string
  type?: string
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
 * @param type - Hub section; required only when the name exists in several sections
 * @returns Promise resolving to PullCrowdsecPresetResponse with preview
 * @throws {AxiosError} If pull fails or preset not found
 */
export async function pullCrowdsecPreset(slug: string, type?: string) {
  const resp = await client.post<PullCrowdsecPresetResponse>('/admin/crowdsec/presets/pull', type ? { slug, type } : { slug })
  return resp.data
}

/**
 * Applies a CrowdSec preset to the configuration.
 * @param payload - The canonical preset slug and hub section returned by pull and the cache_key of the previewed copy.
 *   `cache_key` is required for hub presets and optional for curated ones.
 * @returns Promise resolving to ApplyCrowdsecPresetResponse
 * @throws {AxiosError} If application fails; the body is a CrowdsecPresetApiError
 */
export async function applyCrowdsecPreset(payload: { slug: string; type?: string; cache_key?: string }) {
  const resp = await client.post<ApplyCrowdsecPresetResponse>('/admin/crowdsec/presets/apply', payload, {
    timeout: APPLY_CROWDSEC_PRESET_TIMEOUT_MS,
  })
  return resp.data
}

/**
 * Reads the machine-readable outcome from a failed preset request.
 * @returns The error code, or undefined for other failures
 */
export function getCrowdsecPresetErrorCode(err: unknown): CrowdsecPresetErrorCode | undefined {
  return getApiErrorCode(err, PRESET_ERROR_CODES)
}

/**
 * Reads the edited item names from a failed preset request.
 * @returns The names, or an empty list when the server sent none
 */
export function getCrowdsecPresetTaintedItems(err: unknown): string[] {
  if (!isAxiosError<Partial<CrowdsecPresetApiError>>(err)) return []
  const items = err.response?.data?.tainted_items
  return Array.isArray(items) ? items.filter((item): item is string => typeof item === 'string') : []
}

/**
 * Reads the hub sections behind a `preset_type_ambiguous` outcome.
 * @returns The section names, or an empty list when the server sent none
 */
export function getCrowdsecPresetAmbiguousTypes(err: unknown): string[] {
  if (!isAxiosError<Partial<CrowdsecPresetApiError>>(err)) return []
  const types = err.response?.data?.types
  return Array.isArray(types) ? types.filter((item): item is string => typeof item === 'string') : []
}

/**
 * Gets a cached CrowdSec preset preview.
 * @param slug - The preset slug identifier
 * @param type - Hub section; required only when the name is cached in several sections
 * @returns Promise resolving to CachedCrowdsecPresetPreview
 * @throws {AxiosError} If not cached or request fails
 */
export async function getCrowdsecPresetCache(slug: string, type?: string) {
  const resp = await client.get<CachedCrowdsecPresetPreview>(
    `/admin/crowdsec/presets/cache/${slug.split('/').map(encodeURIComponent).join('/')}`,
    { params: { type } },
  )
  return resp.data
}

export default {
  listCrowdsecPresets,
  getCrowdsecPresets,
  pullCrowdsecPreset,
  applyCrowdsecPreset,
  getCrowdsecPresetCache,
}
