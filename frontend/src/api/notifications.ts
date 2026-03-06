import client from './client';

export const SUPPORTED_NOTIFICATION_PROVIDER_TYPES = ['discord', 'gotify', 'webhook', 'email'] as const;
export type SupportedNotificationProviderType = (typeof SUPPORTED_NOTIFICATION_PROVIDER_TYPES)[number];
const DEFAULT_PROVIDER_TYPE: SupportedNotificationProviderType = 'discord';

const isSupportedNotificationProviderType = (type: string | undefined): type is SupportedNotificationProviderType =>
  typeof type === 'string' && SUPPORTED_NOTIFICATION_PROVIDER_TYPES.includes(type.toLowerCase() as SupportedNotificationProviderType);

const resolveProviderTypeOrThrow = (type: string | undefined): SupportedNotificationProviderType => {
  if (typeof type === 'undefined') {
    return DEFAULT_PROVIDER_TYPE;
  }

  const normalizedType = type.toLowerCase();
  if (isSupportedNotificationProviderType(normalizedType)) {
    return normalizedType;
  }

  throw new Error(`Unsupported notification provider type: ${type}`);
};

/** Notification provider configuration. */
export interface NotificationProvider {
  id: string;
  name: string;
  type: string;
  url: string;
  config?: string;
  template?: string;
  gotify_token?: string;
  token?: string;
  has_token?: boolean;
  enabled: boolean;
  notify_proxy_hosts: boolean;
  notify_remote_servers: boolean;
  notify_domains: boolean;
  notify_certs: boolean;
  notify_uptime: boolean;
  notify_security_waf_blocks: boolean;
  notify_security_acl_denies: boolean;
  notify_security_rate_limit_hits: boolean;
  managed_legacy_security?: boolean;
  created_at: string;
}

const sanitizeProviderForWriteAction = (data: Partial<NotificationProvider>): Partial<NotificationProvider> => {
  const type = resolveProviderTypeOrThrow(data.type);
  const payload: Partial<NotificationProvider> = {
    ...data,
    type,
  };

  const normalizedToken = typeof payload.gotify_token === 'string' && payload.gotify_token.trim().length > 0
    ? payload.gotify_token.trim()
    : typeof payload.token === 'string' && payload.token.trim().length > 0
      ? payload.token.trim()
      : undefined;

  delete payload.gotify_token;

  if (type !== 'gotify') {
    delete payload.token;
    return payload;
  }

  if (normalizedToken) {
    payload.token = normalizedToken;
  } else {
    delete payload.token;
  }

  return payload;
};

const sanitizeProviderForReadLikeAction = (data: Partial<NotificationProvider>): Partial<NotificationProvider> => {
  const payload = sanitizeProviderForWriteAction(data);
  delete payload.token;
  return payload;
};

/**
 * Fetches all notification providers.
 * @returns Promise resolving to array of NotificationProvider objects
 * @throws {AxiosError} If the request fails
 */
export const getProviders = async () => {
  const response = await client.get<NotificationProvider[]>('/notifications/providers');
  return response.data;
};

/**
 * Creates a new notification provider.
 * @param data - Partial NotificationProvider configuration
 * @returns Promise resolving to the created NotificationProvider
 * @throws {AxiosError} If creation fails
 */
export const createProvider = async (data: Partial<NotificationProvider>) => {
  const response = await client.post<NotificationProvider>('/notifications/providers', sanitizeProviderForWriteAction(data));
  return response.data;
};

/**
 * Updates an existing notification provider.
 * @param id - The provider ID to update
 * @param data - Partial NotificationProvider with fields to update
 * @returns Promise resolving to the updated NotificationProvider
 * @throws {AxiosError} If update fails or provider not found
 */
export const updateProvider = async (id: string, data: Partial<NotificationProvider>) => {
  const response = await client.put<NotificationProvider>(`/notifications/providers/${id}`, sanitizeProviderForWriteAction(data));
  return response.data;
};

/**
 * Deletes a notification provider.
 * @param id - The provider ID to delete
 * @throws {AxiosError} If deletion fails or provider not found
 */
export const deleteProvider = async (id: string) => {
  await client.delete(`/notifications/providers/${id}`);
};

/**
 * Tests a notification provider by sending a test message.
 * @param provider - Provider configuration to test
 * @throws {AxiosError} If test fails
 */
export const testProvider = async (provider: Partial<NotificationProvider>) => {
  await client.post('/notifications/providers/test', sanitizeProviderForReadLikeAction(provider));
};

