import { isAxiosError } from 'axios'

import client from './client'
import { getApiErrorCode } from './errorBody'

/** Represents a CrowdSec decision (ban/captcha). */
export interface CrowdSecDecision {
  id: string
  ip: string
  reason: string
  duration: string
  created_at: string
  source: string
}

/** Why a list came back without data; sent beside the English `error` text. */
export const CROWDSEC_LIST_ERROR_CODES = ['cscli_timeout', 'cscli_output_too_large', 'cscli_unavailable'] as const

export type CrowdsecListErrorCode = (typeof CROWDSEC_LIST_ERROR_CODES)[number]

/** Soft error fields shared by the decisions and alerts list responses; absent when the list loaded. */
export interface CrowdsecListSoftError {
  error?: string
  error_code?: CrowdsecListErrorCode
}

export interface CrowdsecDecisionsResponse extends CrowdsecListSoftError {
  decisions: CrowdSecDecision[]
}

/**
 * Starts the CrowdSec security service.
 * @returns Promise resolving to status with process ID and LAPI readiness
 * @throws {AxiosError} If the service fails to start
 */
export async function startCrowdsec(): Promise<{ status: string; pid: number; lapi_ready?: boolean }> {
  const resp = await client.post('/admin/crowdsec/start')
  return resp.data
}

/**
 * Stops the CrowdSec security service.
 * @returns Promise resolving to stop status
 * @throws {AxiosError} If the service fails to stop
 */
export async function stopCrowdsec() {
  const resp = await client.post('/admin/crowdsec/stop')
  return resp.data
}

/** CrowdSec service status information. */
export interface CrowdSecStatus {
  running: boolean
  pid: number
  lapi_ready: boolean
}

/**
 * Gets the current status of the CrowdSec service.
 * @returns Promise resolving to CrowdSecStatus
 * @throws {AxiosError} If status check fails
 */
export async function statusCrowdsec(): Promise<CrowdSecStatus> {
  const resp = await client.get<CrowdSecStatus>('/admin/crowdsec/status')
  return resp.data
}

/** A CrowdSec hub item identified by its type and name. */
export interface HubItem {
  type: string
  name: string
}

/** Why a hub item was not installed by a restore. */
export type HubItemFailureReason = 'not_found' | 'install_failed' | 'verify_failed' | 'not_attempted'

/** A hub item the restore could not install. */
export interface HubItemFailure extends HubItem {
  reason: HubItemFailureReason
}

/** Hub item summary returned by an import of an export that lists hub items. */
export interface ImportHubItems {
  in_export: number
  skipped_invalid: number
  missing: HubItem[]
}

/** Result of a CrowdSec configuration import. */
export interface ImportCrowdsecResult {
  status?: string
  backup?: string
  hub_items?: ImportHubItems
  /** Names of the current files kept because the archive did not contain them. */
  kept_files?: string[]
  /** Imported notification files whose blanked values still need to be entered. */
  needs_values?: string[]
  /** True when Charon rewrote its allow-list file from this instance's database after the import. */
  whitelist_regenerated?: boolean
  /** Other allow-list files found under the parsers directory, which Charon leaves unchanged. */
  other_whitelist_files?: string[]
}

/** Machine-readable reasons the server gives for refusing an import. */
export const IMPORT_ERROR_CODES = [
  'import_flat_archive',
  'import_profile_names_undefined',
  'import_kept_profile_undefined',
  'import_kept_profile_duplicate',
  'import_keep_current_failed',
  'import_config_not_found',
] as const

export type CrowdsecImportErrorCode = (typeof IMPORT_ERROR_CODES)[number]

/** Error body of a refused import; refusals without a code carry only the English `error`. */
export interface CrowdsecImportApiError {
  error: string
  error_code?: CrowdsecImportErrorCode
  /** Notification names behind the two "undefined" refusals (sanitised, at most 50). */
  names?: string[]
}

/** Result of a hub item restore. */
export interface RestoreHubItemsResult {
  installed: HubItem[]
  already_installed: HubItem[]
  failed: HubItemFailure[]
  remaining: number
  skipped_invalid: number
  reloaded: boolean
}

/**
 * Reads the machine-readable refusal reason from a failed import request.
 * @returns The code and the notification names, or undefined for an unknown or missing code
 */
export function getCrowdsecImportError(err: unknown): { code: CrowdsecImportErrorCode; names: string[] } | undefined {
  const code = getApiErrorCode(err, IMPORT_ERROR_CODES)
  if (!code || !isAxiosError<Partial<CrowdsecImportApiError>>(err)) return undefined
  const names = err.response?.data?.names
  return { code, names: Array.isArray(names) ? names.filter((n): n is string => typeof n === 'string') : [] }
}

