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
 * One heartbeat as embedded in `MonitorSummary.recent_beats`.
 * Chronological ASC ordering (oldest first, newest last). (§3.5.2)
 */
export interface BeatDTO {
  status: string;
  latency: number;
  created_at: string;
}

/**
 * A single row of `GET /uptime/monitors/summary` — the batch payload that
 * replaced the per-card history N+1. (§3.5.2 / §3.5.7)
 *
 * `proxy_host_id` / `remote_server_id` are always present but nullable (UI
 * grouping only). `uptime_24h` is always present, null when there is no
 * heartbeat data in the 24h window. `max_retries` is always present (legacy
 * rows with an unset value resolve to the effective default 3) so the
 * Edit-monitor modal round-trips it instead of resetting it on save.
 */
export interface MonitorSummary {
  id: string;
  name: string;
  type: string;
  url: string;
  enabled: boolean;
  status: string;
  latency: number;
  last_check: string | null;
  interval: number;
  proxy_host_id?: number | null;
  remote_server_id?: number | null;
  uptime_24h: number | null;
  recent_beats: BeatDTO[];
  max_retries: number;
}

/** `GET /uptime/health` ingester + worker-pool health signal. (§3.5.5) */
export interface UptimeHealth {
  heartbeats_dropped: number;
  checks_enqueue_dropped: number;
  queue_depth: number;
  worker_pool_size: number;
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
 * Fetches the batch monitor summary — one request that carries every monitor's
 * metadata, resolved status, latency, 24h uptime and its most recent beats.
 * Replaces the per-card history fan-out on the Uptime page. (§3.5.1 / §3.5.7)
 * @param beats - Recent beats per monitor to include (1..60, default 30)
 * @returns Promise resolving to an array of MonitorSummary objects
 * @throws {AxiosError} If the request fails
 */
export const getMonitorsSummary = async (beats = 30): Promise<MonitorSummary[]> => {
  const response = await client.get<MonitorSummary[]>(`/uptime/monitors/summary?beats=${beats}`);
  return response.data;
};

/**
 * Fetches the uptime ingester / worker-pool health signal. (§3.5.5)
 * @returns Promise resolving to UptimeHealth
 * @throws {AxiosError} If the request fails
 */
export const getUptimeHealth = async (): Promise<UptimeHealth> => {
  const response = await client.get<UptimeHealth>('/uptime/health');
  return response.data;
};

/**
 * Fetches heartbeat history for a monitor. Used only by the expanded / detail
 * view — the list page reads `getMonitorsSummary` instead.
 * @param id - The monitor ID
 * @param limit - Maximum number of heartbeats to return (default: 50, server cap 500)
 * @param before - Optional RFC3339 cursor; returns beats with created_at < before
 * @returns Promise resolving to array of UptimeHeartbeat objects
 * @throws {AxiosError} If the request fails or monitor not found
 */
export const getMonitorHistory = async (id: string, limit: number = 50, before?: string) => {
  const query = before
    ? `?limit=${limit}&before=${encodeURIComponent(before)}`
    : `?limit=${limit}`;
  const response = await client.get<UptimeHeartbeat[]>(`/uptime/monitors/${id}/history${query}`);
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
 * @returns Promise resolving to sync result with message; `enqueued` / `dropped`
 *   are populated when the sync fans work through the bounded worker pool and
 *   some checks could not be queued (N5).
 * @throws {AxiosError} If sync fails
 */
export async function syncMonitors(
  body?: { interval?: number; max_retries?: number },
): Promise<{ message: string; enqueued?: number; dropped?: number }> {
  const res = await client.post<{ message: string; enqueued?: number; dropped?: number }>(
    '/uptime/sync',
    body || {},
  );
  return res.data;
}

/**
 * Triggers an immediate check for a monitor.
 * @param id - The monitor ID to check
 * @returns Promise resolving to object with result message
 * @throws {AxiosError} If check fails or monitor not found. Rejects with a
 *   `503` ("check queue is full, try again") when the worker pool is saturated
 *   — callers surface this as a queue-full toast rather than a generic error (N5).
 */
export const checkMonitor = async (id: string) => {
  const response = await client.post<{ message: string }>(`/uptime/monitors/${id}/check`);
  return response.data;
};