/**
 * Fetches all available notification templates.
 * @returns Promise resolving to array of NotificationTemplate objects
 * @throws {AxiosError} If the request fails
 */
export const getTemplates = async () => {
  const response = await client.get<NotificationTemplate[]>('/notifications/templates');
  return response.data;
};

/** Notification template definition. */
export interface NotificationTemplate {
  id: string;
  name: string;
}

/**
 * Previews a notification with sample data.
 * @param provider - Provider configuration for preview
 * @param data - Optional sample data for template rendering
 * @returns Promise resolving to preview result
 * @throws {AxiosError} If preview fails
 */
export const previewProvider = async (provider: Partial<NotificationProvider>, data?: Record<string, unknown>) => {
  const payload: Record<string, unknown> = sanitizeProviderForReadLikeAction(provider) as Record<string, unknown>;
  if (data) payload.data = data;
  const response = await client.post('/notifications/providers/preview', payload);
  return response.data;
};

// External (saved) templates API
/** External notification template configuration. */
export interface ExternalTemplate {
  id: string;
  name: string;
  description?: string;
  config?: string;
  template?: string;
  created_at?: string;
}

/**
 * Fetches all external notification templates.
 * @returns Promise resolving to array of ExternalTemplate objects
 * @throws {AxiosError} If the request fails
 */
export const getExternalTemplates = async () => {
  const response = await client.get<ExternalTemplate[]>('/notifications/external-templates');
  return response.data;
};

/**
 * Creates a new external notification template.
 * @param data - Partial ExternalTemplate configuration
 * @returns Promise resolving to the created ExternalTemplate
 * @throws {AxiosError} If creation fails
 */
export const createExternalTemplate = async (data: Partial<ExternalTemplate>) => {
  const response = await client.post<ExternalTemplate>('/notifications/external-templates', data);
  return response.data;
};

/**
 * Updates an existing external notification template.
 * @param id - The template ID to update
 * @param data - Partial ExternalTemplate with fields to update
 * @returns Promise resolving to the updated ExternalTemplate
 * @throws {AxiosError} If update fails or template not found
 */
export const updateExternalTemplate = async (id: string, data: Partial<ExternalTemplate>) => {
  const response = await client.put<ExternalTemplate>(`/notifications/external-templates/${id}`, data);
  return response.data;
};

/**
 * Deletes an external notification template.
 * @param id - The template ID to delete
 * @throws {AxiosError} If deletion fails or template not found
 */
export const deleteExternalTemplate = async (id: string) => {
  await client.delete(`/notifications/external-templates/${id}`);
};

/**
 * Previews an external template with sample data.
 * @param templateId - Optional existing template ID to preview
 * @param template - Optional template content string
 * @param data - Optional sample data for rendering
 * @returns Promise resolving to preview result
 * @throws {AxiosError} If preview fails
 */
export const previewExternalTemplate = async (templateId?: string, template?: string, data?: Record<string, unknown>) => {
  const payload: Record<string, unknown> = {};
  if (templateId) payload.template_id = templateId;
  if (template) payload.template = template;
  if (data) payload.data = data;
  const response = await client.post('/notifications/external-templates/preview', payload);
  return response.data;
};

// Security Notification Settings
/** Security notification configuration. */
export interface SecurityNotificationSettings {
  enabled: boolean;
  min_log_level: string;
  security_waf_enabled: boolean;
  security_acl_enabled: boolean;
  security_rate_limit_enabled: boolean;
  destination_ambiguous?: boolean;
  webhook_url?: string;
  discord_webhook_url?: string;
  slack_webhook_url?: string;
  gotify_url?: string;
  gotify_token?: string;
  email_recipients?: string;
}

/**
 * Fetches security notification settings.
 * @returns Promise resolving to SecurityNotificationSettings
 * @throws {AxiosError} If the request fails
 */
export const getSecurityNotificationSettings = async (): Promise<SecurityNotificationSettings> => {
  const response = await client.get<SecurityNotificationSettings>('/notifications/settings/security');
  return response.data;
};

/**
 * Updates security notification settings.
 * @param settings - Partial settings to update
 * @returns Promise resolving to the updated SecurityNotificationSettings
 * @throws {AxiosError} If update fails
 */
export const updateSecurityNotificationSettings = async (
  settings: Partial<SecurityNotificationSettings>
): Promise<SecurityNotificationSettings> => {
  const response = await client.put<SecurityNotificationSettings>('/notifications/settings/security', settings);
  return response.data;
};
