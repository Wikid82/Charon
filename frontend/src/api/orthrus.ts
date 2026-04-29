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

export const renameAgent = async (uuid: string, name: string): Promise<OrthrusAgent> => {
  const { data } = await client.patch<OrthrusAgent>(`/orthrus/agents/${uuid}`, { name });
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
