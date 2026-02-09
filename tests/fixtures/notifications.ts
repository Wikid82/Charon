/**
 * Notification Provider Test Fixtures
 *
 * Shared test data for Notification Provider E2E tests.
 * These fixtures provide consistent test data across notification-related test files.
 */

import * as crypto from 'crypto';

// ============================================================================
// Notification Provider Types
// ============================================================================

/**
 * Supported notification provider types
 */
export type NotificationProviderType =
  | 'discord'
  | 'slack'
  | 'gotify'
  | 'telegram'
  | 'generic'
  | 'webhook';

/**
 * Notification provider configuration interface
 */
export interface NotificationProviderConfig {
  name: string;
  type: NotificationProviderType;
  url: string;
  config?: string;
  template?: string;
  enabled: boolean;
  notify_proxy_hosts: boolean;
  notify_certs: boolean;
  notify_uptime: boolean;
}

/**
 * Notification provider response from API (includes ID)
 */
export interface NotificationProvider extends NotificationProviderConfig {
  id: number;
  created_at: string;
  updated_at: string;
}

// ============================================================================
// Generator Functions
// ============================================================================

/**
 * Generate a unique provider name
 */
export function generateProviderName(prefix: string = 'test-provider'): string {
  return `${prefix}-${Date.now()}-${crypto.randomBytes(3).toString('hex')}`;
}

/**
 * Generate a unique webhook URL for testing
 */
export function generateWebhookUrl(service: string = 'webhook'): string {
  return `https://${service}.test.local/notify/${Date.now()}`;
}

// ============================================================================
// Discord Provider Fixtures
// ============================================================================

/**
 * Valid Discord notification provider configuration
 */
export const discordProvider: NotificationProviderConfig = {
  name: generateProviderName('discord'),
  type: 'discord',
  url: 'https://discord.com/api/webhooks/123456789/abcdefghijklmnop',
  enabled: true,
  notify_proxy_hosts: true,
  notify_certs: true,
  notify_uptime: false,
};

/**
 * Discord provider with all notifications enabled
 */
export const discordProviderAllEvents: NotificationProviderConfig = {
  name: generateProviderName('discord-all'),
  type: 'discord',
  url: 'https://discord.com/api/webhooks/987654321/zyxwvutsrqponmlk',
  enabled: true,
  notify_proxy_hosts: true,
  notify_certs: true,
  notify_uptime: true,
};

/**
 * Discord provider (disabled)
 */
export const discordProviderDisabled: NotificationProviderConfig = {
  ...discordProvider,
  name: generateProviderName('discord-disabled'),
  enabled: false,
};

// ============================================================================
// Slack Provider Fixtures
// ============================================================================

/**
 * Valid Slack notification provider configuration
 */
export const slackProvider: NotificationProviderConfig = {
  name: generateProviderName('slack'),
  type: 'slack',
  url: 'https://hooks.example.com/services/T00000000/B00000000/XXXXXXXXXXXXXXXXXXXXXXXX',
  enabled: true,
  notify_proxy_hosts: true,
  notify_certs: false,
  notify_uptime: true,
};

/**
 * Slack provider with custom template
 */
export const slackProviderWithTemplate: NotificationProviderConfig = {
  name: generateProviderName('slack-template'),
  type: 'slack',
  url: 'https://hooks.example.com/services/T11111111/B11111111/YYYYYYYYYYYYYYYYYYYYYYYY',
  template: 'minimal',
  enabled: true,
  notify_proxy_hosts: true,
  notify_certs: true,
  notify_uptime: true,
};

// ============================================================================
// Gotify Provider Fixtures
// ============================================================================

/**
 * Valid Gotify notification provider configuration
 */
export const gotifyProvider: NotificationProviderConfig = {
  name: generateProviderName('gotify'),
  type: 'gotify',
  url: 'https://gotify.test.local/message?token=Axxxxxxxxxxxxxxxxx',
  enabled: true,
  notify_proxy_hosts: true,
  notify_certs: true,
  notify_uptime: false,
};

// ============================================================================
// Telegram Provider Fixtures
// ============================================================================

