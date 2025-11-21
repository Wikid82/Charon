import client from './client';

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
}

export const getRemoteServers = async (enabledOnly = false): Promise<RemoteServer[]> => {
  const params = enabledOnly ? { enabled: true } : {};
  const { data } = await client.get<RemoteServer[]>('/remote-servers', { params });
  return data;
};

export const getRemoteServer = async (uuid: string): Promise<RemoteServer> => {
  const { data } = await client.get<RemoteServer>(`/remote-servers/${uuid}`);
  return data;
};

export const createRemoteServer = async (server: Partial<RemoteServer>): Promise<RemoteServer> => {
  const { data } = await client.post<RemoteServer>('/remote-servers', server);
  return data;
};

export const updateRemoteServer = async (uuid: string, server: Partial<RemoteServer>): Promise<RemoteServer> => {
  const { data } = await client.put<RemoteServer>(`/remote-servers/${uuid}`, server);
  return data;
};

export const deleteRemoteServer = async (uuid: string): Promise<void> => {
  await client.delete(`/remote-servers/${uuid}`);
};

export const testRemoteServerConnection = async (uuid: string): Promise<{ address: string }> => {
  const { data } = await client.post<{ address: string }>(`/remote-servers/${uuid}/test`);
  return data;
};
