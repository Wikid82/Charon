import client from './client';

export interface NotificationProvider {
  id: string;
  name: string;
  type: string;
  url: string;
  config?: string;
  enabled: boolean;
  notify_proxy_hosts: boolean;
  notify_remote_servers: boolean;
  notify_domains: boolean;
  notify_certs: boolean;
  notify_uptime: boolean;
  created_at: string;
}

export const getProviders = async () => {
  const response = await client.get<NotificationProvider[]>('/notifications/providers');
  return response.data;
};

export const createProvider = async (data: Partial<NotificationProvider>) => {
  const response = await client.post<NotificationProvider>('/notifications/providers', data);
  return response.data;
};

export const updateProvider = async (id: string, data: Partial<NotificationProvider>) => {
  const response = await client.put<NotificationProvider>(`/notifications/providers/${id}`, data);
  return response.data;
};

export const deleteProvider = async (id: string) => {
  await client.delete(`/notifications/providers/${id}`);
};

export const testProvider = async (provider: Partial<NotificationProvider>) => {
  await client.post('/notifications/providers/test', provider);
};