/**
 * Valid Telegram notification provider configuration
 */
export const telegramProvider: NotificationProviderConfig = {
  name: generateProviderName('telegram'),
  type: 'telegram',
  url: 'https://api.telegram.org/bot123456789:ABCdefGHIjklMNOpqrSTUvwxYZ/sendMessage?chat_id=987654321',
  enabled: true,
  notify_proxy_hosts: true,
  notify_certs: true,
  notify_uptime: true,
};

// ============================================================================
// Generic Webhook Provider Fixtures
// ============================================================================

/**
 * Valid generic webhook notification provider configuration
 */
export const genericWebhookProvider: NotificationProviderConfig = {
  name: generateProviderName('generic'),
  type: 'generic',
  url: 'https://webhook.test.local/notify',
  config: JSON.stringify({ message: '{{.Message}}', priority: 'normal' }),
  template: 'minimal',
  enabled: true,
  notify_proxy_hosts: true,
  notify_certs: true,
  notify_uptime: true,
};

/**
 * Generic webhook with custom JSON config
 */
export const genericWebhookCustomConfig: NotificationProviderConfig = {
  name: generateProviderName('generic-custom'),
  type: 'generic',
  url: 'https://custom-webhook.test.local/api/notify',
  config: JSON.stringify({
    title: '{{.Title}}',
    body: '{{.Message}}',
    source: 'charon',
    severity: '{{.Severity}}',
  }),
  enabled: true,
  notify_proxy_hosts: true,
  notify_certs: false,
  notify_uptime: false,
};

// ============================================================================
// Custom Webhook Provider Fixtures
// ============================================================================

/**
 * Valid custom webhook notification provider configuration
 */
export const customWebhookProvider: NotificationProviderConfig = {
  name: generateProviderName('webhook'),
  type: 'webhook',
  url: 'https://my-custom-api.test.local/notifications',
  config: JSON.stringify({
    method: 'POST',
    headers: {
      'X-Custom-Header': 'value',
      'Content-Type': 'application/json',
    },
    body: {
      event: '{{.Event}}',
      message: '{{.Message}}',
      timestamp: '{{.Timestamp}}',
    },
  }),
  enabled: true,
  notify_proxy_hosts: true,
  notify_certs: true,
  notify_uptime: true,
};

// ============================================================================
// Invalid Provider Fixtures (for validation testing)
// ============================================================================

/**
 * Invalid provider configurations for testing validation
 */
export const invalidProviderConfigs = {
  missingName: {
    ...discordProvider,
    name: '',
  },
  missingUrl: {
    ...discordProvider,
    url: '',
  },
  invalidUrl: {
    ...discordProvider,
    url: 'not-a-valid-url',
  },
  invalidJsonConfig: {
    ...genericWebhookProvider,
    config: 'invalid-json{',
  },
  nameWithSpecialChars: {
    ...discordProvider,
    name: 'test<script>alert(1)</script>',
  },
  urlWithXss: {
    ...discordProvider,
    url: 'javascript:alert(1)',
  },
};

// ============================================================================
// Notification Template Types and Fixtures
// ============================================================================

/**
 * Notification template interface
 */
export interface NotificationTemplate {
  id?: number;
  name: string;
  content: string;
  description?: string;
  is_builtin?: boolean;
}

/**
 * Built-in template names
 */
export const builtInTemplates = ['default', 'minimal', 'detailed', 'compact'];

/**
 * Custom external template
 */
export const customTemplate: NotificationTemplate = {
  name: 'custom-test-template',
  content: `**{{.Title}}**
Event: {{.Event}}
Time: {{.Timestamp}}
Details: {{.Message}}`,
  description: 'Custom test template for E2E testing',
};

/**
 * Generate a unique template name
 */
export function generateTemplateName(): string {
  return `test-template-${Date.now()}`;
}

/**
 * Create a custom template with unique name
 */
export function createCustomTemplate(overrides: Partial<NotificationTemplate> = {}): NotificationTemplate {
  return {
    name: generateTemplateName(),
    content: `**{{.Title}}**\n{{.Message}}`,
    description: 'Auto-generated test template',
    ...overrides,
  };
}

