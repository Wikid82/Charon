import client from './client'

/** Audit log event category */
export type EventCategory = 'dns_provider' | 'certificate' | 'proxy_host' | 'user' | 'system'

/** Audit log action type */
export type AuditAction =
  | 'dns_provider_create'
  | 'dns_provider_update'
  | 'dns_provider_delete'
  | 'credential_test'
  | 'credential_decrypt'
  | 'certificate_issue'
  | 'certificate_renew'
  | 'proxy_host_create'
  | 'proxy_host_update'
  | 'proxy_host_delete'
  | 'user_login'
  | 'user_logout'
  | 'settings_update'

/** Represents a single audit log entry */
export interface AuditLog {
  id: number
  uuid: string
  actor: string
  action: AuditAction
  event_category: EventCategory
  resource_id?: number
  resource_uuid?: string
  details: string
  ip_address?: string
  user_agent?: string
  created_at: string
}

/** Filters for querying audit logs */
export interface AuditLogFilters {
  event_category?: EventCategory
  actor?: string
  action?: AuditAction
  start_date?: string
  end_date?: string
  resource_uuid?: string
}

/** Response for list endpoint */
interface ListAuditLogsResponse {
  logs: AuditLog[]
  total: number
  page: number
  limit: number
}

/**
 * Fetches audit logs with pagination and filtering.
 * @param filters - Optional filters to apply
 * @param page - Page number (1-indexed)
 * @param limit - Number of records per page
 * @returns Promise resolving to paginated audit logs
 * @throws {AxiosError} If the request fails
 */
export async function getAuditLogs(
  filters?: AuditLogFilters,
  page: number = 1,
  limit: number = 50
): Promise<ListAuditLogsResponse> {
  const params = new URLSearchParams()
  params.append('page', page.toString())
  params.append('limit', limit.toString())

  if (filters) {
    if (filters.event_category) params.append('event_category', filters.event_category)
    if (filters.actor) params.append('actor', filters.actor)
    if (filters.action) params.append('action', filters.action)
    if (filters.start_date) params.append('start_date', filters.start_date)
    if (filters.end_date) params.append('end_date', filters.end_date)
    if (filters.resource_uuid) params.append('resource_uuid', filters.resource_uuid)
  }

  const response = await client.get<ListAuditLogsResponse>(`/audit-logs?${params.toString()}`)
  return response.data
}

/**
 * Fetches a single audit log by UUID.
 * @param uuid - The audit log UUID
 * @returns Promise resolving to the audit log
 * @throws {AxiosError} If not found or request fails
 */
export async function getAuditLog(uuid: string): Promise<AuditLog> {
  const response = await client.get<AuditLog>(`/audit-logs/${uuid}`)
  return response.data
}

/**
 * Fetches audit logs for a specific DNS provider.
 * @param providerId - The DNS provider ID
 * @param page - Page number (1-indexed)
 * @param limit - Number of records per page
 * @returns Promise resolving to paginated audit logs
 * @throws {AxiosError} If not found or request fails
 */
export async function getAuditLogsByProvider(
  providerId: number,
  page: number = 1,
  limit: number = 50
): Promise<ListAuditLogsResponse> {
  const params = new URLSearchParams()
  params.append('page', page.toString())
  params.append('limit', limit.toString())

  const response = await client.get<ListAuditLogsResponse>(
    `/dns-providers/${providerId}/audit-logs?${params.toString()}`
  )
  return response.data
}

/**
 * Exports audit logs to CSV format.
 * @param filters - Optional filters to apply
 * @returns Promise resolving to CSV string
 * @throws {AxiosError} If the request fails
 */
export async function exportAuditLogsCSV(filters?: AuditLogFilters): Promise<string> {
  const params = new URLSearchParams()

  if (filters) {
    if (filters.event_category) params.append('event_category', filters.event_category)
    if (filters.actor) params.append('actor', filters.actor)
    if (filters.action) params.append('action', filters.action)
    if (filters.start_date) params.append('start_date', filters.start_date)
    if (filters.end_date) params.append('end_date', filters.end_date)
    if (filters.resource_uuid) params.append('resource_uuid', filters.resource_uuid)
  }

  const response = await client.get<string>(
    `/audit-logs/export?${params.toString()}`,
    {
      headers: { Accept: 'text/csv' },
    }
  )
  return response.data
}
