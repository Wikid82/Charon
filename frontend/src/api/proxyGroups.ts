import client from './client';

export interface ProxyGroup {
  uuid: string;
  name: string;
  description?: string;
  color?: string;
  host_count?: number;
  created_at: string;
  updated_at: string;
}

export interface CreateProxyGroupRequest {
  name: string;
  description?: string;
  color?: string;
}

export const proxyGroupsApi = {
  list: async (): Promise<ProxyGroup[]> => {
    const { data } = await client.get<ProxyGroup[]>('/proxy-groups');
    return data;
  },

  get: async (uuid: string): Promise<ProxyGroup> => {
    const { data } = await client.get<ProxyGroup>(`/proxy-groups/${uuid}`);
    return data;
  },

  create: async (payload: CreateProxyGroupRequest): Promise<ProxyGroup> => {
    const { data } = await client.post<ProxyGroup>('/proxy-groups', payload);
    return data;
  },

  update: async (uuid: string, payload: Partial<CreateProxyGroupRequest>): Promise<ProxyGroup> => {
    const { data } = await client.put<ProxyGroup>(`/proxy-groups/${uuid}`, payload);
    return data;
  },

  delete: async (uuid: string): Promise<void> => {
    await client.delete(`/proxy-groups/${uuid}`);
  },
};