// ============================================================================
// Notification Event Types
// ============================================================================

/**
 * Notification event types
 */
export type NotificationEvent =
  | 'proxy_host_created'
  | 'proxy_host_updated'
  | 'proxy_host_deleted'
  | 'certificate_issued'
  | 'certificate_renewed'
  | 'certificate_expired'
  | 'uptime_down'
  | 'uptime_recovered';

/**
 * Event configuration for testing specific notification types
 */
export const eventConfigs = {
  proxyHostsOnly: {
    notify_proxy_hosts: true,
    notify_certs: false,
    notify_uptime: false,
  },
  certsOnly: {
    notify_proxy_hosts: false,
    notify_certs: true,
    notify_uptime: false,
  },
  uptimeOnly: {
    notify_proxy_hosts: false,
    notify_certs: false,
    notify_uptime: true,
  },
  allEvents: {
    notify_proxy_hosts: true,
    notify_certs: true,
    notify_uptime: true,
  },
  noEvents: {
    notify_proxy_hosts: false,
    notify_certs: false,
    notify_uptime: false,
  },
};

// ============================================================================
// Mock Notification Test Responses
// ============================================================================

/**
 * Mock notification test success response
 */
export const mockTestSuccess = {
  success: true,
  message: 'Test notification sent successfully',
};

/**
 * Mock notification test failure response
 */
export const mockTestFailure = {
  success: false,
  message: 'Failed to send test notification: Connection refused',
  error: 'ECONNREFUSED',
};

/**
 * Mock notification preview response
 */
export const mockPreviewResponse = {
  content: '**Test Notification**\nThis is a preview of your notification message.',
  rendered_at: new Date().toISOString(),
};

// ============================================================================
// API Helper Functions
// ============================================================================

/**
 * Create notification provider via API
 */
export async function createNotificationProvider(
  request: { post: (url: string, options: { data: unknown }) => Promise<{ ok: () => boolean; json: () => Promise<unknown> }> },
  config: NotificationProviderConfig
): Promise<NotificationProvider> {
  const response = await request.post('/api/v1/notifications/providers', {
    data: config,
  });

  if (!response.ok()) {
    throw new Error('Failed to create notification provider');
  }

  return response.json() as Promise<NotificationProvider>;
}

/**
 * Delete notification provider via API
 */
export async function deleteNotificationProvider(
  request: { delete: (url: string) => Promise<{ ok: () => boolean }> },
  providerId: number
): Promise<void> {
  const response = await request.delete(`/api/v1/notifications/providers/${providerId}`);

  if (!response.ok()) {
    throw new Error(`Failed to delete notification provider: ${providerId}`);
  }
}

/**
 * Create external template via API
 */
export async function createExternalTemplate(
  request: { post: (url: string, options: { data: unknown }) => Promise<{ ok: () => boolean; json: () => Promise<unknown> }> },
  template: NotificationTemplate
): Promise<NotificationTemplate & { id: number }> {
  const response = await request.post('/api/v1/notifications/external-templates', {
    data: template,
  });

  if (!response.ok()) {
    throw new Error('Failed to create external template');
  }

  return response.json() as Promise<NotificationTemplate & { id: number }>;
}

/**
 * Delete external template via API
 */
export async function deleteExternalTemplate(
  request: { delete: (url: string) => Promise<{ ok: () => boolean }> },
  templateId: number
): Promise<void> {
  const response = await request.delete(`/api/v1/notifications/external-templates/${templateId}`);

  if (!response.ok()) {
    throw new Error(`Failed to delete external template: ${templateId}`);
  }
}

/**
 * Test notification provider via API
 */
export async function testNotificationProvider(
  request: { post: (url: string, options: { data: unknown }) => Promise<{ ok: () => boolean; json: () => Promise<unknown> }> },
  providerId: number
): Promise<typeof mockTestSuccess | typeof mockTestFailure> {
  const response = await request.post('/api/v1/notifications/providers/test', {
    data: { provider_id: providerId },
  });

  return response.json() as Promise<typeof mockTestSuccess | typeof mockTestFailure>;
}
