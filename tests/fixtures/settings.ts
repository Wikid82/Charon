/**
 * Settings Test Fixtures
 *
 * Shared test data for Settings E2E tests (System Settings, SMTP Settings, Account Settings).
 * These fixtures provide consistent test data across settings-related test files.
 */

import * as crypto from 'crypto';

// ============================================================================
// SMTP Configuration Types and Fixtures
// ============================================================================

/**
 * SMTP encryption types supported by the system
 */
export type SMTPEncryption = 'none' | 'ssl' | 'starttls';

/**
 * SMTP configuration interface matching backend expectations
 */
export interface SMTPConfig {
  host: string;
  port: number;
  username: string;
  password: string;
  from_address: string;
  encryption: SMTPEncryption;
}

/**
 * Valid SMTP configuration for successful test scenarios
 */
export const validSMTPConfig: SMTPConfig = {
  host: 'smtp.test.local',
  port: 587,
  username: 'testuser',
  password: 'testpass123',
  from_address: 'noreply@test.local',
  encryption: 'starttls',
};

/**
 * Alternative valid SMTP configurations for different encryption types
 */
export const validSMTPConfigSSL: SMTPConfig = {
  host: 'smtp-ssl.test.local',
  port: 465,
  username: 'ssluser',
  password: 'sslpass456',
  from_address: 'ssl-noreply@test.local',
  encryption: 'ssl',
};

export const validSMTPConfigNoAuth: SMTPConfig = {
  host: 'smtp-noauth.test.local',
  port: 25,
  username: '',
  password: '',
  from_address: 'noauth@test.local',
  encryption: 'none',
};

/**
 * Invalid SMTP configurations for validation testing
 */
export const invalidSMTPConfigs = {
  missingHost: { ...validSMTPConfig, host: '' },
  invalidPort: { ...validSMTPConfig, port: -1 },
  portTooHigh: { ...validSMTPConfig, port: 99999 },
  portZero: { ...validSMTPConfig, port: 0 },
  invalidEmail: { ...validSMTPConfig, from_address: 'not-an-email' },
  emptyEmail: { ...validSMTPConfig, from_address: '' },
  invalidEmailMissingDomain: { ...validSMTPConfig, from_address: 'user@' },
  invalidEmailMissingLocal: { ...validSMTPConfig, from_address: '@domain.com' },
};

/**
 * Generate a unique test email address
 */
export function generateTestEmail(): string {
  return `test-${Date.now()}-${crypto.randomBytes(4).toString('hex')}@test.local`;
}

// ============================================================================
// System Settings Types and Fixtures
// ============================================================================

/**
 * SSL provider options
 */
export type SSLProvider = 'auto' | 'letsencrypt-staging' | 'letsencrypt-prod' | 'zerossl';

/**
 * Domain link behavior options
 */
export type DomainLinkBehavior = 'same_tab' | 'new_tab' | 'new_window';

/**
 * System settings interface
 */
export interface SystemSettings {
  caddyAdminApi: string;
  sslProvider: SSLProvider;
  domainLinkBehavior: DomainLinkBehavior;
  publicUrl: string;
  language?: string;
}

/**
 * Default system settings matching application defaults
 */
export const defaultSystemSettings: SystemSettings = {
  caddyAdminApi: 'http://localhost:2019',
  sslProvider: 'auto',
  domainLinkBehavior: 'new_tab',
  publicUrl: 'http://localhost:8080',
  language: 'en',
};

/**
 * System settings with production-like configuration
 */
export const productionSystemSettings: SystemSettings = {
  caddyAdminApi: 'http://caddy:2019',
  sslProvider: 'letsencrypt-prod',
  domainLinkBehavior: 'new_tab',
  publicUrl: 'https://charon.example.com',
  language: 'en',
};

/**
 * Invalid system settings for validation testing
 */
export const invalidSystemSettings = {
  invalidCaddyApiUrl: { ...defaultSystemSettings, caddyAdminApi: 'not-a-url' },
  emptyCaddyApiUrl: { ...defaultSystemSettings, caddyAdminApi: '' },
  invalidPublicUrl: { ...defaultSystemSettings, publicUrl: 'not-a-valid-url' },
  emptyPublicUrl: { ...defaultSystemSettings, publicUrl: '' },
};

/**
 * Generate a valid public URL for testing
 * @param valid - Whether to generate a valid or invalid URL
 */
export function generatePublicUrl(valid: boolean = true): string {
  if (valid) {
    return `https://charon-test-${Date.now()}.example.com`;
  }
  return 'not-a-valid-url';
}

/**
 * Generate a unique Caddy admin API URL for testing
 */
export function generateCaddyApiUrl(): string {
  return `http://caddy-test-${Date.now()}.local:2019`;
}

// ============================================================================
// Account Settings Types and Fixtures
// ============================================================================

/**
 * User profile interface
 */
export interface UserProfile {
  name: string;
  email: string;
}

/**
 * Password change request interface
 */
export interface PasswordChangeRequest {
  currentPassword: string;
  newPassword: string;
  confirmPassword: string;
}

/**
 * Certificate email settings interface
 */
export interface CertificateEmailSettings {
  useAccountEmail: boolean;
  customEmail?: string;
}

