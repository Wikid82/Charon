import client from './client';

export interface ConnectionInfo {
  id: string;
  type: 'logs' | 'cerberus';
  connected_at: string;
  last_activity_at: string;
  remote_addr?: string;
  user_agent?: string;
  filters?: string;
}

export interface ConnectionStats {
  total_active: number;
  logs_connections: number;
  cerberus_connections: number;
  oldest_connection?: string;
  last_updated: string;
}

export interface ConnectionsResponse {
  connections: ConnectionInfo[];
  count: number;
}

/**
 * Get all active WebSocket connections
 */
export const getWebSocketConnections = async (): Promise<ConnectionsResponse> => {
  const response = await client.get('/websocket/connections');
  return response.data;
};

/**
 * Get aggregate WebSocket connection statistics
 */
export const getWebSocketStats = async (): Promise<ConnectionStats> => {
  const response = await client.get('/websocket/stats');
  return response.data;
};