/** A restore downloads items one by one; the server budget is 5 minutes. */
const RESTORE_HUB_ITEMS_TIMEOUT_MS = 330_000

/**
 * Imports a CrowdSec configuration file.
 * @param file - The configuration file to import
 * @returns Promise resolving to import result
 * @throws {AxiosError} If import fails or file is invalid
 */
export async function importCrowdsecConfig(file: File): Promise<ImportCrowdsecResult> {
  const fd = new FormData()
  fd.append('file', file)
  const resp = await client.post<ImportCrowdsecResult>('/admin/crowdsec/import', fd, {
    headers: { 'Content-Type': 'multipart/form-data' },
  })
  return resp.data
}

/**
 * Installs the given hub items from the official CrowdSec hub.
 * @param items - The hub items to install
 * @returns Promise resolving to the per-item outcome
 * @throws {AxiosError} 409 if a restore is running, 502 if the hub is unreachable
 */
export async function restoreHubItems(items: HubItem[]): Promise<RestoreHubItemsResult> {
  const resp = await client.post<RestoreHubItemsResult>(
    '/admin/crowdsec/hub-items/restore',
    { items },
    { timeout: RESTORE_HUB_ITEMS_TIMEOUT_MS },
  )
  return resp.data
}

/** An export archive plus the files the server left out of it. */
export interface ExportCrowdsecResult {
  blob: Blob
  /** Paths of private key files left out; the server lists at most 50. */
  omitted: string[]
  /** Files left out beyond the ones named in `omitted`. */
  omittedMore: number
}

const OMITTED_FILES_HEADER = 'x-charon-omitted-files'
const OMITTED_MORE_PATTERN = /^\+(\d+) more$/

// HTTP header bytes reach the browser as Latin-1; the server sends UTF-8, so non-ASCII names must be re-decoded.
function decodeHeaderUtf8(value: string): string {
  try {
    const bytes = Uint8Array.from(value, (ch) => ch.charCodeAt(0))
    return new TextDecoder('utf-8', { fatal: true }).decode(bytes)
  } catch {
    return value
  }
}

/**
 * Reads the JSON array the server puts in X-Charon-Omitted-Files. A trailing "+N more" entry counts the
 * names that did not fit. A missing or malformed header means nothing is reported.
 */
function parseOmittedHeader(raw: unknown): Pick<ExportCrowdsecResult, 'omitted' | 'omittedMore'> {
  if (typeof raw !== 'string' || raw === '') return { omitted: [], omittedMore: 0 }
  let parsed: unknown
  try {
    parsed = JSON.parse(decodeHeaderUtf8(raw))
  } catch {
    return { omitted: [], omittedMore: 0 }
  }
  if (!Array.isArray(parsed)) return { omitted: [], omittedMore: 0 }
  const entries = parsed.filter((e): e is string => typeof e === 'string')
  const more = entries.length > 0 ? OMITTED_MORE_PATTERN.exec(entries[entries.length - 1]) : null
  if (more) return { omitted: entries.slice(0, -1), omittedMore: Number(more[1]) }
  return { omitted: entries, omittedMore: 0 }
}

/**
 * Exports the current CrowdSec configuration.
 * @returns Promise resolving to the archive and the files the server left out of it
 * @throws {AxiosError} If export fails
 */
export async function exportCrowdsecConfig(): Promise<ExportCrowdsecResult> {
  try {
    const resp = await client.get<Blob>('/admin/crowdsec/export', { responseType: 'blob' })
    return { blob: resp.data, ...parseOmittedHeader(resp.headers?.[OMITTED_FILES_HEADER]) }
  } catch (err) {
    throw await withReadableBody(err)
  }
}

/** Machine-readable reasons the server gives for refusing an export. */
export const EXPORT_ERROR_CODES = ['export_config_unsafe'] as const

export type CrowdsecExportErrorCode = (typeof EXPORT_ERROR_CODES)[number]

/**
 * Reads the machine-readable reason from a failed export request.
 * @returns The code, or undefined for an unknown or missing code
 */
export function getCrowdsecExportError(err: unknown): CrowdsecExportErrorCode | undefined {
  return getApiErrorCode(err, EXPORT_ERROR_CODES)
}

/**
 * The export asks for a blob, so a failure body arrives as a blob too. Replaces it with the parsed JSON so the
 * error code can be read; a body that is not JSON is left as it is.
 */
async function withReadableBody(err: unknown): Promise<unknown> {
  if (isAxiosError(err) && err.response?.data instanceof Blob) {
    try {
      err.response.data = JSON.parse(await err.response.data.text())
    } catch {
      // Not a JSON error body: the caller falls back to its generic message.
    }
  }
  return err
}

/**
 * Lists all CrowdSec configuration files.
 * @returns Promise resolving to object containing file list
 * @throws {AxiosError} If listing fails
 */
