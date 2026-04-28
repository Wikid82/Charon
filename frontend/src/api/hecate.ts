import client from './client';

export type TunnelProvider = 'cloudflare' | 'tailscale' | 'zerotier' | 'netbird';
export type TunnelState = 'connected' | 'connecting' | 'error' | 'stopped';

export interface TunnelConfig {
  uuid: string;
  name: string;
  provider: TunnelProvider;
  configuration: string;
  is_active: boolean;
  created_at: string;
  updated_at: string;
  // NOTE: credentials are NEVER returned; only sent on create/update
}

export interface TunnelStatus {
  uuid: string;
  name: string;
  provider: TunnelProvider;
  state: TunnelState;
  uptime_seconds: number;
  last_error: string;
}

export interface CreateTunnelRequest {
  name: string;
  provider: TunnelProvider;
  credentials: string;
  configuration?: string;
  is_active?: boolean;
}

export interface UpdateTunnelRequest {
  name: string;
  provider: TunnelProvider;
  credentials?: string;
  configuration?: string;
  is_active?: boolean;
}

export interface CloudflareTunnel {
  id: string;
  name: string;
  status: string;
  created_at: string;
}

export interface TailscaleDevice {
  id: string;
  hostname: string;
  addresses: string[];
  os: string;
  last_seen: string;
  online: boolean;
}

export interface ZeroTierNetwork {
  id: string;
  name: string;
  description: string;
  private: boolean;
  total_member_count: number;
}

export interface ZeroTierMember {
  node_id: string;
  name: string;
  description: string;
  ip_assignments: string[];
  authorized: boolean;
  online: boolean;
}

export interface NetBirdPeer {
  id: string;
  name: string;
  ip: string;
  os: string;
  connection_state: string;
  last_seen: string;
  online: boolean;
}

export const getTunnelStatus = async (): Promise<TunnelStatus[]> => {
  const { data } = await client.get<TunnelStatus[]>('/hecate/status');
  return data;
};

export const listTunnels = async (): Promise<TunnelConfig[]> => {
  const { data } = await client.get<TunnelConfig[]>('/hecate/tunnels');
  return data;
};

export const createTunnel = async (req: CreateTunnelRequest): Promise<TunnelConfig> => {
  const { data } = await client.post<TunnelConfig>('/hecate/tunnels', req);
  return data;
};

export const getTunnel = async (uuid: string): Promise<TunnelConfig> => {
  const { data } = await client.get<TunnelConfig>(`/hecate/tunnels/${uuid}`);
  return data;
};

export const updateTunnel = async (
  uuid: string,
  req: UpdateTunnelRequest,
): Promise<{ message: string }> => {
  const { data } = await client.put<{ message: string }>(`/hecate/tunnels/${uuid}`, req);
  return data;
};

export const deleteTunnel = async (uuid: string): Promise<void> => {
  await client.delete(`/hecate/tunnels/${uuid}`);
};

export const startTunnel = async (uuid: string): Promise<{ message: string }> => {
  const { data } = await client.post<{ message: string }>(`/hecate/tunnels/${uuid}/start`);
  return data;
};

export const stopTunnel = async (uuid: string): Promise<{ message: string }> => {
  const { data } = await client.post<{ message: string }>(`/hecate/tunnels/${uuid}/stop`);
  return data;
};

export const rotateCredentials = async (
  uuid: string,
  credentials: string,
): Promise<{ message: string }> => {
  const { data } = await client.post<{ message: string }>(
    `/hecate/tunnels/${uuid}/rotate-credentials`,
    { credentials },
  );
  return data;
};

export const listCloudflareTunnels = async (): Promise<CloudflareTunnel[]> => {
  const { data } = await client.get<CloudflareTunnel[]>('/hecate/cloudflare/tunnels');
  return data;
};

export const getCloudflaredConfig = async (uuid: string): Promise<string> => {
  const { data } = await client.get<string>(`/hecate/tunnels/${uuid}/config/cloudflared`);
  return data;
};

export const listTailscaleDevices = async (): Promise<TailscaleDevice[]> => {
  const { data } = await client.get<TailscaleDevice[]>('/hecate/tailscale/devices');
  return data;
};

export const syncTailscale = async (): Promise<TailscaleDevice[]> => {
  const { data } = await client.post<TailscaleDevice[]>('/hecate/tailscale/sync');
  return data;
};

export const listZeroTierNetworks = async (): Promise<ZeroTierNetwork[]> => {
  const { data } = await client.get<ZeroTierNetwork[]>('/hecate/zerotier/networks');
  return data;
};

export const listZeroTierMembers = async (networkId: string): Promise<ZeroTierMember[]> => {
  const { data } = await client.get<ZeroTierMember[]>(
    `/hecate/zerotier/networks/${networkId}/members`,
  );
  return data;
};

export const listNetBirdPeers = async (): Promise<NetBirdPeer[]> => {
  const { data } = await client.get<NetBirdPeer[]>('/hecate/netbird/peers');
  return data;
};

export const syncNetBird = async (): Promise<NetBirdPeer[]> => {
  const { data } = await client.post<NetBirdPeer[]>('/hecate/netbird/sync');
  return data;
};

/**
 * Opens a WebSocket connection to stream real-time logs for a tunnel.
 * Authentication is handled via HttpOnly cookies sent automatically by the browser.
 * @param uuid - Tunnel UUID to stream logs for
 * @param onMessage - Callback invoked for each received log line
 * @returns The WebSocket instance for caller lifecycle management
 */
export const connectTunnelLogs = (uuid: string, onMessage: (line: string) => void): WebSocket => {
  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
  const wsUrl = `${protocol}//${window.location.host}/api/v1/ws/hecate/logs/${uuid}`;

  const ws = new WebSocket(wsUrl);

  ws.onmessage = (event: MessageEvent) => {
    onMessage(event.data as string);
  };

  return ws;
};
