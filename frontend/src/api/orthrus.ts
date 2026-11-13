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
  // Listen setting for the external proxy: '' = automatic, '0.0.0.0' = all
  // networks, an address, or a CIDR subnet (see ExternalProxyBindChoice in the dialog).
  external_proxy_bind?: string;
  // Opt-in full, unrestricted Docker Engine API access through this agent's
  // External Docker Proxy — every endpoint, every request body, no
  // allowlist. false = read-only (default). This is a deliberate operator
  // trust decision, gated behind an explicit typed-confirmation UI step
  // (see AgentWriteModeDialog) — not something the app polices further.
  write_enabled: boolean;
}

export interface PatchAgentRequest {
  name?: string;
  hecate_tunnel_uuid?: string | null;
  device_id?: string | null;
  resolved_address?: string | null;
  external_proxy_port?: number;
  external_proxy_bind?: string;
  write_enabled?: boolean;
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
  // configured_bind is the saved listen setting; active_bind is the setting the
  // live session used. They differ until the agent reconnects.
  configured_bind: string;
  active_bind: string;
  bind_mode: ExternalProxyBindMode;
  // True when automatic mode found no network and fell back to loopback.
  bind_fallback: boolean;
  // configured_write_enabled reflects the DB value; active_write_enabled
  // reflects the value the currently-connected session actually negotiated
  // at its last reconnect. The two can differ if an operator toggled
  // write_enabled while the agent was already connected — see reconnectNotice.
  configured_write_enabled: boolean;
  active_write_enabled: boolean;
}

export type ExternalProxyBindMode = '' | 'auto' | 'all' | 'address' | 'subnet';

export type LocalNetworkSource = 'default_route' | 'single_interface' | 'loopback_fallback' | 'configured';

export interface LocalNetwork {
  interface: string;
  address: string;
  cidr: string;
  primary: boolean;
}

export interface LocalNetworks {
  auto: { address: string; source: LocalNetworkSource };
  networks: LocalNetwork[];
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

// rotateAgentKey replaces the agent's key. The returned key is shown once; the
// server disconnects the agent, which must reconnect with the new key.
export interface RotateAgentKeyResponse {
  auth_key: string;
}

export const rotateAgentKey = async (uuid: string): Promise<RotateAgentKeyResponse> => {
  const { data } = await client.post<RotateAgentKeyResponse>(`/orthrus/agents/${uuid}/rotate-key`);
  return data;
};

export const getInstallSnippets = async (uuid: string): Promise<InstallSnippets> => {
  const { data } = await client.get<InstallSnippets>(`/orthrus/agents/${uuid}/snippets`, {
    headers: { 'X-Charon-URL': window.location.origin },
  });
  return data;
};

export const getLocalNetworks = async (): Promise<LocalNetworks> => {
  const { data } = await client.get<LocalNetworks>('/orthrus/local-networks');
  return data;
};

export const getAgentProxyStatus = async (agentUUID: string): Promise<ExternalProxyStatus> => {
  const { data } = await client.get<ExternalProxyStatus>(`/orthrus/agents/${agentUUID}/proxy-status`);
  return data;
};
