import client from './client';

export interface Location {
  uuid?: string;
  path: string;
  forward_scheme: string;
  forward_host: string;
  forward_port: number;
}

export interface Certificate {
  id: number;
  uuid: string;
  name: string;
  provider: string;
  domains: string;
  expires_at: string;
}

export type ApplicationPreset = 'none' | 'plex' | 'jellyfin' | 'emby' | 'homeassistant' | 'nextcloud' | 'vaultwarden';

export interface ProxyHost {
  uuid: string;
  name: string;
  domain_names: string;
  forward_scheme: string;
  forward_host: string;
  forward_port: number;
  ssl_forced: boolean;
  http2_support: boolean;
  hsts_enabled: boolean;
  hsts_subdomains: boolean;
  block_exploits: boolean;
  websocket_support: boolean;
  application: ApplicationPreset;
  locations: Location[];
  advanced_config?: string;
  advanced_config_backup?: string;
  enabled: boolean;
  certificate_id?: number | null;
  certificate?: Certificate | null;
  access_list_id?: number | null;
  created_at: string;
  updated_at: string;
}

export const getProxyHosts = async (): Promise<ProxyHost[]> => {
  const { data } = await client.get<ProxyHost[]>('/proxy-hosts');
  return data;
};

export const getProxyHost = async (uuid: string): Promise<ProxyHost> => {
  const { data } = await client.get<ProxyHost>(`/proxy-hosts/${uuid}`);
  return data;
};

export const createProxyHost = async (host: Partial<ProxyHost>): Promise<ProxyHost> => {
  const { data } = await client.post<ProxyHost>('/proxy-hosts', host);
  return data;
};

export const updateProxyHost = async (uuid: string, host: Partial<ProxyHost>): Promise<ProxyHost> => {
  const { data } = await client.put<ProxyHost>(`/proxy-hosts/${uuid}`, host);
  return data;
};

export const deleteProxyHost = async (uuid: string): Promise<void> => {
  await client.delete(`/proxy-hosts/${uuid}`);
};

export const testProxyHostConnection = async (host: string, port: number): Promise<void> => {
  await client.post('/proxy-hosts/test', { forward_host: host, forward_port: port });
};

export interface BulkUpdateACLRequest {
  host_uuids: string[];
  access_list_id: number | null;
}

export interface BulkUpdateACLResponse {
  updated: number;
  errors: { uuid: string; error: string }[];
}

export const bulkUpdateACL = async (
  hostUUIDs: string[],
  accessListID: number | null
): Promise<BulkUpdateACLResponse> => {
  const { data } = await client.put<BulkUpdateACLResponse>('/proxy-hosts/bulk-update-acl', {
    host_uuids: hostUUIDs,
    access_list_id: accessListID,
  });
  return data;
};
