import client from './client';

/** Uptime monitor configuration. */
export interface UptimeMonitor {
  id: string;
  upstream_host?: string;
  proxy_host_id?: number;
  remote_server_id?: number;
  name: string;
  type: string;
  url: string;
  interval: number;
  enabled: boolean;
  status: string;
  last_check?: string | null;
  latency: number;
  max_retries: number;
}

/** Uptime heartbeat (check result) entry. */
export interface UptimeHeartbeat {
  id: number;
  monitor_id: string;
  status: string;
  latency: number;
  message: string;
  created_at: string;
}

/**
 * Fetches all uptime monitors.
 * @returns Promise resolving to array of UptimeMonitor objects
 * @throws {AxiosError} If the request fails
 */
export const getMonitors = async () => {
  const response = await client.get<UptimeMonitor[]>('/uptime/monitors');
  return response.data;
};

/**
 * Fetches heartbeat history for a monitor.
 * @param id - The monitor ID
 * @param limit - Maximum number of heartbeats to return (default: 50)
 * @returns Promise resolving to array of UptimeHeartbeat objects
 * @throws {AxiosError} If the request fails or monitor not found
 */
export const getMonitorHistory = async (id: string, limit: number = 50) => {
  const response = await client.get<UptimeHeartbeat[]>(`/uptime/monitors/${id}/history?limit=${limit}`);
  return response.data;
};

/**
 * Updates an uptime monitor configuration.
 * @param id - The monitor ID to update
 * @param data - Partial UptimeMonitor with fields to update
 * @returns Promise resolving to the updated UptimeMonitor
 * @throws {AxiosError} If update fails or monitor not found
 */
export const updateMonitor = async (id: string, data: Partial<UptimeMonitor>) => {
  const response = await client.put<UptimeMonitor>(`/uptime/monitors/${id}`, data);
  return response.data;
};

/**
 * Deletes an uptime monitor.
 * @param id - The monitor ID to delete
 * @returns Promise resolving to void
 * @throws {AxiosError} If deletion fails or monitor not found
 */
export const deleteMonitor = async (id: string) => {
  const response = await client.delete<void>(`/uptime/monitors/${id}`);
  return response.data;
};

/**
 * Creates a new uptime monitor.
 * @param data - Monitor configuration (name, url, type, interval, max_retries)
 * @returns Promise resolving to the created UptimeMonitor
 * @throws {AxiosError} If creation fails
 */
export const createMonitor = async (data: {
  name: string;
  url: string;
  type: string;
  interval?: number;
  max_retries?: number;
}): Promise<UptimeMonitor> => {
  const response = await client.post<UptimeMonitor>('/uptime/monitors', data);
  return response.data;
};

/**
 * Syncs monitors with proxy hosts and remote servers.
 * @param body - Optional configuration for sync (interval, max_retries)
 * @returns Promise resolving to sync result with message
 * @throws {AxiosError} If sync fails
 */
export async function syncMonitors(body?: { interval?: number; max_retries?: number }): Promise<{ message: string }> {
  const res = await client.post<{ message: string }>('/uptime/sync', body || {});
  return res.data;
}

/**
 * Triggers an immediate check for a monitor.
 * @param id - The monitor ID to check
 * @returns Promise resolving to object with result message
 * @throws {AxiosError} If check fails or monitor not found
 */
export const checkMonitor = async (id: string) => {
  const response = await client.post<{ message: string }>(`/uptime/monitors/${id}/check`);
  return response.data;
};
