import client from './client';

export interface Location {
  uuid?: string;
  path: string;
  forward_scheme: string;
  forward_host: string;
  forward_port: number;
}

export interface Certificate {
  id?: number;
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
  enable_standard_headers?: boolean;
  forward_auth_enabled?: boolean;
  waf_disabled?: boolean;
  application: ApplicationPreset;
  locations: Location[];
  advanced_config?: string;
  advanced_config_backup?: string;
  enabled: boolean;
  certificate_id?: number | string | null;
  certificate?: Certificate | null;
  access_list_id?: number | string | null;
  access_list?: {
    uuid: string;
    name: string;
    description: string;
    type: string;
  } | null;
  proxy_group_id?: number | string | null;
  proxy_group?: {
    uuid: string;
    name: string;
    color: string;
  } | null;
  security_header_profile_id?: number | string | null;
  dns_provider_id?: number | null;
  security_header_profile?: {
    id?: number;
    uuid: string;
    name: string;
    description: string;
    security_score: number;
    is_preset: boolean;
  } | null;
  created_at: string;
  updated_at: string;
}

/**
 * Fetches all proxy hosts from the API.
 * @returns Promise resolving to array of ProxyHost objects
 * @throws {AxiosError} If the request fails
 */
export const getProxyHosts = async (): Promise<ProxyHost[]> => {
  const { data } = await client.get<ProxyHost[]>('/proxy-hosts');
  return data;
};

/**
 * Fetches a single proxy host by UUID.
 * @param uuid - The unique identifier of the proxy host
 * @returns Promise resolving to the ProxyHost object
 * @throws {AxiosError} If the request fails or host not found
 */
export const getProxyHost = async (uuid: string): Promise<ProxyHost> => {
  const { data } = await client.get<ProxyHost>(`/proxy-hosts/${uuid}`);
  return data;
};

/**
 * Creates a new proxy host.
 * @param host - Partial ProxyHost object with configuration
 * @returns Promise resolving to the created ProxyHost
 * @throws {AxiosError} If the request fails or validation errors occur
 */
export const createProxyHost = async (host: Partial<ProxyHost>): Promise<ProxyHost> => {
  const { data } = await client.post<ProxyHost>('/proxy-hosts', host);
  return data;
};

/**
 * Updates an existing proxy host.
 * @param uuid - The unique identifier of the proxy host to update
 * @param host - Partial ProxyHost object with fields to update
 * @returns Promise resolving to the updated ProxyHost
 * @throws {AxiosError} If the request fails or host not found
 */
export const updateProxyHost = async (uuid: string, host: Partial<ProxyHost>): Promise<ProxyHost> => {
  const { data } = await client.put<ProxyHost>(`/proxy-hosts/${uuid}`, host);
  return data;
};

/**
 * Deletes a proxy host.
 * @param uuid - The unique identifier of the proxy host to delete
 * @param deleteUptime - Optional flag to also delete associated uptime monitors
 * @throws {AxiosError} If the request fails or host not found
 */
export const deleteProxyHost = async (uuid: string, deleteUptime?: boolean): Promise<void> => {
  const url = `/proxy-hosts/${uuid}${deleteUptime ? '?delete_uptime=true' : ''}`
  await client.delete(url);
};

/**
 * Tests connectivity to a backend host.
 * @param host - The hostname or IP address to test
 * @param port - The port number to test
 * @throws {AxiosError} If the connection test fails
 */
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

/**
 * Bulk updates access control list assignments for multiple proxy hosts.
 * @param hostUUIDs - Array of proxy host UUIDs to update
 * @param accessListID - The access list ID to assign, or null to remove
 * @returns Promise resolving to the bulk update result with success/error counts
 * @throws {AxiosError} If the request fails
 */
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

export interface BulkUpdateSecurityHeadersRequest {
  host_uuids: string[];
  security_header_profile_id: number | null;
}

export interface BulkUpdateSecurityHeadersResponse {
  updated: number;
  errors: { uuid: string; error: string }[];
}

/**
 * Bulk updates security header profile assignments for multiple proxy hosts.
 * @param hostUUIDs - Array of proxy host UUIDs to update
 * @param securityHeaderProfileId - The security header profile ID to assign, or null to remove
 * @returns Promise resolving to the bulk update result with success/error counts
 * @throws {AxiosError} If the request fails
 */
export const bulkUpdateSecurityHeaders = async (
  hostUUIDs: string[],
  securityHeaderProfileId: number | null
): Promise<BulkUpdateSecurityHeadersResponse> => {
  const { data } = await client.put<BulkUpdateSecurityHeadersResponse>(
    '/proxy-hosts/bulk-update-security-headers',
    {
      host_uuids: hostUUIDs,
      security_header_profile_id: securityHeaderProfileId,
    }
  );
  return data;
};

export interface BulkUpdateGroupRequest {
  host_uuids: string[]
  proxy_group_id: string | null
}

export interface BulkUpdateGroupResponse {
  updated: number
  errors: { uuid: string; error: string }[]
}

export const bulkUpdateGroup = async (
  hostUUIDs: string[],
  proxyGroupId: string | null,
): Promise<BulkUpdateGroupResponse> => {
  const { data } = await client.put<BulkUpdateGroupResponse>(
    '/proxy-hosts/bulk-update-group',
    { host_uuids: hostUUIDs, proxy_group_id: proxyGroupId },
  )
  return data
}
