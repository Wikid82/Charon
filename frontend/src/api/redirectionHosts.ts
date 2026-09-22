import client from './client';

/** Redirect status codes Charon supports for Redirection Hosts (spec §5). */
export type RedirectStatusCode = 301 | 302 | 307 | 308;

export interface RedirectionHostCertificate {
  id?: number;
  uuid: string;
  name: string;
  provider: string;
  domains: string;
  expires_at: string;
}

export interface RedirectionHostDNSProvider {
  uuid: string;
  name: string;
  provider_type: string;
  is_default: boolean;
}

export interface RedirectionHost {
  uuid: string;
  name: string;
  domain_names: string;
  target_url: string;
  status_code: RedirectStatusCode;
  preserve_path: boolean;
  ssl_forced: boolean;
  http2_support: boolean;
  hsts_enabled: boolean;
  hsts_subdomains: boolean;
  enabled: boolean;
  certificate_id?: number | string | null;
  certificate?: RedirectionHostCertificate | null;
  dns_provider_id?: number | string | null;
  dns_provider?: RedirectionHostDNSProvider | null;
  use_dns_challenge: boolean;
  created_at: string;
  updated_at: string;
}

/**
 * Plain-language status code options for the Status Code selector (spec §5,
 * §4.5). Only the four codes Charon supports are offered — see
 * docs/plans/current_spec.md §5 for why 300/303/304 are excluded.
 */
export const REDIRECT_STATUS_CODES: { value: RedirectStatusCode; label: string }[] = [
  { value: 301, label: '301 - Permanent' },
  { value: 302, label: '302 - Temporary' },
  { value: 307, label: '307 - Temporary (preserve method)' },
  { value: 308, label: '308 - Permanent (preserve method)' },
];

export const redirectionHostsApi = {
  list: async (): Promise<RedirectionHost[]> => {
    const { data } = await client.get<RedirectionHost[]>('/redirection-hosts');
    return data;
  },

  get: async (uuid: string): Promise<RedirectionHost> => {
    const { data } = await client.get<RedirectionHost>(`/redirection-hosts/${uuid}`);
    return data;
  },

  create: async (payload: Partial<RedirectionHost>): Promise<RedirectionHost> => {
    const { data } = await client.post<RedirectionHost>('/redirection-hosts', payload);
    return data;
  },

  update: async (uuid: string, payload: Partial<RedirectionHost>): Promise<RedirectionHost> => {
    const { data } = await client.put<RedirectionHost>(`/redirection-hosts/${uuid}`, payload);
    return data;
  },

  delete: async (uuid: string): Promise<void> => {
    await client.delete(`/redirection-hosts/${uuid}`);
  },
};
