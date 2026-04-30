import client from './client';

/** Remote server configuration for Docker host connections. */
export interface RemoteServer {
  uuid: string;
  name: string;
  provider: string;
  host: string;
  port: number;
  username?: string;
  enabled: boolean;
  reachable: boolean;
  last_check?: string;
  created_at: string;
  updated_at: string;
  connection_type?: 'direct' | 'orthrus' | 'cloudflare' | 'tailscale' | 'netbird' | 'zerotier';
  orthrus_agent_uuid?: string;
  hecate_tunnel_uuid?: string;
}

/**
 * Fetches all remote servers.
 * @param enabledOnly - If true, only returns enabled servers
 * @returns Promise resolving to array of RemoteServer objects
 * @throws {AxiosError} If the request fails
 */
export const getRemoteServers = async (enabledOnly = false): Promise<RemoteServer[]> => {
  const params = enabledOnly ? { enabled: true } : {};
  const { data } = await client.get<RemoteServer[]>('/remote-servers', { params });
  return data;
};

/**
 * Fetches a single remote server by UUID.
 * @param uuid - The unique identifier of the remote server
 * @returns Promise resolving to the RemoteServer object
 * @throws {AxiosError} If the request fails or server not found
 */
export const getRemoteServer = async (uuid: string): Promise<RemoteServer> => {
  const { data } = await client.get<RemoteServer>(`/remote-servers/${uuid}`);
  return data;
};

/**
 * Creates a new remote server.
 * @param server - Partial RemoteServer configuration
 * @returns Promise resolving to the created RemoteServer
 * @throws {AxiosError} If creation fails
 */
export const createRemoteServer = async (server: Partial<RemoteServer>): Promise<RemoteServer> => {
  const { data } = await client.post<RemoteServer>('/remote-servers', server);
  return data;
};

/**
 * Updates an existing remote server.
 * @param uuid - The unique identifier of the server to update
 * @param server - Partial RemoteServer with fields to update
 * @returns Promise resolving to the updated RemoteServer
 * @throws {AxiosError} If update fails or server not found
 */
export const updateRemoteServer = async (uuid: string, server: Partial<RemoteServer>): Promise<RemoteServer> => {
  const { data } = await client.put<RemoteServer>(`/remote-servers/${uuid}`, server);
  return data;
};

/**
 * Deletes a remote server.
 * @param uuid - The unique identifier of the server to delete
 * @throws {AxiosError} If deletion fails or server not found
 */
export const deleteRemoteServer = async (uuid: string): Promise<void> => {
  await client.delete(`/remote-servers/${uuid}`);
};

/**
 * Tests connectivity to an existing remote server.
 * @param uuid - The unique identifier of the server to test
 * @returns Promise resolving to object with server address
 * @throws {AxiosError} If connection test fails
 */
export const testRemoteServerConnection = async (uuid: string): Promise<{ address: string }> => {
  const { data } = await client.post<{ address: string }>(`/remote-servers/${uuid}/test`);
  return data;
};

/**
 * Tests connectivity to a custom host and port.
 * @param host - The hostname or IP to test
 * @param port - The port number to test
 * @returns Promise resolving to connection result with reachable status
 * @throws {AxiosError} If request fails
 */
export const testCustomRemoteServerConnection = async (host: string, port: number): Promise<{ address: string; reachable: boolean; error?: string }> => {
  const { data } = await client.post<{ address: string; reachable: boolean; error?: string }>('/remote-servers/test', { host, port });
  return data;
};