export async function listCrowdsecFiles() {
  const resp = await client.get<{ files: string[] }>('/admin/crowdsec/files')
  return resp.data
}

/**
 * Reads the content of a CrowdSec configuration file.
 * @param path - The file path to read
 * @returns Promise resolving to object containing file content
 * @throws {AxiosError} If file cannot be read
 */
export async function readCrowdsecFile(path: string) {
  const resp = await client.get<{ content: string }>(`/admin/crowdsec/file?path=${encodeURIComponent(path)}`)
  return resp.data
}

/**
 * Writes content to a CrowdSec configuration file.
 * @param path - The file path to write
 * @param content - The content to write
 * @returns Promise resolving to write result
 * @throws {AxiosError} If file cannot be written
 */
export async function writeCrowdsecFile(path: string, content: string) {
  const resp = await client.post('/admin/crowdsec/file', { path, content })
  return resp.data
}

/**
 * Lists all active CrowdSec decisions (bans).
 * @returns Promise resolving to object containing decisions array
 * @throws {AxiosError} If listing fails
 */
export async function listCrowdsecDecisions(): Promise<CrowdsecDecisionsResponse> {
  const resp = await client.get<CrowdsecDecisionsResponse>('/admin/crowdsec/decisions')
  return resp.data
}

/**
 * Bans an IP address via CrowdSec.
 * @param ip - The IP address to ban
 * @param duration - Ban duration (e.g., "24h", "7d")
 * @param reason - Reason for the ban
 * @throws {AxiosError} If ban fails
 */
export async function banIP(ip: string, duration: string, reason: string): Promise<void> {
  await client.post('/admin/crowdsec/ban', { ip, duration, reason })
}

/** Machine-readable reasons the server gives for refusing a ban or unban. */
export const BAN_ERROR_CODES = [
  'invalid_ip',
  'invalid_duration',
  'invalid_reason',
  'decision_not_found',
  'cscli_timeout',
  'request_too_large',
] as const

export type CrowdsecBanErrorCode = (typeof BAN_ERROR_CODES)[number]

/**
 * Reads the machine-readable reason from a failed ban or unban request.
 * @returns The code, or undefined for an unknown or missing code
 */
export function getCrowdsecBanError(err: unknown): CrowdsecBanErrorCode | undefined {
  return getApiErrorCode(err, BAN_ERROR_CODES)
}

/**
 * Removes a ban for an IP address.
 * @param ip - The IP address or range to unban
 * @throws {AxiosError} If unban fails
 */
export async function unbanIP(ip: string): Promise<void> {
  // A range such as 192.0.2.0/24 is several path segments; the route takes them all.
  await client.delete(`/admin/crowdsec/ban/${ip.split('/').map(encodeURIComponent).join('/')}`)
}

/** CrowdSec API key status information for key rejection notifications. */
export interface CrowdSecKeyStatus {
  key_source: 'env' | 'file' | 'auto-generated' | 'none'
  env_key_rejected: boolean
  full_key?: string  // Only present when env_key_rejected is true
  current_key_preview: string
  rejected_key_preview?: string
  message: string
}

/**
 * Gets the current CrowdSec API key status.
 * Used to display warning banner when env key was rejected.
 * @returns Promise resolving to CrowdSecKeyStatus
 * @throws {AxiosError} If status check fails
 */
export async function getCrowdsecKeyStatus(): Promise<CrowdSecKeyStatus> {
  const resp = await client.get<CrowdSecKeyStatus>('/admin/crowdsec/key-status')
  return resp.data
}

export interface CrowdSecWhitelistEntry {
  uuid: string
  ip_or_cidr: string
  reason: string
  created_at: string
  updated_at: string
}

export interface AddWhitelistPayload {
  ip_or_cidr: string
  reason: string
}

export const listWhitelists = async (): Promise<CrowdSecWhitelistEntry[]> => {
  const resp = await client.get<{ whitelist: CrowdSecWhitelistEntry[] }>('/admin/crowdsec/whitelist')
  return resp.data.whitelist
}

export const addWhitelist = async (data: AddWhitelistPayload): Promise<CrowdSecWhitelistEntry> => {
  const resp = await client.post<CrowdSecWhitelistEntry>('/admin/crowdsec/whitelist', data)
  return resp.data
}

export const deleteWhitelist = async (uuid: string): Promise<void> => {
  await client.delete(`/admin/crowdsec/whitelist/${uuid}`)
}

export default { startCrowdsec, stopCrowdsec, statusCrowdsec, importCrowdsecConfig, restoreHubItems, exportCrowdsecConfig, listCrowdsecFiles, readCrowdsecFile, writeCrowdsecFile, listCrowdsecDecisions, banIP, unbanIP, getCrowdsecKeyStatus, listWhitelists, addWhitelist, deleteWhitelist }
