import client from './client';

export interface NotificationProvider {
  id: string;
  name: string;
  type: string;
  url: string;
  config?: string;
  template?: string;
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

export const getTemplates = async () => {
  const response = await client.get<NotificationTemplate[]>('/notifications/templates');
  return response.data;
};

export interface NotificationTemplate {
  id: string;
  name: string;
}

export const previewProvider = async (provider: Partial<NotificationProvider>, data?: Record<string, unknown>) => {
  const payload: Record<string, unknown> = { ...provider } as Record<string, unknown>;
  if (data) payload.data = data;
  const response = await client.post('/notifications/providers/preview', payload);
  return response.data;
};

// External (saved) templates API
export interface ExternalTemplate {
  id: string;
  name: string;
  description?: string;
  config?: string;
  template?: string;
  created_at?: string;
}

export const getExternalTemplates = async () => {
  const response = await client.get<ExternalTemplate[]>('/notifications/external-templates');
  return response.data;
};

export const createExternalTemplate = async (data: Partial<ExternalTemplate>) => {
  const response = await client.post<ExternalTemplate>('/notifications/external-templates', data);
  return response.data;
};

export const updateExternalTemplate = async (id: string, data: Partial<ExternalTemplate>) => {
  const response = await client.put<ExternalTemplate>(`/notifications/external-templates/${id}`, data);
  return response.data;
};

export const deleteExternalTemplate = async (id: string) => {
  await client.delete(`/notifications/external-templates/${id}`);
};

export const previewExternalTemplate = async (templateId?: string, template?: string, data?: Record<string, unknown>) => {
  const payload: Record<string, unknown> = {};
  if (templateId) payload.template_id = templateId;
  if (template) payload.template = template;
  if (data) payload.data = data;
  const response = await client.post('/notifications/external-templates/preview', payload);
  return response.data;
};
