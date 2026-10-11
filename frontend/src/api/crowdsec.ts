import client from './client'

/** Represents a CrowdSec decision (ban/captcha). */
export interface CrowdSecDecision {
  id: string
  ip: string
  reason: string
  duration: string
  created_at: string
  source: string
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

/**
 * Exports the current CrowdSec configuration.
 * @returns Promise resolving to configuration blob for download
 * @throws {AxiosError} If export fails
 */
export async function exportCrowdsecConfig() {
  const resp = await client.get('/admin/crowdsec/export', { responseType: 'blob' })
  return resp.data
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
export async function listCrowdsecDecisions(): Promise<{ decisions: CrowdSecDecision[] }> {
  const resp = await client.get<{ decisions: CrowdSecDecision[] }>('/admin/crowdsec/decisions')
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

/**
 * Removes a ban for an IP address.
 * @param ip - The IP address to unban
 * @throws {AxiosError} If unban fails
 */
export async function unbanIP(ip: string): Promise<void> {
  await client.delete(`/admin/crowdsec/ban/${encodeURIComponent(ip)}`)
}

/** CrowdSec API key status information for key rejection notifications. */
export interface CrowdSecKeyStatus {
  key_source: 'env' | 'file' | 'auto-generated'
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