/**
 * Valid user profile for testing
 */
export const validUserProfile: UserProfile = {
  name: 'Test User',
  email: 'testuser@example.com',
};

/**
 * Generate a unique user profile for testing
 */
export function generateUserProfile(): UserProfile {
  const timestamp = Date.now();
  return {
    name: `Test User ${timestamp}`,
    email: `testuser-${timestamp}@test.local`,
  };
}

/**
 * Valid password change request
 */
export const validPasswordChange: PasswordChangeRequest = {
  currentPassword: 'OldPassword123!',
  newPassword: 'NewSecureP@ss456',
  confirmPassword: 'NewSecureP@ss456',
};

/**
 * Invalid password change requests for validation testing
 */
export const invalidPasswordChanges = {
  wrongCurrentPassword: {
    currentPassword: 'WrongPassword123!',
    newPassword: 'NewSecureP@ss456',
    confirmPassword: 'NewSecureP@ss456',
  },
  mismatchedPasswords: {
    currentPassword: 'OldPassword123!',
    newPassword: 'NewSecureP@ss456',
    confirmPassword: 'DifferentPassword789!',
  },
  weakPassword: {
    currentPassword: 'OldPassword123!',
    newPassword: '123',
    confirmPassword: '123',
  },
  emptyNewPassword: {
    currentPassword: 'OldPassword123!',
    newPassword: '',
    confirmPassword: '',
  },
  emptyCurrentPassword: {
    currentPassword: '',
    newPassword: 'NewSecureP@ss456',
    confirmPassword: 'NewSecureP@ss456',
  },
};

/**
 * Certificate email settings variations
 */
export const certificateEmailSettings = {
  useAccountEmail: {
    useAccountEmail: true,
  } as CertificateEmailSettings,
  useCustomEmail: {
    useAccountEmail: false,
    customEmail: 'certs@example.com',
  } as CertificateEmailSettings,
  invalidCustomEmail: {
    useAccountEmail: false,
    customEmail: 'not-an-email',
  } as CertificateEmailSettings,
};

// ============================================================================
// Feature Flags Types and Fixtures
// ============================================================================

/**
 * Feature flags interface
 */
export interface FeatureFlags {
  cerberus_enabled: boolean;
  crowdsec_console_enrollment: boolean;
  "feature.notifications.service.email.enabled": boolean;
  uptime_monitoring: boolean;
}

/**
 * Default feature flags (all disabled)
 */
export const defaultFeatureFlags: FeatureFlags = {
  cerberus_enabled: false,
  crowdsec_console_enrollment: false,
  "feature.notifications.service.email.enabled": false,
  uptime_monitoring: false,
};

/**
 * All features enabled
 */
export const allFeaturesEnabled: FeatureFlags = {
  cerberus_enabled: true,
  crowdsec_console_enrollment: true,
  "feature.notifications.service.email.enabled": true,
  uptime_monitoring: true,
};

// ============================================================================
// API Key Types and Fixtures
// ============================================================================

/**
 * API key response interface
 */
export interface ApiKeyResponse {
  api_key: string;
  created_at: string;
}

/**
 * Mock API key for display testing
 */
export const mockApiKey: ApiKeyResponse = {
  api_key: 'charon_api_key_mock_12345678901234567890',
  created_at: new Date().toISOString(),
};

// ============================================================================
// System Health Types and Fixtures
// ============================================================================

/**
 * System health status interface
 */
export interface SystemHealth {
  status: 'healthy' | 'degraded' | 'unhealthy';
  caddy: boolean;
  database: boolean;
  version: string;
  uptime: number;
}

/**
 * Healthy system status
 */
export const healthySystemStatus: SystemHealth = {
  status: 'healthy',
  caddy: true,
  database: true,
  version: '1.0.0-beta',
  uptime: 86400,
};

/**
 * Degraded system status
 */
export const degradedSystemStatus: SystemHealth = {
  status: 'degraded',
  caddy: true,
  database: false,
  version: '1.0.0-beta',
  uptime: 3600,
};

/**
 * Unhealthy system status
 */
export const unhealthySystemStatus: SystemHealth = {
  status: 'unhealthy',
  caddy: false,
  database: false,
  version: '1.0.0-beta',
  uptime: 0,
};

// ============================================================================
// Helper Functions
// ============================================================================

/**
 * Create SMTP config via API
 */
export async function createSMTPConfig(
  request: { post: (url: string, options: { data: unknown }) => Promise<{ ok: () => boolean; json: () => Promise<unknown> }> },
  config: SMTPConfig
): Promise<void> {
  const response = await request.post('/api/v1/settings/smtp', {
    data: config,
  });

  if (!response.ok()) {
    throw new Error('Failed to create SMTP config');
  }
}

/**
 * Update system setting via API
 */
export async function updateSystemSetting(
  request: { post: (url: string, options: { data: unknown }) => Promise<{ ok: () => boolean; json: () => Promise<unknown> }> },
  key: string,
  value: unknown
): Promise<void> {
  const response = await request.post('/api/v1/settings', {
    data: { key, value },
  });

  if (!response.ok()) {
    throw new Error(`Failed to update setting: ${key}`);
  }
}
