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
  forward_auth_enabled: boolean;
  forward_auth_bypass: string;
  auth_policy_id?: number | null;
  locations: Location[];
  advanced_config?: string;
  enabled: boolean;
  certificate_id?: number | null;
  certificate?: Certificate | null;
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
