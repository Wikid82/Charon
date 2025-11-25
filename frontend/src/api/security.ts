import client from './client';

// --- Forward Auth (Legacy) ---

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

// --- Built-in SSO ---

// Users
export interface AuthUser {
  id: number;
  uuid: string;
  username: string;
  email: string;
  name: string;
  password?: string; // Only for creation/update
  roles: string;
  mfa_enabled: boolean;
  enabled: boolean;
  created_at: string;
  updated_at: string;
  additional_emails?: string;
}

export interface AuthUserStats {
  total_users: number;
  admin_users: number;
}

export interface CreateAuthUserRequest {
  username: string;
  email: string;
  name: string;
  password?: string;
  roles: string;
  mfa_enabled: boolean;
  additional_emails?: string;
}

export interface UpdateAuthUserRequest {
  email?: string;
  name?: string;
  password?: string;
  roles?: string;
  mfa_enabled?: boolean;
  enabled?: boolean;
  additional_emails?: string;
}

export const getAuthUsers = async (): Promise<AuthUser[]> => {
  const { data } = await client.get<AuthUser[]>('/security/users');
  return data;
};

export const getAuthUser = async (uuid: string): Promise<AuthUser> => {
  const { data } = await client.get<AuthUser>(`/security/users/${uuid}`);
  return data;
};

export const createAuthUser = async (user: CreateAuthUserRequest): Promise<AuthUser> => {
  const { data } = await client.post<AuthUser>('/security/users', user);
  return data;
};

export const updateAuthUser = async (uuid: string, user: UpdateAuthUserRequest): Promise<AuthUser> => {
  const { data } = await client.put<AuthUser>(`/security/users/${uuid}`, user);
  return data;
};

export const deleteAuthUser = async (uuid: string): Promise<void> => {
  await client.delete(`/security/users/${uuid}`);
};

export const getAuthUserStats = async (): Promise<AuthUserStats> => {
  const { data } = await client.get<AuthUserStats>('/security/users/stats');
  return data;
};

// Providers
export interface AuthProvider {
  id: number;
  uuid: string;
  name: string;
  type: 'google' | 'github' | 'oidc';
  client_id: string;
  client_secret?: string; // Only for creation/update
  issuer_url?: string;
  auth_url?: string;
  token_url?: string;
  user_info_url?: string;
  scopes?: string;
  role_mapping?: string;
  display_name?: string;
  enabled: boolean;
  created_at: string;
  updated_at: string;
}

export interface CreateAuthProviderRequest {
  name: string;
  type: 'google' | 'github' | 'oidc';
  client_id: string;
  client_secret: string;
  issuer_url?: string;
  auth_url?: string;
  token_url?: string;
  user_info_url?: string;
  scopes?: string;
  role_mapping?: string;
  display_name?: string;
}

export interface UpdateAuthProviderRequest {
  name?: string;
  type?: 'google' | 'github' | 'oidc';
  client_id?: string;
  client_secret?: string;
  issuer_url?: string;
  auth_url?: string;
  token_url?: string;
  user_info_url?: string;
  scopes?: string;
  role_mapping?: string;
  display_name?: string;
  enabled?: boolean;
}

export const getAuthProviders = async (): Promise<AuthProvider[]> => {
  const { data } = await client.get<AuthProvider[]>('/security/providers');
  return data;
};

export const getAuthProvider = async (uuid: string): Promise<AuthProvider> => {
  const { data } = await client.get<AuthProvider>(`/security/providers/${uuid}`);
  return data;
};

export const createAuthProvider = async (provider: CreateAuthProviderRequest): Promise<AuthProvider> => {
  const { data } = await client.post<AuthProvider>('/security/providers', provider);
  return data;
};

export const updateAuthProvider = async (uuid: string, provider: UpdateAuthProviderRequest): Promise<AuthProvider> => {
  const { data } = await client.put<AuthProvider>(`/security/providers/${uuid}`, provider);
  return data;
};

export const deleteAuthProvider = async (uuid: string): Promise<void> => {
  await client.delete(`/security/providers/${uuid}`);
};

// Policies
export interface AuthPolicy {
  id: number;
  uuid: string;
  name: string;
  description: string;
  allowed_roles: string;
  allowed_users: string;
  allowed_domains: string;
  require_mfa: boolean;
  session_timeout: number;
  enabled: boolean;
  created_at: string;
  updated_at: string;
}

export interface CreateAuthPolicyRequest {
  name: string;
  description?: string;
  allowed_roles?: string;
  allowed_users?: string;
  allowed_domains?: string;
  require_mfa?: boolean;
  session_timeout?: number;
}

export interface UpdateAuthPolicyRequest {
  name?: string;
  description?: string;
  allowed_roles?: string;
  allowed_users?: string;
  allowed_domains?: string;
  require_mfa?: boolean;
  session_timeout?: number;
  enabled?: boolean;
}

export const getAuthPolicies = async (): Promise<AuthPolicy[]> => {
  const { data } = await client.get<AuthPolicy[]>('/security/policies');
  return data;
};

export const getAuthPolicy = async (uuid: string): Promise<AuthPolicy> => {
  const { data } = await client.get<AuthPolicy>(`/security/policies/${uuid}`);
  return data;
};

export const createAuthPolicy = async (policy: CreateAuthPolicyRequest): Promise<AuthPolicy> => {
  const { data } = await client.post<AuthPolicy>('/security/policies', policy);
  return data;
};

export const updateAuthPolicy = async (uuid: string, policy: UpdateAuthPolicyRequest): Promise<AuthPolicy> => {
  const { data } = await client.put<AuthPolicy>(`/security/policies/${uuid}`, policy);
  return data;
};

export const deleteAuthPolicy = async (uuid: string): Promise<void> => {
  await client.delete(`/security/policies/${uuid}`);
};
