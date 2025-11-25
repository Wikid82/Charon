import client from './client';

export interface ForwardAuthConfig {
  id?: number;
  provider: 'authelia' | 'authentik' | 'pomerium' | 'custom';
  address: string;
  trust_forward_header: boolean;
  created_at?: string;
  updated_at?: string;
}

export interface ForwardAuthTemplate {
  provider: string;
  address: string;
  trust_forward_header: boolean;
  description: string;
}

export const getForwardAuthConfig = async (): Promise<ForwardAuthConfig> => {
  const { data } = await client.get<ForwardAuthConfig>('/security/forward-auth');
  return data;
};

export const updateForwardAuthConfig = async (config: ForwardAuthConfig): Promise<ForwardAuthConfig> => {
  const { data } = await client.put<ForwardAuthConfig>('/security/forward-auth', config);
  return data;
};

export const getForwardAuthTemplates = async (): Promise<Record<string, ForwardAuthTemplate>> => {
  const { data } = await client.get<Record<string, ForwardAuthTemplate>>('/security/forward-auth/templates');
  return data;
};
