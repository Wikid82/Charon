import { useQuery } from '@tanstack/react-query'
import {
  getAuditLogs,
  getAuditLog,
  getAuditLogsByProvider,
  type AuditLog,
  type AuditLogFilters,
} from '../api/auditLogs'

/** Query key factory for audit logs */
const queryKeys = {
  all: ['audit-logs'] as const,
  lists: () => [...queryKeys.all, 'list'] as const,
  list: (filters?: AuditLogFilters, page?: number, limit?: number) =>
    [...queryKeys.lists(), filters, page, limit] as const,
  details: () => [...queryKeys.all, 'detail'] as const,
  detail: (uuid: string) => [...queryKeys.details(), uuid] as const,
  byProvider: (providerId: number, page?: number, limit?: number) =>
    [...queryKeys.all, 'provider', providerId, page, limit] as const,
}

/**
 * Hook for fetching audit logs with pagination and filtering.
 * @param filters - Optional filters to apply
 * @param page - Page number (1-indexed)
 * @param limit - Number of records per page
 * @returns Query result with paginated audit logs
 */
export function useAuditLogs(
  filters?: AuditLogFilters,
  page: number = 1,
  limit: number = 50
) {
  return useQuery({
    queryKey: queryKeys.list(filters, page, limit),
    queryFn: () => getAuditLogs(filters, page, limit),
    staleTime: 1000 * 30, // 30 seconds - audit logs are relatively static
    placeholderData: (previousData) => previousData, // Keep previous data while fetching new page
  })
}

/**
 * Hook for fetching a single audit log.
 * @param uuid - Audit log UUID
 * @returns Query result with audit log data
 */
export function useAuditLog(uuid: string | null) {
  return useQuery({
    queryKey: queryKeys.detail(uuid || ''),
    queryFn: () => getAuditLog(uuid!),
    enabled: !!uuid,
  })
}

/**
 * Hook for fetching audit logs for a specific DNS provider.
 * @param providerId - DNS provider ID
 * @param page - Page number (1-indexed)
 * @param limit - Number of records per page
 * @returns Query result with paginated audit logs
 */
export function useAuditLogsByProvider(
  providerId: number | null,
  page: number = 1,
  limit: number = 50
) {
  return useQuery({
    queryKey: queryKeys.byProvider(providerId || 0, page, limit),
    queryFn: () => getAuditLogsByProvider(providerId!, page, limit),
    enabled: providerId !== null && providerId > 0,
    staleTime: 1000 * 30,
    placeholderData: (previousData) => previousData,
  })
}

export type { AuditLog, AuditLogFilters }
