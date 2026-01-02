import client from './client';

/** Information about a WebSocket connection. */
export interface ConnectionInfo {
  id: string;
  type: 'logs' | 'cerberus';
  connected_at: string;
  last_activity_at: string;
  remote_addr?: string;
  user_agent?: string;
  filters?: string;
}

/** Aggregate statistics for WebSocket connections. */
export interface ConnectionStats {
  total_active: number;
  logs_connections: number;
  cerberus_connections: number;
  oldest_connection?: string;
  last_updated: string;
}

/** Response containing WebSocket connections list. */
export interface ConnectionsResponse {
  connections: ConnectionInfo[];
  count: number;
}

/**
 * Gets all active WebSocket connections.
 * @returns Promise resolving to ConnectionsResponse with connections list
 * @throws {AxiosError} If the request fails
 */
export const getWebSocketConnections = async (): Promise<ConnectionsResponse> => {
  const response = await client.get('/websocket/connections');
  return response.data;
};

/**
 * Gets aggregate WebSocket connection statistics.
 * @returns Promise resolving to ConnectionStats
 * @throws {AxiosError} If the request fails
 */
export const getWebSocketStats = async (): Promise<ConnectionStats> => {
  const response = await client.get('/websocket/stats');
  return response.data;
};
