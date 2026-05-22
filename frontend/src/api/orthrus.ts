import client from './client';

export type OrthrusStatus = 'online' | 'offline' | 'pending';

export interface OrthrusAgent {
  uuid: string;
  name: string;
  status: OrthrusStatus;
  capabilities: string;
  agent_cert_pem?: string;
  last_heartbeat?: string;
  last_seen?: string;
  created_at: string;
  updated_at: string;
  // Provider assignment — set via Assign Provider dialog
  hecate_tunnel_uuid?: string;
  device_id?: string;
  resolved_address?: string;
  // External Docker proxy port (0 = disabled, 1024–65535 = enabled)
  external_proxy_port: number;
}

export interface PatchAgentRequest {
  name?: string;
  hecate_tunnel_uuid?: string | null;
  device_id?: string | null;
  resolved_address?: string | null;
  external_proxy_port?: number;
}

export interface ExternalProxyStatus {
  agent_uuid: string;
  agent_online: boolean;
  configured_port: number;
  active: boolean;
  active_port: number;
  bind_address: string;
  connection_string: string;
  error: string;
}

export interface ProvisionAgentRequest {
  name: string;
}

export interface ProvisionAgentResponse {
  agent: OrthrusAgent;
  auth_key: string;
}

/** @deprecated Use ProvisionAgentResponse */
export type ProvisionResponse = ProvisionAgentResponse;

export interface InstallSnippets {
  docker_compose: string;
  systemd: string;
  tarball: string;
  homebrew: string;
  kubernetes_daemonset: string;
}

export const listAgents = async (): Promise<OrthrusAgent[]> => {
  const { data } = await client.get<OrthrusAgent[]>('/orthrus/agents');
  return data;
};

export const provisionAgent = async (req: ProvisionAgentRequest): Promise<ProvisionAgentResponse> => {
  const { data } = await client.post<ProvisionAgentResponse>('/orthrus/agents', req);
  return data;
};

export const getAgent = async (uuid: string): Promise<OrthrusAgent> => {
  const { data } = await client.get<OrthrusAgent>(`/orthrus/agents/${uuid}`);
  return data;
};

export const deleteAgent = async (uuid: string): Promise<void> => {
  await client.delete(`/orthrus/agents/${uuid}`);
};

export const renameAgent = async (uuid: string, name: string): Promise<OrthrusAgent> =>
  patchAgent(uuid, { name });

export const patchAgent = async (uuid: string, req: PatchAgentRequest): Promise<OrthrusAgent> => {
  const { data } = await client.patch<OrthrusAgent>(`/orthrus/agents/${uuid}`, req);
  return data;
};

export const revokeAgent = async (uuid: string): Promise<{ message: string }> => {
  const { data } = await client.post<{ message: string }>(`/orthrus/agents/${uuid}/revoke`);
  return data;
};

export const getInstallSnippets = async (uuid: string): Promise<InstallSnippets> => {
  const { data } = await client.get<InstallSnippets>(`/orthrus/agents/${uuid}/snippets`, {
    headers: { 'X-Charon-URL': window.location.origin },
  });
  return data;
};

export const getAgentProxyStatus = async (agentUUID: string): Promise<ExternalProxyStatus> => {
  const { data } = await client.get<ExternalProxyStatus>(`/orthrus/agents/${agentUUID}/proxy-status`);
  return data;
};
