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
  'hub_unreachable',
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

/** Response of the preset list. `hub_offline` is true when the list was served without the CrowdSec hub. */
export interface CrowdsecPresetListResponse {
  presets: CrowdsecPresetSummary[]
  /** Optional so that an older server without the field still parses. */
  hub_offline?: boolean
}

/** Response from pulling a CrowdSec preset. */
export interface PullCrowdsecPresetResponse {
  status: string
  slug: string
  /** Hub section of the pulled item. */
  type?: string
  preview: string
  cache_key: string
  /** Digest of the pulled item; sent back on apply so a changed cache entry is refused as stale. */
  item_sha256?: string
  etag?: string
  retrieved_at?: string
  source?: string
}

/** Body of a preset apply request. */
export interface ApplyCrowdsecPresetRequest {
  slug: string
  type?: string
  cache_key?: string
  /** Digest from pull; the server refuses the apply when the cached item no longer matches. */
  item_sha256?: string
}

/** Response from applying a CrowdSec preset. */
export interface ApplyCrowdsecPresetResponse {
  status: string
  backup?: string
  reload_hint?: boolean
  /** True when a reload was queued until CrowdSec finishes starting; show passive text, not a reload prompt. */
  reload_pending?: boolean
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
 * @returns Promise resolving to the presets array and the hub_offline flag
 * @throws {AxiosError} If the request fails
 */
export async function listCrowdsecPresets(): Promise<CrowdsecPresetListResponse> {
  const resp = await client.get<CrowdsecPresetListResponse>('/admin/crowdsec/presets')
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
 * @param payload - The canonical preset slug and hub section returned by pull, plus the cache_key and item_sha256 of the previewed copy.
 *   `cache_key` is required for hub presets and optional for curated ones.
 * @returns Promise resolving to ApplyCrowdsecPresetResponse
 * @throws {AxiosError} If application fails; the body is a CrowdsecPresetApiError
 */
export async function applyCrowdsecPreset(payload: ApplyCrowdsecPresetRequest) {
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

/** States of a background preset install job. */
export const APPLY_JOB_STATES = ['queued', 'running', 'verifying', 'succeeded', 'failed', 'timed_out', 'cancelled'] as const
export type CrowdsecApplyJobState = (typeof APPLY_JOB_STATES)[number]

/** Phases a background install moves through. */
export const APPLY_JOB_PHASES = ['preflight', 'snapshot', 'install', 'verify', 'reload', 'rollback'] as const
export type CrowdsecApplyJobPhase = (typeof APPLY_JOB_PHASES)[number]

/** Failure outcome of a job: a preset code, or `interrupted` when the server restarted mid-install. */
export type CrowdsecApplyJobErrorCode = CrowdsecPresetErrorCode | 'interrupted'

/** A background preset install job as reported by the server. */
export interface CrowdsecApplyJob {
  job_id: string
  slug: string
  item_type: string
  cache_key: string
  state: CrowdsecApplyJobState
  phase: CrowdsecApplyJobPhase
  error_code?: CrowdsecApplyJobErrorCode | ''
  error?: string
  warnings: string[]
  items_installed: number
  items_total: number | null
  cancel_requested: boolean
  elapsed_seconds: number
  created_at: string
  started_at: string | null
  finished_at: string | null
}

/** Error codes of the job endpoints. */
export const APPLY_JOB_ERROR_CODES = ['job_not_found', 'job_not_active', 'invalid_job_id'] as const
export type CrowdsecApplyJobApiErrorCode = (typeof APPLY_JOB_ERROR_CODES)[number]

/** Reads the machine-readable outcome from a failed job request. */
export function getApplyJobErrorCode(err: unknown): CrowdsecApplyJobApiErrorCode | undefined {
  return getApiErrorCode(err, APPLY_JOB_ERROR_CODES)
}

/**
 * Starts installing a hub preset as a background job.
 * @returns The queued job (HTTP 202)
 * @throws {AxiosError} 409 `crowdsec_busy` while another install or a data change runs
 */
export async function applyPresetAsync(payload: ApplyCrowdsecPresetRequest) {
  const resp = await client.post<CrowdsecApplyJob>('/admin/crowdsec/presets/apply', { ...payload, async: true })
  return resp.data
}

/**
 * Reads the active install job, or the newest one that finished in the last 30 minutes.
 * @returns The job, or null when there is none
 */
export async function getCurrentApplyJob() {
  const resp = await client.get<{ job: CrowdsecApplyJob | null }>('/admin/crowdsec/presets/jobs/current')
  return resp.data.job
}

/** Reads one install job by id. */
export async function getApplyJob(id: string) {
  const resp = await client.get<CrowdsecApplyJob>(`/admin/crowdsec/presets/jobs/${encodeURIComponent(id)}`)
  return resp.data
}

/** Lists the most recent install jobs, newest first. */
export async function listApplyJobs(limit?: number) {
  const resp = await client.get<{ jobs: CrowdsecApplyJob[] }>('/admin/crowdsec/presets/jobs', { params: { limit } })
  return resp.data.jobs
}

/**
 * Asks an active install job to stop; it rolls back and ends `cancelled`.
 * @returns The job (HTTP 202)
 * @throws {AxiosError} 409 `job_not_active` for a finished job, 404 `job_not_found`
 */
export async function cancelApplyJob(id: string) {
  const resp = await client.delete<CrowdsecApplyJob>(`/admin/crowdsec/presets/jobs/${encodeURIComponent(id)}`)
  return resp.data
}

export default {
  listCrowdsecPresets,
  getCrowdsecPresets,
  pullCrowdsecPreset,
  applyCrowdsecPreset,
  getCrowdsecPresetCache,
  applyPresetAsync,
  getCurrentApplyJob,
  getApplyJob,
  listApplyJobs,
  cancelApplyJob,
}
